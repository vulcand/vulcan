package vulcan

import (
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/loadbalance"
	. "github.com/mailgun/vulcan/loadbalance/roundrobin"
	. "github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcan/route"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"
)

type ProxySuite struct {
	authHeaders http.Header
}

var _ = Suite(&ProxySuite{
	authHeaders: http.Header{
		"Authorization": []string{"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}}})

func (s *ProxySuite) SetUpTest(c *C) {
}

func (s *ProxySuite) Get(c *C, requestUrl string, header http.Header, body string) (*http.Response, []byte) {
	request, _ := http.NewRequest("GET", requestUrl, strings.NewReader(body))
	netutils.CopyHeaders(request.Header, header)
	request.Close = true
	// the HTTP lib treats Host as a special header.  it only respects the value on req.Host, and ignores
	// values in req.Headers
	if header.Get("Host") != "" {
		request.Host = header.Get("Host")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		c.Fatalf("Get: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Fatalf("Get body failed: %v", err)
	}
	return response, bodyBytes
}

func (s *ProxySuite) Post(c *C, requestUrl string, header http.Header, body url.Values) (*http.Response, []byte) {
	request, _ := http.NewRequest("POST", requestUrl, strings.NewReader(body.Encode()))
	netutils.CopyHeaders(request.Header, header)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Close = true
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		c.Fatalf("Post: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Fatalf("Post body failed: %v", err)
	}
	return response, bodyBytes
}

type WebHandler func(http.ResponseWriter, *http.Request)

func (s *ProxySuite) newServer(handler WebHandler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}

func (s *ProxySuite) newProxyWithParams(
	l LoadBalancer,
	r Limiter,
	readTimeout time.Duration,
	dialTimeout time.Duration) *httptest.Server {

	location, err := NewHttpLocation(HttpLocationSettings{LoadBalancer: l, Limiter: r})
	if err != nil {
		panic(err)
	}
	proxySettings := ProxySettings{
		Router: &MatchAll{
			Location: location,
		},
	}

	proxy, err := NewReverseProxy(proxySettings)
	if err != nil {
		panic(err)
	}
	return httptest.NewServer(proxy)
}

func (s *ProxySuite) newProxy(l LoadBalancer, r Limiter) *httptest.Server {
	return s.newProxyWithParams(l, r, time.Duration(0), time.Duration(0))
}

// Success, make sure we've successfully proxied the response
func (s *ProxySuite) TestSuccess(c *C) {
	server := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer server.Close()

	proxy := s.newProxy(NewRoundRobin(MustParseUrl(server.URL)), nil)
	defer proxy.Close()

	response, bodyBytes := s.Get(c, proxy.URL, s.authHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")
}
