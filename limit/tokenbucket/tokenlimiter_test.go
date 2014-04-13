package tokenbucket

import (
	"github.com/mailgun/gotools-time"
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
	l, err := NewClientIpLimiter(Settings{
		Rate:         Rate{Units: 1, Period: time.Second},
		TimeProvider: s.tm,
	})
	c.Assert(err, Equals, nil)
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)

	// Next request from the same ip hits rate limit
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// Second later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestFailure(c *C) {
	l, err := NewClientIpLimiter(Settings{
		Rate:         Rate{Units: 1, Period: time.Second},
		TimeProvider: s.tm,
	})
	c.Assert(err, Equals, nil)
	_, err = l.Before(makeRequest(""))
	c.Assert(err, Not(Equals), nil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestInvalidParams(c *C) {
	// Not supplying time provider
	_, err := NewTokenLimiter(Settings{
		TimeProvider: nil,
	})
	c.Assert(err, Not(Equals), nil)

	// Not supplying mapper function
	_, err = NewTokenLimiter(Settings{
		Mapper:       nil,
		TimeProvider: s.tm,
	})
	c.Assert(err, Not(Equals), nil)
}

// Make sure rates from different ips are controlled separatedly
func (s *LimiterSuite) TestIsolation(c *C) {
	l, err := NewClientIpLimiter(Settings{
		Rate:         Rate{Units: 1, Period: time.Second},
		TimeProvider: s.tm,
	})
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)

	// Next request from the same ip hits rate limit
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// The request from other ip can proceed
	_, err = l.Before(makeRequest("1.2.3.5"))
	c.Assert(err, Equals, nil)
}

// Make sure that expiration works (Expiration is triggered after significant amount of time passes)
func (s *LimiterSuite) TestExpiration(c *C) {
	l, err := NewClientIpLimiter(Settings{
		Rate:         Rate{Units: 1, Period: time.Second},
		TimeProvider: s.tm,
	})
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)

	// Next request from the same ip hits rate limit
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// 24 hours later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(24 * time.Hour)
	_, err = l.Before(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
