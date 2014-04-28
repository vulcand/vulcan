package httploc

import (
	timetools "github.com/mailgun/gotools-time"
	"github.com/mailgun/vulcan"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/loadbalance"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	. "github.com/mailgun/vulcan/route"
	. "github.com/mailgun/vulcan/testutils"
	. "launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type LocSuite struct {
	authHeaders http.Header
	tm          *timetools.FreezedTime
}

func Test(t *testing.T) { TestingT(t) }

var _ = Suite(&LocSuite{
	authHeaders: http.Header{
		"Authorization": []string{"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="},
	},
	tm: &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	},
})

func (s *LocSuite) newRoundRobin(endpoints ...string) LoadBalancer {
	rr, err := roundrobin.NewRoundRobinWithOptions(roundrobin.Options{TimeProvider: s.tm})
	if err != nil {
		panic(err)
	}
	for _, e := range endpoints {
		rr.AddEndpoint(MustParseUrl(e))
	}
	return rr
}

func (s *LocSuite) newProxyWithParams(
	l LoadBalancer,
	readTimeout time.Duration,
	dialTimeout time.Duration) *httptest.Server {

	location, err := NewLocationWithOptions("dummy", l, Options{
		TrustForwardHeader: true,
	})
	if err != nil {
		panic(err)
	}
	proxy, err := vulcan.NewProxy(&ConstRouter{
		Location: location,
	})
	if err != nil {
		panic(err)
	}
	return httptest.NewServer(proxy)
}

func (s *LocSuite) newProxy(l LoadBalancer) *httptest.Server {
	return s.newProxyWithParams(l, time.Duration(0), time.Duration(0))
}

// Success, make sure we've successfully proxied the response
func (s *LocSuite) TestSuccess(c *C) {
	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer server.Close()

	proxy := s.newProxy(s.newRoundRobin(server.URL))
	defer proxy.Close()

	response, bodyBytes := Get(c, proxy.URL, s.authHeaders, "hello!")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")
}
