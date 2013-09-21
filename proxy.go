package vulcan

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

// On every request proxy asks control server what to do
// with the request, control server replies with this structure
// or rejects the request.
type ProxyInstructions struct {
	// Tokens uniquely identify the requester. E.g. token can be account id or
	// combination of ip and account id. Tokens can be throttled as well.
	// The reply can have 0 or several tokens
	Tokens []*Token
	// List of upstreams that can accept this request. Load balancer will
	// choose an upstream based on the algo, e.g. random, round robin,
	// or least connections. At least one upstream is required.
	Upstreams []*Upstream
	// If supplied, headers will be added to the proxied request.
	Headers http.Header
}

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
	// Chooses upstreams
	loadBalancer LoadBalancer
	// Customized transport with dial and read timeouts set
	httpTransport *http.Transport
	// Client that uses customized transport
	httpClient *http.Client
}

const (
	DefaultHttpReadTimeout = time.Duration(30) * time.Second
	DefaultHttpDialTimeout = time.Duration(30) * time.Second
)

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
	LogMessage("Serving Request %s %s", req.Method, req.RequestURI)

	// Ask control server for instructions
	instructions, httpError, err := getInstructions(p.httpClient, p.getServer(), req)
	if err != nil {
		LogError("Failed to get instructions: %s", err)
		p.replyError(NewHttpError(http.StatusInternalServerError), w, req)
		return
	}

	// Control server denied the request
	if httpError != nil {
		//Control server has rejected the request
		LogError("Control server denied request: %s", httpError)
		p.replyError(httpError, w, req)
		return
	}

	// Select an upstream
	upstream, httpError, err := p.chooseUpstream(instructions)
	if err != nil {
		p.replyError(NewHttpError(http.StatusInternalServerError), w, req)
		return
	}
	if httpError != nil {
		p.replyError(httpError, w, req)
		return
	}

	// Proxy request to the selected upstream
	err = p.proxyRequest(w, req, upstream)
	if err != nil {
		LogError("Upstream error: %v", err)
		p.replyError(NewHttpError(http.StatusBadGateway), w, req)
		return
	}

	// Update usage stats
	err = p.throttler.updateStats(instructions.Tokens, upstream)
	if err != nil {
		LogError("Failed to update stats: %s", err)
	}
}

func NewProxyInstructions(
	tokens []*Token,
	upstreams []*Upstream,
	headers http.Header) (*ProxyInstructions, error) {

	if len(upstreams) <= 0 {
		return nil, fmt.Errorf("At least one upstream is required")
	}

	return &ProxyInstructions{
		Tokens:    tokens,
		Upstreams: upstreams,
		Headers:   headers}, nil
}

func (r *ReverseProxy) getServer() *url.URL {
	index := randomRange(0, len(r.controlServers))
	return r.controlServers[index]
}

func (p *ReverseProxy) chooseUpstream(instructions *ProxyInstructions) (*Upstream, *HttpError, error) {
	// Throttle the requests to find available upstreams
	// We may fall back to all upstreams if throttler is down
	// If there are no available upstreams, we reject the request
	upstreamStats, retrySeconds, err := p.throttler.throttle(instructions)
	if err != nil {
		// throtller is down, we are falling back
		// so we won't loose the request
		index := randomRange(0, len(instructions.Upstreams))
		upstream := instructions.Upstreams[index]
		LogError("Throtter down, falling back to upstream %s", upstream.Url)
		return upstream, nil, nil
	} else if len(upstreamStats) == 0 {
		// No available upstreams
		httpError, err := TooManyRequestsError(retrySeconds)
		if err != nil {
			return nil, nil, err
		}
		return nil, httpError, nil
	} else {
		// Choose an upstream based on the stats
		upstream, err := p.loadBalancer.chooseUpstream(upstreamStats)
		if err != nil {
			return nil, nil, err
		}
		return upstream, nil, nil
	}
}

func (p *ReverseProxy) proxyRequest(w http.ResponseWriter, req *http.Request, upstream *Upstream) error {
	// Rewrites the request: adds headers, changes url etc
	outReq := rewriteRequest(upstream, req)

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

func rewriteRequest(upstream *Upstream, req *http.Request) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but okay

	outReq.URL.Scheme = upstream.Url.Scheme
	outReq.URL.Host = upstream.Url.Host
	outReq.URL.Path = upstream.Url.Path
	outReq.URL.RawQuery = req.URL.RawQuery

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	outReq.Close = false

	if upstream.Headers != nil {
		LogMessage("Proxying Upstream headers: %s", upstream.Headers)
		copyHeaders(outReq.Header, upstream.Headers)
	}

	return outReq
}

func (p *ReverseProxy) replyError(err *HttpError, w http.ResponseWriter, req *http.Request) {
	// Discard the request body, so that clients can actually receive the response
	// Otherwise they can only see lost connection
	// TODO: actually check this
	io.Copy(ioutil.Discard, req.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)
	w.Write(err.Body)
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
