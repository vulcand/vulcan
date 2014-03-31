package httploc

import (
	"fmt"
	log "github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/callback"
	. "github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/failover"
	"github.com/mailgun/vulcan/headers"
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/loadbalance"
	"github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcan/request"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type SleepFn func(time.Duration)

// Location with built in failover and load balancing support
type HttpLocation struct {
	transport    *http.Transport
	loadBalancer LoadBalancer // Load balancer controls endpoints in this location
	options      Options      // Additional parameters
}

// Additional options to control this location, such as timeouts and so on
type Options struct {
	Timeouts struct {
		Read time.Duration // Socket read timeout (before we receive the first reply header)
		Dial time.Duration // Socket connect timeout
	}
	ShouldFailover failover.Predicate // Predicate that defines when requests are allowed to failover
	Limiter        Limiter            // Rate limiting algorithm
	// Before callback executed before request gets routed to the endpoint
	// and can intervene during the request lifetime
	Before Before
	// Callback executed after proxy received response from the endpoint
	After After
	// Used to set forwarding headers
	Hostname string
	// In this case appends new forward info to the existing header
	TrustForwardHeader bool
	// Option to override sleep function (useful for testing purposes)
	SleepFn SleepFn
	// Time provider (useful for testing purposes)
	TimeProvider timetools.TimeProvider
}

func NewLocation(loadBalancer LoadBalancer) (*HttpLocation, error) {
	return NewLocationWithOptions(loadBalancer, Options{})
}

func NewLocationWithOptions(loadBalancer LoadBalancer, o Options) (*HttpLocation, error) {
	if loadBalancer == nil {
		return nil, fmt.Errorf("Provide load balancer")
	}
	o, err := parseOptions(o)
	if err != nil {
		return nil, err
	}
	return &HttpLocation{
		loadBalancer: loadBalancer,
		transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, o.Timeouts.Dial)
			},
			ResponseHeaderTimeout: o.Timeouts.Read,
		},
		options: o,
	}, nil
}

// Round trips the request to one of the endpoints, returns the streamed
// request body length in bytes and the endpoint reply.
func (l *HttpLocation) RoundTrip(req Request) (*http.Response, error) {
	for {
		_, err := req.GetBody().Seek(0, 0)
		if err != nil {
			return nil, err
		}

		endpoint, err := l.loadBalancer.NextEndpoint(req)
		if err != nil {
			log.Errorf("Load Balancer failure: %s", err)
			return nil, err
		}

		// Rewrites the request: adds headers, changes urls
		newRequest := l.rewriteRequest(req.GetHttpRequest(), endpoint)

		if l.options.Limiter != nil {
			delay, err := l.options.Limiter.Limit(req)
			if err != nil {
				log.Errorf("Limiter rejects request: %s", err)
				return nil, err
			}
			if delay > 0 {
				log.Infof("Limiter delays request by %s", delay)
				l.options.SleepFn(delay)
			}
		}

		// In case if error is not nil, we allow load balancer to choose the next endpoint
		// e.g. to do request failover. Nil error means that we got proxied the request successfully.
		response, err := l.proxyToEndpoint(endpoint, req, newRequest)
		if l.options.ShouldFailover(req) {
			log.Errorf("Request(%s) failover", req)
			continue
		} else {
			return response, err
		}
	}
	log.Errorf("All endpoints failed!")
	return nil, fmt.Errorf("All endpoints failed")
}

// Proxy the request to the given endpoint, in case if endpoint is down
// or failover code sequence has been recorded as the reply, return the error.
// Failover sequence - is a special response code from the endpoint that indicates
// that endpoint is shutting down and is not willing to accept new requests.
func (l *HttpLocation) proxyToEndpoint(endpoint Endpoint, req Request, httpReq *http.Request) (*http.Response, error) {

	before := []Before{l.options.Before, l.loadBalancer, l.options.Limiter}
	for _, cb := range before {
		if cb != nil {
			response, err := cb.Before(req)
			// In case if error is not nil, return this error to the client
			// and interrupt the callback chain
			if err != nil {
				log.Errorf("Callback says error: %s", err)
				return nil, err
			}
			// If response is present that means that callback wants to proxy
			// this response to the client
			if response != nil {
				return response, nil
			}
		}
	}

	// Forward the reuest and mirror the response
	start := l.options.TimeProvider.UtcNow()
	res, err := l.transport.RoundTrip(httpReq)
	diff := l.options.TimeProvider.UtcNow().Sub(start)

	// Record attempt
	req.AddAttempt(&BaseAttempt{Endpoint: endpoint, Duration: diff, Response: res, Error: err})

	// This gives a chance for callbacks to change the response
	after := []After{l.options.After, l.loadBalancer, l.options.Limiter}
	for _, cb := range after {
		if cb != nil {
			err := cb.After(req)
			if err != nil {
				log.Errorf("After callback returns error and intercepts the response: %s", err)
				return nil, err
			}
		}
	}
	return res, err
}

// This function alters the original request - adds/removes headers, removes hop headers, changes the request path.
func (l *HttpLocation) rewriteRequest(req *http.Request, endpoint Endpoint) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	outReq.URL.Scheme = endpoint.GetUrl().Scheme
	outReq.URL.Host = endpoint.GetUrl().Host
	outReq.URL.RawQuery = req.URL.RawQuery

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	outReq.Close = false

	outReq.Header = make(http.Header)
	netutils.CopyHeaders(outReq.Header, req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if l.options.TrustForwardHeader {
			if prior, ok := outReq.Header[headers.XForwardedFor]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
		}
		outReq.Header.Set(headers.XForwardedFor, clientIP)
	}
	if req.TLS != nil {
		outReq.Header.Set(headers.XForwardedProto, "https")
	} else {
		outReq.Header.Set(headers.XForwardedProto, "http")
	}
	if req.Host != "" {
		outReq.Header.Set(headers.XForwardedHost, req.Host)
	}
	outReq.Header.Set(headers.XForwardedServer, l.options.Hostname)

	// Remove hop-by-hop headers to the backend.  Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	netutils.RemoveHeaders(headers.HopHeaders, outReq.Header)
	return outReq
}

// Standard dial and read timeouts, can be overriden when supplying location
const (
	DefaultHttpReadTimeout = time.Duration(10) * time.Second
	DefaultHttpDialTimeout = time.Duration(10) * time.Second
)

func parseOptions(o Options) (Options, error) {
	if o.Timeouts.Read <= time.Duration(0) {
		o.Timeouts.Read = DefaultHttpReadTimeout
	}
	if o.Timeouts.Dial <= time.Duration(0) {
		o.Timeouts.Dial = DefaultHttpDialTimeout
	}

	if o.SleepFn == nil {
		o.SleepFn = time.Sleep
	}
	if o.Hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			o.Hostname = h
		}
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	if o.ShouldFailover == nil {
		// Failover on erros for 2 times maximum on GET requests only.
		o.ShouldFailover = failover.And(failover.MaxAttempts(2), failover.OnErrors, failover.OnGets)
	}
	return o, nil
}
