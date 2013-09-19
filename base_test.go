/*
Declares gocheck's test suites
*/
package vulcan

import (
	. "launchpad.net/gocheck"
	"testing"
	"time"
)

func Test(t *testing.T) { TestingT(t) }

//This is a simple suite to use if tests dont' need anything
//special
type MainSuite struct {
	timeProvider *FreezedTime
}

func (s *MainSuite) SetUpTest(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &FreezedTime{CurrentTime: start}
}

var _ = Suite(&MainSuite{})
