// This package contains the proxy core - the main proxy function that accepts and modifies
// request, forwards or denies it.
package vulcan

import (
	"fmt"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcan/callback"
	"github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcan/headers"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcan/route"
	. "github.com/mailgun/vulcan/upstream"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type SleepFn func(time.Duration)

// Reverse proxy settings, what loadbalancing algo to use,
// timeouts, rate limiting backend
type ProxySettings struct {
	// Formatter that takes a status code and formats it into proxy response
	ErrorFormatter errors.Formatter
	// Router decides where does the request go and what load balancer handles it
	Router route.Router
	// How long would proxy wait for upstream response
	HttpReadTimeout time.Duration
	// How long would proxy try to dial the upstream
	HttpDialTimeout time.Duration
	// Used to set forwarding headers
	Hostname string
	// In this case appends new forward info to the existing header
	TrustForwardHeader bool
	// Before callback executed before request gets routed to the upstream
	// and can intervene during the request lifetime
	Before callback.Before
	// Callback executed after proxy received response from the upstream
	After callback.After
	// Option to override sleep function (useful for testing purposes)
	SleepFn SleepFn
}

type ReverseProxy struct {
	// Customized transport with dial and read timeouts set
	httpTransport *http.Transport
	// Client that uses customized transport
	httpClient *http.Client
	// Connection settings, load balancing algo to use, callbacks and watchers
	settings ProxySettings
	// Counter that is used to provide unique identifiers for requests
	lastRequestId int64
}

// Standard dial and read timeouts, can be overriden when supplying proxy settings
const (
	DefaultHttpReadTimeout = time.Duration(10) * time.Second
	DefaultHttpDialTimeout = time.Duration(10) * time.Second
)

// Creates reverse proxy that acts like http server.
func NewReverseProxy(s ProxySettings) (*ReverseProxy, error) {
	s, err := validateProxySettings(s)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, s.HttpDialTimeout)
		},
		ResponseHeaderTimeout: s.HttpReadTimeout,
	}

	p := &ReverseProxy{
		settings:      s,
		httpTransport: transport,
		httpClient: &http.Client{
			Transport: transport,
		},
	}
	return p, nil
}

// Main request handler, accepts requests, round trips it to the upstream
// proxies back the response.
func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Infof("Serving Request %s %s", req.Method, req.RequestURI)
	// Wrap the original request into wrapper with more detailed information available
	request := &request.BaseRequest{
		HttpRequest: req,
		Id:          atomic.AddInt64(&p.lastRequestId, 1),
	}
	_, err := p.proxyRequest(w, request)
	if err != nil {
		log.Errorf("Failed to proxy to all upstreams:", err)
		p.replyError(p.settings.ErrorFormatter.FromStatus(http.StatusBadGateway), w, req)
	}
}

// Round trips the request to one of the upstreams, returns the streamed
// request body length in bytes and the upstream reply.
func (p *ReverseProxy) proxyRequest(w http.ResponseWriter, request *request.BaseRequest) (*http.Response, error) {

	// We are allowed to fallback in case of upstream failure,
	// record the request body so we can replay it on errors.
	body, err := netutils.NewBodyBuffer(request.HttpRequest.Body)
	if err != nil {
		log.Errorf("Request read error %s", err)
		return nil, p.settings.ErrorFormatter.FromStatus(http.StatusBadRequest)
	}

	request.HttpRequest.Body = body
	defer body.Close()

	location, err := p.settings.Router.Route(request)
	if err != nil {
		return nil, err
	}

	for {
		_, err := body.Seek(0, 0)
		if err != nil {
			return nil, err
		}
		upstream, err := location.GetLoadBalancer().NextUpstream(request)
		if err != nil {
			log.Errorf("Load Balancer failure: %s", err)
			return nil, err
		}
		request.CurrentUpstream = upstream
		// Rewrites the request: adds headers, changes urls
		request.HttpRequest = p.rewriteRequest(request.HttpRequest, request.CurrentUpstream)
		log.Infof("Proxy to upstream: %s", upstream)

		if location.GetLimiter() != nil {
			delay, err := location.GetLimiter().Limit(request)
			if err != nil {
				log.Errorf("Limiter rejects request: %s", err)
				return nil, err
			}
			if delay > 0 {
				log.Infof("Limiter delays request by %s", delay)
				if p.settings.SleepFn != nil {
					p.settings.SleepFn(delay)
				} else {
					time.Sleep(delay)
				}
			}
		}

		// In case if error is not nil, we allow load balancer to choose the next upstream
		// e.g. to do request failover. Nil error means that we got proxied the request successfully.
		response, err := p.proxyToUpstream(w, location, request)
		if err == nil {
			return response, err
		}
	}
	log.Errorf("All upstreams failed!")
	return nil, p.settings.ErrorFormatter.FromStatus(http.StatusBadGateway)
}

// Proxy the request to the given upstream, in case if upstream is down
// or failover code sequence has been recorded as the reply, return the error.
// Failover sequence - is a special response code from the upstream that indicates
// that upstream is shutting down and is not willing to accept new requests.
func (p *ReverseProxy) proxyToUpstream(
	w http.ResponseWriter,
	location route.Location,
	request *request.BaseRequest) (*http.Response, error) {

	before := []callback.Before{
		p.settings.Before,
		location.GetLoadBalancer(),
		location.GetLimiter(),
	}

	for _, cb := range before {
		if cb != nil {
			response, err := cb.Before(request)
			// In case if error is not nil, return this error to the client
			// and interrupt the callback chain
			if err != nil {
				log.Errorf("Callback says error: %s", err)
				return nil, err
			}
			// If response is present that means that callback wants to proxy
			// this response to the client
			if response != nil {
				netutils.CopyHeaders(w.Header(), response.Header)
				w.WriteHeader(response.StatusCode)
				io.Copy(w, response.Body)
				return response, nil
			}
		}
	}

	// Forward the reuest and mirror the response
	res, err := p.httpTransport.RoundTrip(request.HttpRequest)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	after := []callback.After{
		p.settings.After,
		location.GetLoadBalancer(),
		location.GetLimiter(),
	}

	// This gives a chance for callbacks to change the response
	for _, cb := range after {
		if cb != nil {
			err := cb.After(request, res, err)
			if err != nil {
				log.Errorf("After returned error and intercepts the response: %s", err)
				return nil, err
			}
		}
	}

	netutils.CopyHeaders(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	return res, nil
}

// This function alters the original request - adds/removes headers, removes hop headers,
// changes the request path.
func (p *ReverseProxy) rewriteRequest(req *http.Request, upstream Upstream) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	outReq.URL.Scheme = upstream.GetUrl().Scheme
	outReq.URL.Host = upstream.GetUrl().Host
	outReq.URL.RawQuery = req.URL.RawQuery

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	outReq.Close = false

	log.Infof("Proxying request to: %v", outReq)

	outReq.Header = make(http.Header)
	netutils.CopyHeaders(outReq.Header, req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if p.settings.TrustForwardHeader {
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
	outReq.Header.Set(headers.XForwardedServer, p.settings.Hostname)

	// Remove hop-by-hop headers to the backend.  Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	netutils.RemoveHeaders(headers.HopHeaders, outReq.Header)
	return outReq
}

// Helper function to reply with http errors
func (p *ReverseProxy) replyError(err errors.HttpError, w http.ResponseWriter, req *http.Request) {
	// Discard the request body, so that clients can actually receive the response
	// Otherwise they can only see lost connection
	// TODO: actually check this
	io.Copy(ioutil.Discard, req.Body)
	w.Header().Set("Content-Type", err.GetContentType())
	w.WriteHeader(err.GetStatusCode())
	w.Write(err.GetBody())
}

func validateProxySettings(s ProxySettings) (ProxySettings, error) {
	if s.Router == nil {
		return s, fmt.Errorf("Router can not be nil")
	}
	if s.HttpReadTimeout == time.Duration(0) {
		s.HttpReadTimeout = DefaultHttpReadTimeout
	}
	if s.HttpReadTimeout == time.Duration(0) {
		s.HttpDialTimeout = DefaultHttpDialTimeout
	}
	return s, nil
}
