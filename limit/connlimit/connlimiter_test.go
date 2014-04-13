package connlimit

import (
	"github.com/mailgun/vulcan/request"
	. "launchpad.net/gocheck"
	"net/http"
	"testing"
)

func TestConn(t *testing.T) { TestingT(t) }

type ConnLimiterSuite struct {
}

var _ = Suite(&ConnLimiterSuite{})

func (s *ConnLimiterSuite) SetUpSuite(c *C) {
}

// We've hit the limit and were able to proceed once the request has completed
func (s *ConnLimiterSuite) TestHitLimitAndRelease(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)

	r := makeRequest("1.2.3.4")

	_, err = l.Before(r)
	c.Assert(err, Equals, nil)

	// Next request from the same ip hits rate limit, because the active connections > 1
	_, err = l.Before(r)
	c.Assert(err, Not(Equals), nil)

	// Once the first request finished, next one succeeds
	err = l.After(r)
	c.Assert(err, Equals, nil)

	_, err = l.Before(r)
	c.Assert(err, Equals, nil)
}

// Make sure connections are counted independently for different ips
func (s *ConnLimiterSuite) TestDifferentIps(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)

	r := makeRequest("1.2.3.4")
	r2 := makeRequest("1.2.3.5")

	_, err = l.Before(r)
	c.Assert(err, Equals, nil)

	_, err = l.Before(r)
	c.Assert(err, Not(Equals), nil)

	_, err = l.Before(r2)
	c.Assert(err, Equals, nil)
}

// Make sure connections are counted independently for different ips
func (s *ConnLimiterSuite) TestConnectionCount(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)

	r := makeRequest("1.2.3.4")
	r2 := makeRequest("1.2.3.5")

	_, err = l.Before(r)
	c.Assert(err, Equals, nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(1))

	_, err = l.Before(r)
	c.Assert(err, Not(Equals), nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(1))

	_, err = l.Before(r2)
	c.Assert(err, Equals, nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(2))

	err = l.After(r)
	c.Assert(err, Equals, nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(1))

	err = l.After(r2)
	c.Assert(err, Equals, nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(0))
}

// We've failed to extract client ip, everything crashes, bam!
func (s *ConnLimiterSuite) TestFailure(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)
	_, err = l.Before(makeRequest(""))
	c.Assert(err, Not(Equals), nil)
}

func (s *ConnLimiterSuite) TestWrongParams(c *C) {
	_, err := NewConnectionLimiter(nil, 1)
	c.Assert(err, Not(Equals), nil)

	_, err = NewClientIpLimiter(0)
	c.Assert(err, Not(Equals), nil)

	_, err = NewClientIpLimiter(-1)
	c.Assert(err, Not(Equals), nil)
}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
