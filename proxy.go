package vulcan

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

// This tells proxy what to do with request
type ProxyInstructions struct {
	Tokens    []*Token    // Tokens uniquely identify the requester. Token can be account id or combination of ip and account id
	Upstreams []*Upstream // List of upstreams that can accept this reuqest
	Headers   http.Header // Headers will be apllied to the proxied request
}

type ReverseProxy struct {
	controlServers []*url.URL   // control servers that decide what to do with the request
	throttler      *Throttler   // filters upstreams based on the throtting data
	loadBalancer   LoadBalancer // chooses upstreams using whatever algo
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

func NewReverseProxy(controlServers []string, backend Backend, loadBalancer LoadBalancer) (*ReverseProxy, error) {

	p := &ReverseProxy{
		controlServers: make([]*url.URL, len(controlServers)),
		throttler:      NewThrottler(backend),
		loadBalancer:   loadBalancer,
	}
	for i, str := range controlServers {
		u, err := url.Parse(str)
		if err != nil {
			return nil, err
		}
		p.controlServers[i] = u
	}
	return p, nil
}

func (r *ReverseProxy) getServer() *url.URL {
	index := randomRange(0, len(r.controlServers))
	return r.controlServers[index]
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	LogMessage("Serving Request %s %s", req.Method, req.RequestURI)

	// Ask control server for instructions
	instructions, httpError, err := getInstructions(p.getServer(), req)
	if err != nil {
		LogError("Failed to get instructions %s", err)
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
	// Now proxy the request
	outReq := rewriteRequest(upstream, req)

	// Now forward the reuest and mirror the response
	res, err := http.DefaultTransport.RoundTrip(outReq)
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
