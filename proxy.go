// Proxy accepts the request, calls the control service for instructions
// And takes actions according to instructions received.
package vulcan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/mailgun/vulcan/backend"
	"github.com/mailgun/vulcan/command"
	"github.com/mailgun/vulcan/control"
	"github.com/mailgun/vulcan/loadbalance"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/ratelimit"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// Defines Reverse proxy runtime settings, what loadbalancing algo to use,
// timeouts, throttling backend.
type ProxySettings struct {
	// Controlller that tells proxy what to do with the request
	// as controller implementation may vary
	Controller control.Controller
	// Any backend that would be used by throttler to keep throttling stats,
	// e.g. MemoryBackend or CassandraBackend
	ThrottlerBackend backend.Backend
	// Load balancing algo, e.g. RandomLoadBalancer
	LoadBalancer loadbalance.Balancer
	// How long would proxy wait for server response
	HttpReadTimeout time.Duration
	// How long would proxy try to dial server
	HttpDialTimeout time.Duration
}

// This is a reverse proxy, not meant to be created directly,
// use NewReverseProxy function instead
type ReverseProxy struct {
	// Controller that decides what to do with the request
	controller control.Controller
	// Sorts upstreams, control servers in accrordance to it's internal
	// algorithm
	loadBalancer loadbalance.Balancer
	// Customized transport with dial and read timeouts set
	httpTransport *http.Transport
	// Client that uses customized transport
	httpClient *http.Client
	// Rate limiter
	rateLimiter *ratelimit.RateLimiter
}

// Standard dial and read timeouts, can be overriden when supplying proxy settings
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

	var rateLimiter *ratelimit.RateLimiter
	if s.ThrottlerBackend != nil {
		rateLimiter = &ratelimit.RateLimiter{Backend: s.ThrottlerBackend}
	}

	p := &ReverseProxy{
		controller:    s.Controller,
		loadBalancer:  s.LoadBalancer,
		httpTransport: transport,
		rateLimiter:   rateLimiter,
		httpClient: &http.Client{
			Transport: transport,
		},
	}
	return p, nil
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	glog.Infof("Serving Request %s %s", req.Method, req.RequestURI)

	// Ask controller for instructions
	cmdI, err := p.controller.GetInstructions(req)
	if err != nil {
		glog.Errorf("Error getting instructions: %s", err)
		p.replyError(err, w, req)
		return
	}

	switch cmd := cmdI.(type) {
	case *command.Reply:
		glog.Infof("Got Reply command: %v", cmd)
		p.replyCommand(cmd, w, req)
		return
	case *command.Forward:
		glog.Infof("Got Forward command %v", cmd)
		// Get upstreams ready to process the request
		retrySeconds, err := p.rateLimit(cmd)
		if err != nil {
			p.replyError(err, w, req)
			return
		}
		if retrySeconds != 0 {
			p.replyError(&command.RetryError{Seconds: retrySeconds}, w, req)
			return
		}
		endpoints := command.EndpointsFromUpstreams(cmd.Upstreams)
		// Proxy request to the selected upstream
		requestBytes, err := p.proxyRequest(w, req, cmd, endpoints)
		if err != nil {
			glog.Error("Failed to proxy to the upstreams:", err)
			p.replyError(err, w, req)
			return
		}
		p.updateRates(requestBytes, cmd)
		return
	}
	p.replyError(fmt.Errorf("Internal logic error"), w, req)
}

func (p *ReverseProxy) rateLimit(cmd *command.Forward) (int, error) {
	if p.rateLimiter == nil || cmd.Rates == nil {
		return 0, nil
	}
	retrySeconds, err := p.rateLimiter.GetRetrySeconds(cmd.Rates)
	if err != nil {
		glog.Errorf("RateLimiter failure: %s continuing with the request", err)
		return 0, nil
	}
	return retrySeconds, err
}

func (p *ReverseProxy) updateRates(requestBytes int, cmd *command.Forward) {
	if p.rateLimiter == nil || cmd.Rates == nil {
		return
	}
	err := p.rateLimiter.UpdateStats(int64(requestBytes), cmd.Rates)
	if err != nil {
		glog.Errorf("RateLimiter failure: %s ignoring", err)
	}
}

// We need this struct to add a Close method and comply with io.ReadCloser
type Buffer struct {
	*bytes.Reader
}

func (*Buffer) Close() error {
	// Does nothing, created to comply with io.ReadCloser requirements
	return nil
}

func (p *ReverseProxy) nextEndpoint(endpoints []loadbalance.Endpoint) (*command.Endpoint, error) {
	// Get first endpoint
	pendpoint, err := p.loadBalancer.NextEndpoint(endpoints)
	if err != nil {
		glog.Errorf("Loadbalancer failure: %s", err)
		return nil, err
	}
	endpoint, ok := pendpoint.(*command.Endpoint)
	if !ok {
		return nil, fmt.Errorf("Failed to convert types! Unknown type: %v", pendpoint)
	}
	return endpoint, nil
}

func (p *ReverseProxy) proxyRequest(
	w http.ResponseWriter, req *http.Request,
	cmd *command.Forward,
	endpoints []loadbalance.Endpoint) (int, error) {

	// We are allowed to fallback in case of upstream failure,
	// so let us record the request body so we can replay
	// it on errors actually
	buffer, err := ioutil.ReadAll(req.Body)
	if err != nil {
		glog.Errorf("Request read error %s", err)
		return 0, netutils.NewHttpError(http.StatusBadRequest)
	}

	reader := &Buffer{bytes.NewReader(buffer)}
	requestLength := reader.Len()
	req.Body = reader

	for i := 0; i < len(endpoints); i++ {
		_, err := reader.Seek(0, 0)
		if err != nil {
			return 0, err
		}
		endpoint, err := p.nextEndpoint(endpoints)
		if err != nil {
			glog.Errorf("Load Balancer failure: %s", err)
			return 0, err
		}
		glog.Infof("With failover, proxy to upstream: %s", endpoint.Upstream)
		err = p.proxyToUpstream(w, req, cmd, endpoint.Upstream)
		if err != nil {
			if cmd.Failover == nil || !cmd.Failover.Active {
				return 0, err
			}
			glog.Errorf("Upstream %s error, falling back to another", endpoint.Upstream)
			endpoint.Active = false
		} else {
			return 0, nil
		}
	}

	glog.Errorf("All upstreams failed!")
	return requestLength, netutils.NewHttpError(http.StatusBadGateway)
}

func (p *ReverseProxy) proxyToUpstream(
	w http.ResponseWriter,
	req *http.Request,
	cmd *command.Forward,
	upstream *command.Upstream) error {

	// Rewrites the request: adds headers, changes urls etc.
	outReq := rewriteRequest(req, cmd, upstream)

	// Forward the reuest and mirror the response
	res, err := p.httpTransport.RoundTrip(outReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// In some cases upstreams may return special error codes that indicate that instead
	// of proxying the response of the upstream to the client we should initiate a failover
	if cmd.Failover != nil && len(cmd.Failover.Codes) != 0 {
		for _, code := range cmd.Failover.Codes {
			if res.StatusCode == code {
				glog.Errorf("Upstream %s initiated failover with status code %d", upstream, code)
				return fmt.Errorf("Upstream %s initiated failover with status code %d", upstream, code)
			}
		}
	}

	netutils.CopyHeaders(w.Header(), res.Header)

	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	return nil
}

func rewriteRequest(req *http.Request, cmd *command.Forward, upstream *command.Upstream) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	outReq.URL.Scheme = upstream.Scheme
	outReq.URL.Host = fmt.Sprintf("%s:%d", upstream.Host, upstream.Port)
	if len(upstream.RewritePath) != 0 {
		outReq.URL.Path = upstream.RewritePath
	}

	outReq.URL.RawQuery = req.URL.RawQuery

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	outReq.Close = false

	glog.Infof("Proxying request to: %v", outReq)

	// We copy headers only if we alter the original request
	// headers, otherwise we use the shallow copy
	if len(cmd.AddHeaders) != 0 ||
		len(cmd.RemoveHeaders) != 0 ||
		len(upstream.AddHeaders) != 0 ||
		len(upstream.RemoveHeaders) != 0 ||
		netutils.HasHeaders(hopHeaders, req.Header) {
		outReq.Header = make(http.Header)
		netutils.CopyHeaders(outReq.Header, req.Header)
	}

	if len(upstream.RemoveHeaders) != 0 {
		netutils.RemoveHeaders(upstream.RemoveHeaders, outReq.Header)
	}

	// Add upstream headers to the request
	if len(upstream.AddHeaders) != 0 {
		glog.Info("Proxying Upstream headers:", upstream.AddHeaders)
		netutils.CopyHeaders(outReq.Header, upstream.AddHeaders)
	}

	if len(cmd.RemoveHeaders) != 0 {
		netutils.RemoveHeaders(cmd.RemoveHeaders, outReq.Header)
	}

	// Add generic instructions headers to the request
	if len(cmd.AddHeaders) != 0 {
		glog.Info("Proxying instructions headers:", cmd.AddHeaders)
		netutils.CopyHeaders(outReq.Header, cmd.AddHeaders)
	}

	// Remove hop-by-hop headers to the backend.  Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	netutils.RemoveHeaders(hopHeaders, outReq.Header)
	return outReq
}

// Helper function to reply with http errors
func (p *ReverseProxy) replyError(err error, w http.ResponseWriter, req *http.Request) {
	httpResponse := p.controller.ConvertError(req, err)
	// Discard the request body, so that clients can actually receive the response
	// Otherwise they can only see lost connection
	// TODO: actually check this
	io.Copy(ioutil.Discard, req.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpResponse.StatusCode)
	w.Write(httpResponse.Body)
}

// Helper function to reply with http errors
func (p *ReverseProxy) replyCommand(cmd *command.Reply, w http.ResponseWriter, req *http.Request) {
	// Discard the request body, so that clients can actually receive the response
	// Otherwise they can only see lost connection
	// TODO: actually check this
	io.Copy(ioutil.Discard, req.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(cmd.Code)
	body, err := json.Marshal(cmd.Message)
	if err != nil {
		glog.Errorf("Failed to serialize body: %s", err)
		body = []byte("Internal system error")
	}
	w.Write(body)
}

func validateProxySettings(s *ProxySettings) (*ProxySettings, error) {
	if s == nil {
		return nil, fmt.Errorf("Provide proxy settings")
	}
	if s.Controller == nil {
		return nil, fmt.Errorf("Controller can not be nil")
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
