package command

import (
	. "launchpad.net/gocheck"
	"testing"
	"time"
)

func TestRates(t *testing.T) { TestingT(t) }

type RateSuite struct{}

var _ = Suite(&RateSuite{})

func (s *RateSuite) TestNewRateSuccess(c *C) {
	rates := []struct {
		Units    int64
		Period   time.Duration
		UnitType int
		Expected *Rate
	}{
		{
			Units:    2,
			Period:   time.Second,
			UnitType: UnitTypeRequests,
			Expected: &Rate{Units: 2, Period: time.Second},
		},
		{
			Units:    10,
			UnitType: UnitTypeMegabytes,
			Period:   time.Minute,
			Expected: &Rate{Units: 10, Period: time.Minute, UnitType: UnitTypeMegabytes},
		},
	}

	for _, in := range rates {
		r, err := NewRate(in.Units, in.Period, in.UnitType)
		c.Assert(err, IsNil)
		c.Assert(*r, Equals, *in.Expected)
	}
}
