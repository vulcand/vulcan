package vulcan

import (
	"fmt"
	"time"
)

// Rates stores the information on how many hits per
// period of time any endpoint can accept
type Rate struct {
	Value  int
	Period time.Duration
}

func NewRate(value int, period time.Duration) (*Rate, error) {
	if value <= 0 {
		return nil, fmt.Errorf("Value should be > 0")
	}
	if period < time.Second || period > 24*time.Hour {
		return nil, fmt.Errorf("Period should be within [1 second, 24 hours]")
	}
	return &Rate{Value: value, Period: period}, nil
}

func (r *Rate) String() string {
	return (time.Duration(r.Value) * r.Period).String()
}

// Calculates when this rate can be hit the next time from
// the given time t, assuming all the requests in the given
func (r *Rate) retrySeconds(now time.Time) int {
	return int(r.nextBucket(now).Unix() - now.Unix())
}

//Returns epochSeconds rounded to the rate period
//e.g. minutes rate would return epoch seconds with seconds set to zero
//hourly rate would return epoch seconds with minutes and seconds set to zero
func (r *Rate) currentBucket(t time.Time) time.Time {
	return t.Truncate(r.Period)
}

// Returns the epoch seconds of the begining of the next time bucket
func (r *Rate) nextBucket(t time.Time) time.Time {
	return r.currentBucket(t.Add(r.duration()))
}

// Returns the equivalent of the rate period in seconds
func (r *Rate) periodSeconds() int {
	return r.Value * int(r.Period/time.Second)
}

func (r *Rate) duration() time.Duration {
	return time.Duration(r.Value) * r.Period
}
