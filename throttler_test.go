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

func (s *ThrottlerSuite) TestThrottlerUpstreamsNoRates(c *C) {
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

func (s *ThrottlerSuite) TestThrottlerRatesClear(c *C) {
	instructions := &ProxyInstructions{
		Upstreams: []*Upstream{
			ExpectUpstream("http://google.com", []*Rate{&Rate{1, time.Second}}, http.Header{}),
			ExpectUpstream("http://yahoo.com", []*Rate{&Rate{10, time.Minute}}, http.Header{}),
		},
	}

	upstreamStats, _, err := s.throttler.throttle(instructions)

	c.Assert(err, IsNil)
	c.Assert(2, Equals, len(upstreamStats))
	c.Assert(upstreamStats[0].upstream.Id(), Equals, "http://google.com")
	c.Assert(upstreamStats[1].upstream.Id(), Equals, "http://yahoo.com")
}

func (s *ThrottlerSuite) TestThrottlerRatesOneUpstreamOut(c *C) {
	instructions := &ProxyInstructions{
		Upstreams: []*Upstream{
			ExpectUpstream("http://google.com", []*Rate{&Rate{1, time.Second}}, http.Header{}),
			ExpectUpstream("http://yahoo.com", []*Rate{&Rate{10, time.Minute}}, http.Header{}),
		},
	}

	u1 := instructions.Upstreams[0]
	err := s.backend.updateStats(u1.Id(), u1.Rates[0], 1)
	c.Assert(err, IsNil)

	upstreamStats, _, err := s.throttler.throttle(instructions)

	c.Assert(1, Equals, len(upstreamStats))
	c.Assert(upstreamStats[0].upstream.Id(), Equals, "http://yahoo.com")
}

func (s *ThrottlerSuite) TestThrottlerRatesAllUpstreamsOut(c *C) {
	instructions := &ProxyInstructions{
		Upstreams: []*Upstream{
			ExpectUpstream("http://google.com", []*Rate{&Rate{1, time.Second}}, http.Header{}),
			ExpectUpstream("http://yahoo.com", []*Rate{&Rate{10, time.Minute}}, http.Header{}),
		},
	}

	up := instructions.Upstreams[0]
	err := s.backend.updateStats(up.Id(), up.Rates[0], 1)
	c.Assert(err, IsNil)

	up = instructions.Upstreams[1]
	err = s.backend.updateStats(up.Id(), up.Rates[0], 12)
	c.Assert(err, IsNil)

	upstreamStats, retrySeconds, err := s.throttler.throttle(instructions)

	c.Assert(err, IsNil)
	c.Assert(0, Equals, len(upstreamStats))
	c.Assert(1, Equals, retrySeconds)
}
