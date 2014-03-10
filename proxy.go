// This package contains the proxy core - the main proxy function that accepts and modifies
// request, forwards or denies it.
package vulcan

import (
	"bytes"
	"fmt"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcan/callbacks"
	"github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcan/headers"
	"github.com/mailgun/vulcan/loadbalance"
	"github.com/mailgun/vulcan/netutils"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

// Reverse proxy settings, what loadbalancing algo to use,
// timeouts, rate limiting backend
type ProxySettings struct {
	// Formatter that takes a status code and formats it into proxy response
	ErrorFormatter errors.Formatter
	// Load balancing algo, e.g. RandomLoadBalancer
	LoadBalancer loadbalance.LoadBalancer
	// How long would proxy wait for upstream response
	HttpReadTimeout time.Duration
	// How long would proxy try to dial the upstream
	HttpDialTimeout time.Duration
	// Used to set forwarding headers
	Hostname string
	// In this case appends new forward info to the existing header
	TrustForwardHeader bool
	// Callback executed before request gets routed to the upstream
	Before callbacks.Before
	// Callback executed after proxy received response from the upstream
	After callbacks.After
}

// This is a reverse proxy, not meant to be created directly,
// use NewReverseProxy function instead
type ReverseProxy struct {
	// Customized transport with dial and read timeouts set
	httpTransport *http.Transport
	// Client that uses customized transport
	httpClient *http.Client
	// Remember the settings to use later
	settings ProxySettings
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

	err := p.proxyRequest(w, req)
	if err != nil {
		log.Errorf("Failed to proxy to all upstreams:", err)
		p.replyError(p.settings.ErrorFormatter.FromStatus(http.StatusBadGateway), w, req)
		return
	}
	return
}

// We need this struct to add a Close method and comply with io.ReadCloser
type Buffer struct {
	*bytes.Reader
}

func (*Buffer) Close() error {
	// Does nothing, created to comply with io.ReadCloser requirements
	return nil
}

// Round trips the request to one of the upstreams, returns the streamed
// request body length in bytes and the upstream reply.
func (p *ReverseProxy) proxyRequest(w http.ResponseWriter, req *http.Request) error {

	// We are allowed to fallback in case of upstream failure,
	// record the request body so we can replay it on errors.
	body, err := netutils.NewBodyBuffer(req.Body)
	if err != nil {
		log.Errorf("Request read error %s", err)
		return p.settings.ErrorFormatter.FromStatus(http.StatusBadRequest)
	}

	req.Body = body
	defer body.Close()
	attempt := 0
	for {
		attempt += 1
		_, err := body.Seek(0, 0)
		if err != nil {
			return err
		}
		upstream, err := p.settings.LoadBalancer.NextUpstream(req)
		if err != nil {
			log.Errorf("Load Balancer failure: %s", err)
			return err
		}

		if p.settings.Before != nil {
			err := p.settings.Before.Before(upstream, req, attempt)
			if err != nil {
				log.Errorf("Callback says error: %s", err)
				return err
			}
		}

		log.Infof("Proxy to upstream: %s", upstream)
		err = p.proxyToUpstream(w, req, upstream)
		if err != nil {
			p.settings.LoadBalancer.ReportFailure(upstream, err)
		} else {
			return nil
		}
	}
	log.Errorf("All upstreams failed!")
	return p.settings.ErrorFormatter.FromStatus(http.StatusBadGateway)
}

// Proxy the request to the given upstream, in case if upstream is down
// or failover code sequence has been recorded as the reply, return the error.
// Failover sequence - is a special response code from the upstream that indicates
// that upstream is shutting down and is not willing to accept new requests.
func (p *ReverseProxy) proxyToUpstream(
	w http.ResponseWriter,
	req *http.Request,
	upstream loadbalance.Upstream) error {

	// Rewrites the request: adds headers, changes urls etc.
	outReq := p.rewriteRequest(req, upstream)

	// Forward the reuest and mirror the response
	res, err := p.httpTransport.RoundTrip(outReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if p.settings.After != nil {
		err := p.settings.After.After(upstream, req, res)
		if err != nil {
			log.Errorf("After error: %s", err)
		}
	}

	netutils.CopyHeaders(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	return nil
}

// This function alters the original request - adds/removes headers, removes hop headers,
// changes the request path.
func (p *ReverseProxy) rewriteRequest(req *http.Request, upstream loadbalance.Upstream) *http.Request {
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
	if s.LoadBalancer == nil {
		return s, fmt.Errorf("Load balancer can not be nil")
	}
	if s.HttpReadTimeout == time.Duration(0) {
		s.HttpReadTimeout = DefaultHttpReadTimeout
	}
	if s.HttpReadTimeout == time.Duration(0) {
		s.HttpDialTimeout = DefaultHttpDialTimeout
	}
	return s, nil
}
