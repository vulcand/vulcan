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
	delay, err := l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Next request from the same ip hits rate limit
	delay, err = l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// Second later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	delay, err = l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))
}

// We've failed to extract client ip
func (s *LimiterSuite) TestFailure(c *C) {
	l, err := NewClientIpLimiter(Settings{
		Rate:         Rate{Units: 1, Period: time.Second},
		TimeProvider: s.tm,
	})
	c.Assert(err, Equals, nil)
	_, err = l.Limit(makeRequest(""))
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
	delay, err := l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Next request from the same ip hits rate limit
	delay, err = l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// The request from other ip can proceed
	delay, err = l.Limit(makeRequest("1.2.3.5"))
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))
}

// Make sure that expiration works (Expiration is triggered after significant amount of time passes)
func (s *LimiterSuite) TestExpiration(c *C) {
	l, err := NewClientIpLimiter(Settings{
		Rate:         Rate{Units: 1, Period: time.Second},
		TimeProvider: s.tm,
	})
	delay, err := l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Next request from the same ip hits rate limit
	delay, err = l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Not(Equals), nil)

	// 24 hours later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(24 * time.Hour)
	delay, err = l.Limit(makeRequest("1.2.3.4"))
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
