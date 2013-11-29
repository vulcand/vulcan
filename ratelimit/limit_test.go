package ratelimit

import (
	. "launchpad.net/gocheck"
	"testing"
)

func TestUtils(t *testing.T) { TestingT(t) }

type LimitSuite struct{}

var _ = Suite(&LimitSuite{})

func (s *LimitSuite) TestBasics(c *C) {
	c.Assert(true, Equals, true)
}
