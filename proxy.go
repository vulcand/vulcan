// Proxy accepts the request, calls the control service for instructions
// And takes actions according to instructions received.
package vulcan

import (
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Defines Reverse proxy runtime settings, what loadbalancing algo to use,
// timeouts, throttling backend.
type ProxySettings struct {
	// List of http urls of servers controlling the reqquest,
	// see ControlRequest for details
	ControlServers []string
	// Any backend that would be used by throttler to keep throttling stats,
	// e.g. MemoryBackend or CassandraBackend
	ThrottlerBackend Backend
	// Load balancing algo, e.g. RandomLoadBalancer
	LoadBalancer LoadBalancer
	// How long would proxy wait for server response
	HttpReadTimeout time.Duration
	// How long would proxy try to dial server
	HttpDialTimeout time.Duration
}

// This is a reverse proxy, not meant to be created directly,
// use NewReverseProxy function instead
type ReverseProxy struct {
	// Control server urls that decide what to do with the request
	controlServers []*url.URL
	// Filters upstreams based on the throtting data
	throttler *Throttler
	// Sorts upstreams, control servers in accrordance to it's internal
	// algorithm
	loadBalancer LoadBalancer
	// Customized transport with dial and read timeouts set
	httpTransport *http.Transport
	// Client that uses customized transport
	httpClient *http.Client
}

// Standard dial and read timeouts, can be overriden when supplying
// proxy settings
const (
	DefaultHttpReadTimeout = time.Duration(10) * time.Second
	DefaultHttpDialTimeout = time.Duration(10) * time.Second
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
// Copied from reverseproxy.go, too bad
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// Creates reverse proxy that acts like http server
func NewReverseProxy(s *ProxySettings) (*ReverseProxy, error) {
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
		controlServers: make([]*url.URL, len(s.ControlServers)),
		throttler:      NewThrottler(s.ThrottlerBackend),
		loadBalancer:   s.LoadBalancer,
		httpTransport:  transport,
		httpClient: &http.Client{
			Transport: transport,
		},
	}

	for i, str := range s.ControlServers {
		u, err := url.Parse(str)
		if err != nil {
			return nil, err
		}
		p.controlServers[i] = u
	}
	return p, nil
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	glog.Infof("Serving Request %s %s", req.Method, req.RequestURI)

	controlServers, err := p.loadBalancer.sortedUrls(p.controlServers)
	if err != nil {
		p.replyError(err, w, req)
		return
	}

	// Ask control server for instructions
	instructions, err := getInstructions(p.httpClient, controlServers, req)
	if err != nil {
		p.replyError(err, w, req)
		return
	}

	// Select an upstream
	upstreams, err := p.getUpstreams(instructions)
	if err != nil {
		p.replyError(err, w, req)
		return
	}

	// Proxy request to the selected upstream
	upstream, err := p.proxyRequest(w, req, instructions, upstreams)
	if err != nil {
		glog.Error("Failed to proxy to the upstreams:", err)
		p.replyError(err, w, req)
		return
	}

	// Update usage stats
	err = p.throttler.updateStats(instructions.Tokens, upstream)
	if err != nil {
		glog.Error("Failed to update stats:", err)
	}
}

func NewProxyInstructions(
	failover bool,
	tokens []*Token,
	upstreams []*Upstream,
	headers http.Header) (*ProxyInstructions, error) {

	if len(upstreams) <= 0 {
		return nil, fmt.Errorf("At least one upstream is required")
	}

	return &ProxyInstructions{
		Failover:  failover,
		Tokens:    tokens,
		Upstreams: upstreams,
		Headers:   headers}, nil
}

func (p *ReverseProxy) getUpstreams(instructions *ProxyInstructions) ([]*Upstream, error) {
	// Throttle the requests to find available upstreams
	// We may fall back to all upstreams if throttler is down
	// If there are no available upstreams, we reject the request
	upstreamStats, retrySeconds, err := p.throttler.throttle(instructions)
	if err != nil {
		// throtller is down, we are falling back
		// so we won't loose the request
		glog.Error("Throtter is down, falling back to random shuffling")
		return p.loadBalancer.sortedUpstreams(instructions.Upstreams)
	} else if len(upstreamStats) == 0 {
		// No available upstreams
		return nil, TooManyRequestsError(retrySeconds)
	} else {
		// Choose an upstream based on the usage stats
		return p.loadBalancer.sortedUpstreamsByStats(upstreamStats)
	}
}

// We need this struct to add a Close method
// and comply with io.ReadCloser
type Buffer struct {
	*bytes.Reader
}

func (*Buffer) Close() error {
	// Does nothing, created to comply with
	// io.ReadCloser requirements
	return nil
}

func (p *ReverseProxy) proxyRequest(w http.ResponseWriter, req *http.Request, instructions *ProxyInstructions, upstreams []*Upstream) (*Upstream, error) {

	if !instructions.Failover {
		upstream := upstreams[0]
		glog.Infof("Without failover, proxy to upstream: %s", upstream)
		err := p.proxyToUpstream(w, req, instructions, upstream)
		if err != nil {
			glog.Errorf("Upstream error: %s", err)
			return nil, NewHttpError(http.StatusBadGateway)
		}
		return upstream, nil
	}

	// We are allowed to fallback in case of upstream failure,
	// so let us record the request body so we can replay
	// it on errors actually
	buffer, err := ioutil.ReadAll(req.Body)
	if err != nil {
		glog.Errorf("Request read error %s", err)
		return nil, NewHttpError(http.StatusBadRequest)
	}
	reader := &Buffer{bytes.NewReader(buffer)}
	req.Body = reader

	for _, upstream := range upstreams {
		_, err := reader.Seek(0, 0)
		if err != nil {
			return nil, err
		}
		glog.Infof("With failover, proxy to upstream: %s", upstream)
		err = p.proxyToUpstream(w, req, instructions, upstream)
		if err != nil {
			glog.Errorf("Upstream %s error, falling back to another", upstream)
		} else {
			return upstream, nil
		}
	}

	glog.Errorf("All upstreams failed!")
	return nil, NewHttpError(http.StatusBadGateway)
}

func (p *ReverseProxy) proxyToUpstream(w http.ResponseWriter, req *http.Request, instructions *ProxyInstructions, upstream *Upstream) error {
	// Rewrites the request: adds headers, changes urls etc.
	outReq := rewriteRequest(req, instructions, upstream)

	// Forward the reuest and mirror the response
	res, err := p.httpTransport.RoundTrip(outReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	copyHeaders(w.Header(), res.Header)

	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	return nil
}

func rewriteRequest(req *http.Request, instructions *ProxyInstructions, upstream *Upstream) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	outReq.URL.Scheme = upstream.Url.Scheme
	outReq.URL.Host = upstream.Url.Host
	outReq.URL.Path = upstream.Url.Path
	outReq.URL.RawQuery = req.URL.RawQuery

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	outReq.Close = false

	// We copy headers only if we alter the original request
	// headers, otherwise we use the shallow copy
	if len(instructions.Headers) != 0 || len(upstream.Headers) != 0 || hasHeaders(hopHeaders, req.Header) {
		outReq.Header = make(http.Header)
		copyHeaders(outReq.Header, req.Header)
	}

	// Add upstream headers to the request
	if len(upstream.Headers) != 0 {
		glog.Info("Proxying Upstream headers:", upstream.Headers)
		copyHeaders(outReq.Header, upstream.Headers)
	}

	// Add generic instructions headers to the request
	if len(instructions.Headers) != 0 {
		glog.Info("Proxying instructions headers:", instructions.Headers)
		copyHeaders(outReq.Header, instructions.Headers)
	}

	// Remove hop-by-hop headers to the backend.  Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	removeHeaders(hopHeaders, outReq.Header)
	return outReq
}

// Helper function to reply with http errors
func (p *ReverseProxy) replyError(err error, w http.ResponseWriter, req *http.Request) {
	httpErr, isHttp := err.(*HttpError)
	if !isHttp {
		httpErr = NewHttpError(http.StatusInternalServerError)
	}

	// Discard the request body, so that clients can actually receive the response
	// Otherwise they can only see lost connection
	// TODO: actually check this
	io.Copy(ioutil.Discard, req.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.StatusCode)
	w.Write(httpErr.Body)
}

func validateProxySettings(s *ProxySettings) (*ProxySettings, error) {
	if s == nil {
		return nil, fmt.Errorf("Provide proxy settings")
	}
	if len(s.ControlServers) == 0 {
		return nil, fmt.Errorf("Supply at least one control server")
	}
	if s.ThrottlerBackend == nil {
		return nil, fmt.Errorf("Backend can not be nil")
	}
	if s.LoadBalancer == nil {
		return nil, fmt.Errorf("Load balancer can not be nil")
	}
	if s.HttpReadTimeout == time.Duration(0) {
		s.HttpReadTimeout = DefaultHttpReadTimeout
	}
	if s.HttpReadTimeout == time.Duration(0) {
		s.HttpDialTimeout = DefaultHttpDialTimeout
	}
	return s, nil
}
