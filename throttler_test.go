package vulcan

import (
	. "launchpad.net/gocheck"
	"net/http"
	"time"
)

type ThrottlerSuite struct {
	timeProvider *FreezedTime
	backend      *MemoryBackend
	throttler    *Throttler
}

var _ = Suite(&ThrottlerSuite{})

func (s *ThrottlerSuite) SetUpTest(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &FreezedTime{CurrentTime: start}
	backend, err := NewMemoryBackend(s.timeProvider)
	c.Assert(err, IsNil)
	s.backend = backend
	s.throttler = NewThrottler(s.backend)
}

func (s *ThrottlerSuite) TestThrottlerUpstreamsRatesClear(c *C) {
	instructions := &ProxyInstructions{
		Upstreams: []*Upstream{
			ExpectUpstream("http://google.com", []*Rate{}, http.Header{}),
			ExpectUpstream("http://yahoo.com", []*Rate{}, http.Header{}),
		},
	}

	upstreamStats, _, err := s.throttler.throttle(instructions)

	c.Assert(err, IsNil)
	c.Assert(2, Equals, len(upstreamStats))
	c.Assert(upstreamStats[0].upstream.Id(), Equals, "http://google.com")
	c.Assert(upstreamStats[1].upstream.Id(), Equals, "http://yahoo.com")
}
