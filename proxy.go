// This package contains the proxy core - the main proxy function that accepts and modifies
// request, forwards or denies it.
package vulcan

import (
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcan/route"
	"io"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

// Reverse proxy settings, what loadbalancing algo to use,
// timeouts, rate limiting backend
type Options struct {
	// Formatter that takes a status code and formats it into proxy response
	ErrorFormatter errors.Formatter
	// Router decides where does the request go and what load balancer handles it
	Router route.Router
}

type Proxy struct {
	// Router defines where does request go
	router route.Router
	// Connection settings, load balancing algo to use, callbacks and watchers
	options Options
	// Counter that is used to provide unique identifiers for requests
	lastRequestId int64
}

func NewProxy(router route.Router) (*Proxy, error) {
	return NewProxyWithOptions(router, Options{})
}

// Creates reverse proxy that acts like http server.
func NewProxyWithOptions(router route.Router, o Options) (*Proxy, error) {
	o, err := validateOptions(o)
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		options: o,
		router:  router,
	}
	return p, nil
}

// Main request handler, accepts requests, round trips it to the endpoint and writes backe the response.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infof("Serving Request %s %s", r.Method, r.RequestURI)

	// We are allowed to fallback in case of endpoint failure,
	// record the request body so we can replay it on errors.
	body, err := netutils.NewBodyBuffer(r.Body)
	if err != nil {
		log.Errorf("Request read error %s", err)
		p.replyError(p.options.ErrorFormatter.FromStatus(http.StatusBadRequest), w, r)
	}
	defer body.Close()
	r.Body = body

	// Wrap the original request into wrapper with more detailed information available
	req := &request.BaseRequest{
		HttpRequest: r,
		Id:          atomic.AddInt64(&p.lastRequestId, 1),
		Body:        body,
	}

	err = p.proxyRequest(w, req)
	if err != nil {
		log.Errorf("Failed to proxy request:", err)
		p.replyError(p.options.ErrorFormatter.FromStatus(http.StatusBadGateway), w, r)
	}
}

// Round trips the request to one of the endpoints, returns the streamed
// request body length in bytes and the endpoint reply.
func (p *Proxy) proxyRequest(w http.ResponseWriter, req *request.BaseRequest) error {

	location, err := p.router.Route(req)
	if err != nil {
		return err
	}

	response, err := location.RoundTrip(req)
	if response != nil {
		netutils.CopyHeaders(w.Header(), response.Header)
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
		defer response.Body.Close()
		return nil
	} else {
		return err
	}
}

// Helper function to reply with http errors
func (p *Proxy) replyError(err errors.HttpError, w http.ResponseWriter, req *http.Request) {
	// Discard the request body, so that clients can actually receive the response
	// Otherwise they can only see lost connection
	// TODO: actually check this
	io.Copy(ioutil.Discard, req.Body)
	w.Header().Set("Content-Type", err.GetContentType())
	w.WriteHeader(err.GetStatusCode())
	w.Write(err.GetBody())
}

func validateOptions(o Options) (Options, error) {
	if o.ErrorFormatter == nil {
		o.ErrorFormatter = &errors.JsonFormatter{}
	}
	return o, nil
}
