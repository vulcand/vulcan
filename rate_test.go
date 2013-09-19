package vulcan

import (
	. "launchpad.net/gocheck"
	"time"
)

func (s *MainSuite) TestNewRateSuccess(c *C) {
	rates := []struct {
		Value    int
		Period   time.Duration
		Expected Rate
	}{
		{
			Value:    1,
			Period:   time.Second,
			Expected: Rate{1, time.Second},
		},
		{
			Value:    10,
			Period:   time.Minute,
			Expected: Rate{10, time.Minute},
		},
	}

	for _, u := range rates {
		r, err := NewRate(u.Value, u.Period)
		c.Assert(err, IsNil)
		c.Assert(*r, Equals, u.Expected)
	}
}

func (s *MainSuite) TestNewRateFail(c *C) {
	rates := []struct {
		Value  int
		Period time.Duration
	}{
		//period too small
		{
			Value:  1,
			Period: time.Millisecond,
		},
		//period too large
		{
			Value:  1,
			Period: time.Hour * 25,
		},
		//Zero not allowed
		{
			Value:  0,
			Period: time.Hour,
		},
		//Negative numbers
		{
			Value:  -1,
			Period: time.Hour,
		},
	}

	for _, u := range rates {
		_, err := NewRate(u.Value, u.Period)
		c.Assert(err, NotNil)
	}
}

func (s *MainSuite) TestRateToString(c *C) {
	rates := []struct {
		Rate     Rate
		Expected string
	}{
		{
			Rate: Rate{
				Value:  1,
				Period: time.Second,
			},
			Expected: "1s",
		},
		{
			Rate: Rate{
				Value:  2,
				Period: time.Hour,
			},
			Expected: "2h0m0s",
		},
	}

	for _, u := range rates {
		c.Assert(u.Rate.String(), Equals, u.Expected)
	}
}

func (s *MainSuite) TestPeriodSecondsAndDuration(c *C) {
	rates := []struct {
		Rate     Rate
		Seconds  int
		Duration time.Duration
	}{
		{
			Rate: Rate{
				Value:  1,
				Period: time.Second,
			},
			Seconds:  1,
			Duration: time.Duration(time.Second) * time.Duration(1),
		},
		{
			Rate: Rate{
				Value:  2,
				Period: time.Hour,
			},
			Seconds:  7200,
			Duration: time.Duration(time.Hour) * time.Duration(2),
		},
		{
			Rate: Rate{
				Value:  10,
				Period: time.Minute,
			},
			Seconds:  600,
			Duration: time.Duration(time.Minute) * time.Duration(10),
		},
	}

	for _, u := range rates {
		c.Assert(u.Rate.periodSeconds(), Equals, u.Seconds)
	}
}

func (s *MainSuite) TestBuckets(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	startMinutes := time.Date(2012, 3, 4, 5, 6, 0, 0, time.UTC)

	rates := []struct {
		Rate          Rate
		CurrentBucket time.Time
		NextBucket    time.Time
	}{
		{
			Rate: Rate{
				Value:  1,
				Period: time.Second,
			},
			CurrentBucket: start,
			NextBucket:    start.Add(time.Second),
		},
		{
			Rate: Rate{
				Value:  1,
				Period: time.Minute,
			},
			CurrentBucket: startMinutes,
			NextBucket:    startMinutes.Add(time.Minute),
		},
		{
			Rate: Rate{
				Value:  10,
				Period: time.Minute,
			},
			CurrentBucket: startMinutes,
			NextBucket:    startMinutes.Add(time.Minute),
		},
	}

	for _, u := range rates {
		c.Assert(u.Rate.currentBucket(start), Equals, u.CurrentBucket)
		c.Assert(u.Rate.nextBucket(start), Equals, u.NextBucket)
	}
}

func (s *MainSuite) TestRetrySeconds(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)

	rates := []struct {
		Rate         Rate
		RetrySeconds int
	}{
		{
			Rate: Rate{
				Value:  1,
				Period: time.Second,
			},
			RetrySeconds: 1,
		},
		{
			Rate: Rate{
				Value:  1,
				Period: time.Minute,
			},
			RetrySeconds: 53,
		},
		{
			Rate: Rate{
				Value:  10,
				Period: time.Minute,
			},
			RetrySeconds: 53,
		},
		{
			Rate: Rate{
				Value:  1,
				Period: time.Hour,
			},
			RetrySeconds: 3233,
		},
	}

	for _, u := range rates {
		c.Assert(u.Rate.retrySeconds(start), Equals, u.RetrySeconds)
	}
}
