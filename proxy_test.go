package vulcan

import (
	"encoding/json"
	. "github.com/mailgun/vulcan/loadbalance"
	. "github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/netutils"
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

func (s *ProxySuite) loadJson(bytes []byte) map[string]interface{} {
	var replyObject interface{}
	err := json.Unmarshal(bytes, &replyObject)
	if err != nil {
		panic(err)
	}
	return replyObject.(map[string]interface{})
}

func (s *ProxySuite) newProxyWithTimeouts(
	l LoadBalancer,
	readTimeout time.Duration,
	dialTimeout time.Duration) *httptest.Server {

	proxySettings := ProxySettings{
		LoadBalancer:    l,
		HttpReadTimeout: readTimeout,
		HttpDialTimeout: dialTimeout,
	}

	proxy, err := NewReverseProxy(proxySettings)
	if err != nil {
		panic(err)
	}
	return httptest.NewServer(proxy)
}

func (s *ProxySuite) newProxy(l LoadBalancer) *httptest.Server {
	return s.newProxyWithTimeouts(l, time.Duration(0), time.Duration(0))
}

func (s *ProxySuite) newUpstream(url string) Upstream {
	u, err := NewUpstreamFromString(url)
	if err != nil {
		panic(err)
	}
	return u
}

// Success, make sure we've successfully proxied the response
func (s *ProxySuite) TestSuccess(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm upstream"))
	})
	defer upstream.Close()

	lb := NewRoundRobin(&MatchAll{Group: "*"})
	lb.AddUpstreams("*", s.newUpstream(upstream.URL))
	proxy := s.newProxy(lb)
	defer proxy.Close()

	response, bodyBytes := s.Get(c, proxy.URL, s.authHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm upstream")
}
