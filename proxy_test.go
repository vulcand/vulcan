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

	proxySettings := &ProxySettings{
		ControlServers:   controlUrls,
		ThrottlerBackend: b,
		LoadBalancer:     l,
	}

	proxyHandler, err := NewReverseProxy(proxySettings)
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

// This proxy requires authentication, so Authenticate header is required
func (s *ProxySuite) TestProxyAccessDenied(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access denied, sorry"))
	})
	defer upstream.Close()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s"}]}`, upstream.URL)))
	})
	defer control.Close()

	proxy := s.newProxy([]*httptest.Server{control}, s.backend, s.loadBalancer)
	defer proxy.Close()

	response, bodyBytes := s.Get(c, proxy.URL, s.authHeaders, "")
	c.Assert(response.StatusCode, Equals, http.StatusForbidden)
	c.Assert(string(bodyBytes), Equals, "Access denied, sorry")
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
	c.Assert(fmt.Sprintf("%s", value), Equals, "[Bla]")
}

// Make sure upstream headers were added to the request
func (s *ProxySuite) TestUpstreamHeaders(c *C) {
	var customHeaders http.Header

	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		customHeaders = r.Header
		w.Write([]byte("Hi, I'm upstream"))
	})
	defer upstream.Close()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s", "headers": {"X-Header-A": ["val"], "X-Header-B": ["val2"]}}]}`, upstream.URL)))
	})
	defer control.Close()

	proxy := s.newProxy([]*httptest.Server{control}, s.backend, s.loadBalancer)
	defer proxy.Close()
	response, bodyBytes := s.Get(c, proxy.URL, s.authHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm upstream")

	// make sure the headers are set
	c.Assert(customHeaders["X-Header-A"][0], Equals, "val")
	c.Assert(customHeaders["X-Header-B"][0], Equals, "val2")
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

func (s *ProxySuite) TestProxyControlServerWrongUrl(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access denied, sorry"))
	})
	defer upstream.Close()

	proxySettings := &ProxySettings{
		ControlServers:   []string{"whoa"},
		ThrottlerBackend: s.backend,
		LoadBalancer:     s.loadBalancer,
	}
	proxyHandler, err := NewReverseProxy(proxySettings)
	if err != nil {
		c.Assert(err, IsNil)
	}
	proxy := httptest.NewServer(proxyHandler)
	defer proxy.Close()

	response, _ := s.Get(c, proxy.URL, s.authHeaders, "")
	c.Assert(response.StatusCode, Equals, http.StatusInternalServerError)
}

func (s *ProxySuite) TestProxyControlServerUnreachableControlServer(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access denied, sorry"))
	})
	defer upstream.Close()

	proxySettings := &ProxySettings{
		ControlServers:   []string{"http://localhost:9999"},
		ThrottlerBackend: s.backend,
		LoadBalancer:     s.loadBalancer,
	}

	proxyHandler, err := NewReverseProxy(proxySettings)
	if err != nil {
		c.Assert(err, IsNil)
	}
	proxy := httptest.NewServer(proxyHandler)
	defer proxy.Close()

	response, _ := s.Get(c, proxy.URL, s.authHeaders, "")
	c.Assert(response.StatusCode, Equals, http.StatusInternalServerError)
}

func (s *ProxySuite) TestUpstreamUpstreamIsDown(c *C) {
	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"upstreams": [{"url": "http://localhost:9999", "rates": [{"value": 10, "period": "minute"}]}]}`))
	})
	defer control.Close()

	proxySettings := &ProxySettings{
		ControlServers:   []string{control.URL},
		ThrottlerBackend: s.backend,
		LoadBalancer:     s.loadBalancer,
	}
	proxyHandler, err := NewReverseProxy(proxySettings)
	if err != nil {
		c.Assert(err, IsNil)
	}
	proxy := httptest.NewServer(proxyHandler)
	defer proxy.Close()

	response, _ := s.Get(c, proxy.URL, s.authHeaders, "")
	c.Assert(response.StatusCode, Equals, http.StatusBadGateway)
}

// Make sure proxy gives up when control server is too slow
func (s *ProxySuite) TestControlServerTimeout(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm upstream"))
	})
	defer upstream.Close()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * time.Duration(100))
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s"}]}`, upstream.URL)))
	})
	// Do not call Close as it will hang for 100 seconds
	defer control.CloseClientConnections()

	proxySettings := &ProxySettings{
		ControlServers:   []string{control.URL},
		ThrottlerBackend: s.backend,
		LoadBalancer:     s.loadBalancer,
		HttpReadTimeout:  time.Duration(1) * time.Millisecond,
		HttpDialTimeout:  time.Duration(1) * time.Millisecond,
	}
	proxyHandler, err := NewReverseProxy(proxySettings)
	if err != nil {
		c.Assert(err, IsNil)
	}
	proxy := httptest.NewServer(proxyHandler)
	defer proxy.Close()

	response, _ := s.Get(c, proxy.URL, s.authHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusInternalServerError)
}

// The same story with upstream, if upstream is too slow we should give up
func (s *ProxySuite) TestUpstreamServerTimeout(c *C) {
	upstream := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * time.Duration(100))
		w.Write([]byte("Hi, I'm upstream"))
	})
	// Do not call Close as it will hang for 100 seconds
	defer upstream.CloseClientConnections()

	control := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"upstreams": [{"url": "%s"}]}`, upstream.URL)))
	})
	defer control.Close()

	proxySettings := &ProxySettings{
		ControlServers:   []string{control.URL},
		ThrottlerBackend: s.backend,
		LoadBalancer:     s.loadBalancer,
		HttpReadTimeout:  time.Duration(1) * time.Millisecond,
		HttpDialTimeout:  time.Duration(1) * time.Millisecond,
	}
	proxyHandler, err := NewReverseProxy(proxySettings)
	if err != nil {
		c.Assert(err, IsNil)
	}
	proxy := httptest.NewServer(proxyHandler)
	defer proxy.Close()

	response, _ := s.Get(c, proxy.URL, s.authHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusBadGateway)
}
