package vulcan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
)

type ProxySuite struct {
	timeProvider *FreezedTime
	backend      *MemoryBackend
	throttler    *Throttler
	loadBalancer LoadBalancer
	authHeaders  http.Header
}

var _ = Suite(&ProxySuite{})

func (s *ProxySuite) SetUpTest(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &FreezedTime{CurrentTime: start}
	backend, err := NewMemoryBackend(s.timeProvider)
	c.Assert(err, IsNil)
	s.backend = backend
	s.throttler = NewThrottler(s.backend)

	s.loadBalancer = NewRandomLoadBalancer()
	if err != nil {
		panic(err)
	}

	s.authHeaders = http.Header{"Authorization": []string{"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}}
}

func (s *ProxySuite) Get(c *C, requestUrl string, header http.Header, body string) (*http.Response, []byte) {
	request, _ := http.NewRequest("GET", requestUrl, strings.NewReader(body))
	copyHeaders(request.Header, header)
	request.Close = true
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

func (s *ProxySuite) newProxy(controlServers []*httptest.Server, b Backend, l LoadBalancer) *httptest.Server {
	controlUrls := make([]string, len(controlServers))
	for i, controlServer := range controlServers {
		controlUrls[i] = controlServer.URL
	}
	proxyHandler, err := NewReverseProxy(controlUrls, b, l)
	if err != nil {
		panic(err)
	}

	return httptest.NewServer(proxyHandler)
}

// This proxy requires authentication, so Authenticate header is required
func (s *ProxySuite) TestProxyAuthRequired(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm upstream"))
	})
	defer upstream.Close()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s"}]}`, upstream.URL)))
	})
	defer control.Close()

	proxy := s.newProxy([]*httptest.Server{control}, s.backend, s.loadBalancer)
	defer proxy.Close()

	response, bodyBytes := s.Get(c, proxy.URL, http.Header{}, "")
	c.Assert(response.StatusCode, Equals, http.StatusProxyAuthRequired)
	c.Assert(string(bodyBytes), Equals, http.StatusText(http.StatusProxyAuthRequired))
}

// Just success, make sure we've successfully proxied the response
func (s *ProxySuite) TestSuccess(c *C) {
	var queryValues map[string][]string
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm upstream"))
	})
	defer upstream.Close()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		queryValues = r.URL.Query()
		LogMessage("RAW query: %s", r.URL.RawQuery)
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s"}]}`, upstream.URL)))
	})
	defer control.Close()

	proxy := s.newProxy([]*httptest.Server{control}, s.backend, s.loadBalancer)
	defer proxy.Close()
	requestHeaders := http.Header{"X-Custom-Header": []string{"Bla"}}
	copyHeaders(requestHeaders, s.authHeaders)
	response, bodyBytes := s.Get(c, proxy.URL, requestHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm upstream")

	//now make sure that control request was correct
	c.Assert(queryValues, NotNil)
	c.Assert(queryValues["username"][0], Equals, "Aladdin")
	c.Assert(queryValues["password"][0], Equals, "open sesame")
	c.Assert(queryValues["protocol"][0], Equals, "HTTP/1.1")
	c.Assert(queryValues["method"][0], Equals, "GET")
	length, err := strconv.Atoi(queryValues["length"][0])
	c.Assert(err, IsNil)
	c.Assert(length, Equals, len("hello!"))

	headers := s.loadJson([]byte(queryValues["headers"][0]))
	value := headers["X-Custom-Header"]
	LogMessage("Value: %q", value)
}

// Make sure we've returned response with valid retry-seconds
func (s *ProxySuite) TestUpstreamThrottled(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm upstream"))
	})
	defer upstream.Close()

	// Upstream is out of capacity, we should be told to be throttled
	s.backend.updateStats(upstream.URL, &Rate{10, time.Minute}, 10)

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s", "rates": [{"value": 10, "period": "minute"}]}]}`, upstream.URL)))
	})
	defer control.Close()

	proxy := s.newProxy([]*httptest.Server{control}, s.backend, s.loadBalancer)
	defer proxy.Close()
	response, bodyBytes := s.Get(c, proxy.URL, s.authHeaders, "")
	c.Assert(response.StatusCode, Equals, 429)

	m := s.loadJson(bodyBytes)
	c.Assert(m["retry-seconds"], Equals, float64(53))
}

// Make sure we've falled back to the first available upstream if throttler went down
func (s *ProxySuite) TestUpstreamThrottlerDown(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm upstream!"))
	})
	defer upstream.Close()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s", "rates": [{"value": 10, "period": "minute"}]}]}`, upstream.URL)))
	})
	defer control.Close()

	proxy := s.newProxy([]*httptest.Server{control}, &FailingBackend{}, s.loadBalancer)
	defer proxy.Close()
	response, bodyBytes := s.Get(c, proxy.URL, s.authHeaders, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm upstream!")
}
