package vulcan

import (
	. "launchpad.net/gocheck"
	"time"
)

type MemoryBackendSuite struct {
	timeProvider *FreezedTime
	backend      *MemoryBackend
}

var _ = Suite(&MemoryBackendSuite{})

func (s *MemoryBackendSuite) SetUpTest(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &FreezedTime{CurrentTime: start}
	backend, err := NewMemoryBackend(s.timeProvider)
	c.Assert(err, IsNil)
	s.backend = backend
}

func (s *MemoryBackendSuite) TestUtcNow(c *C) {
	c.Assert(s.backend.utcNow(), Equals, s.timeProvider.CurrentTime)
}

func (s *MemoryBackendSuite) TestMemoryBackendGetSet(c *C) {
	counter, err := s.backend.getStats("key1", &Rate{1, time.Second})
	c.Assert(err, IsNil)
	c.Assert(counter, Equals, 0)

	err = s.backend.updateStats("key1", &Rate{1, time.Second}, 1)
	c.Assert(err, IsNil)

	counter, err = s.backend.getStats("key1", &Rate{1, time.Second})
	c.Assert(err, IsNil)
	c.Assert(counter, Equals, 1)
}
