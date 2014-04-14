package tokenbucket

import (
	"github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcan/request"
	. "launchpad.net/gocheck"
	"net/http"
	"time"
)

type LimiterSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&LimiterSuite{})

func (s *LimiterSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

// We've hit the limit and were able to proceed on the next time run
func (s *LimiterSuite) TestHitLimit(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})

	c.Assert(err, IsNil)
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// Second later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestFailure(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})
	c.Assert(err, IsNil)
	_, err = l.Before(makeRequest(""))
	c.Assert(err, NotNil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestInvalidParams(c *C) {
	_, err := NewTokenLimiter(nil, Rate{})
	c.Assert(err, NotNil)
}

// Make sure rates from different ips are controlled separatedly
func (s *LimiterSuite) TestIsolation(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})

	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// The request from other ip can proceed
	_, err = l.Before(makeRequest("1.2.3.5"))
	c.Assert(err, IsNil)
}

// Make sure that expiration works (Expiration is triggered after significant amount of time passes)
func (s *LimiterSuite) TestExpiration(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})

	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// 24 hours later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(24 * time.Hour)
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)
}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
