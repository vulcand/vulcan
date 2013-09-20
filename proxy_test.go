package vulcan

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"time"
)

type ProxySuite struct {
	timeProvider     *FreezedTime
	backend          *MemoryBackend
	throttler        *Throttler
	failingThrottler *Throttler
	loadBalancer     LoadBalancer
}

var _ = Suite(&ProxySuite{})

func (s *ProxySuite) SetUpTest(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &FreezedTime{CurrentTime: start}
	backend, err := NewMemoryBackend(s.timeProvider)
	c.Assert(err, IsNil)
	s.backend = backend
	s.throttler = NewThrottler(s.backend)

	s.failingThrottler = NewThrottler(&FailingBackend{})

	s.loadBalancer = NewRandomLoadBalancer()
	if err != nil {
		panic(err)
	}
}

func (s *ProxySuite) Get(c *C, requestUrl string) (*http.Response, []byte) {
	request, _ := http.NewRequest("GET", requestUrl, nil)
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

func (s *ProxySuite) setUpProxy(c *C, controlHandler WebHandler, backendHandler WebHandler) (*httptest.Server, *httptest.Server, *httptest.Server) {
	backend := httptest.NewServer(http.HandlerFunc(backendHandler))
	defer backend.Close()

	control := httptest.NewServer(http.HandlerFunc(controlHandler))
	defer control.Close()

	proxyHandler, err := NewReverseProxy([]string{control.URL}, s.backend, s.loadBalancer)
	if err != nil {
		c.Fatal(err)
	}

	proxy := httptest.NewServer(proxyHandler)

	return control, backend, proxy
}

func (s *ProxySuite) TestProxyMailformedControlReply(c *C) {

	control, backend, proxy := s.setUpProxy(c,
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hi"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello, I'm control server"))
		})

	defer backend.Close()
	defer control.Close()
	defer proxy.Close()

	response, _ := s.Get(c, proxy.URL)
	c.Assert(response.StatusCode, Equals, http.StatusInternalServerError)
}
