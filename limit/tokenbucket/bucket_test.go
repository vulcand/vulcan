package tokenbucket

import (
	"github.com/mailgun/gotools-time"
	. "launchpad.net/gocheck"
	"testing"
	"time"
)

func Test(t *testing.T) { TestingT(t) }

type BucketSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&BucketSuite{})

func (s *BucketSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *BucketSuite) TestRate(c *C) {
	l, err := NewTokenBucket(Rate{1, time.Second}, 1, s.tm)
	c.Assert(err, Equals, nil)

	// First request passes
	delay, err := l.Limit(nil)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Next request does not pass the same second
	delay, err = l.Limit(nil)
	c.Assert(err, Not(Equals), nil)

	// Second later, the request passes
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	delay, err = l.Limit(nil)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Five seconds later, still only one request is allowed
	// because maxBurst is 1
	s.tm.CurrentTime = s.tm.CurrentTime.Add(5 * time.Second)
	delay, err = l.Limit(nil)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// The next one is forbidden
	delay, err = l.Limit(nil)
	c.Assert(err, Not(Equals), nil)
}
