package vulcan

import (
	"io"
	"net/http"
	"net/url"
)

type ReverseProxy struct {
	AuthServers []*url.URL
	throttler   *Throttler
}

func NewReverseProxy(authServers []string, throttlerConfig ThrottlerConfig) (*ReverseProxy, error) {
	throttler, err := NewThrottler(throttlerConfig)
	if err != nil {
		return nil, err
	}

	p := &ReverseProxy{
		AuthServers: make([]*url.URL, len(authServers)),
		throttler:   throttler,
	}
	for i, str := range authServers {
		u, err := url.Parse(str)
		if err != nil {
			return nil, err
		}
		p.AuthServers[i] = u
	}
	return p, nil
}

func (r *ReverseProxy) getServer() *url.URL {
	index := RandomRange(0, len(r.AuthServers))
	return r.AuthServers[index]
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	LogMessage("Serving Request %s", req.RequestURI)

	//create auth request from the incoming request
	authRequest, err := FromHttpRequest(req)
	if err != nil {
		LogError("Failed to create auth request, got error: %s", err)
		p.WriteError(w, NewInternalError())
		return
	}

	//Ask auth server for directions
	authResponse, httpError := authRequest.authorize(p.getServer())
	if httpError != nil {
		//Auth server has rejected the request
		LogError("Failed to execute auth request, got error: %s", httpError)
		p.WriteError(w, httpError)
		return
	}

	// choosing an upstream
	p.throttler.getUpstream(authResponse)

	// Now choose an upstream (temporarily choose one)
	outReq := rewriteRequest(&authResponse.Upstreams[0], req)

	// Now forward the reuest and mirror the response
	res, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		LogError("Upstream error: %v", err)
		p.WriteError(w, NewUpstreamError())
		return
	}
	defer res.Body.Close()
	copyHeaders(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
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

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (p *ReverseProxy) WriteError(w http.ResponseWriter, err *HttpError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)
	w.Write(err.Body)
}
