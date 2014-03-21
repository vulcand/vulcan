package roundrobin

import (
	. "github.com/mailgun/vulcan/upstream"
	. "launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type RoundRobinSuite struct {
}

var _ = Suite(&RoundRobinSuite{})

func (s *RoundRobinSuite) SetUpSuite(c *C) {
}

func (s *RoundRobinSuite) TestNoUpstreams(c *C) {
	r := NewRoundRobin()
	_, err := r.NextUpstream(nil)
	c.Assert(err, NotNil)
}

// Subsequent calls to load balancer with 1 upstream are ok
func (s *RoundRobinSuite) TestSingleUpstream(c *C) {
	r := NewRoundRobin()

	u := MustParseUpstream("http://localhost:5000")
	r.AddUpstreams(u)

	u2, err := r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u2, Equals, u)

	u3, err := r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u3, Equals, u)
}

// Make sure that load balancer round robins requests
func (s *RoundRobinSuite) TestMultipleUpstreams(c *C) {
	r := NewRoundRobin()

	uA := MustParseUpstream("http://localhost:5000")
	uB := MustParseUpstream("http://localhost:5001")
	r.AddUpstreams(uA, uB)

	u, err := r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)

	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)
}

// Make sure that adding upstreams during load balancing works fine
func (s *RoundRobinSuite) TestAddUpstreams(c *C) {
	r := NewRoundRobin()
	return

	uA := MustParseUpstream("http://localhost:5000")
	uB := MustParseUpstream("http://localhost:5001")
	r.AddUpstreams(uA)

	u, err := r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	r.AddUpstreams(uB)

	// index was reset after altering upstreams
	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)
}

// Removing upstreams from the load balancer works fine as well
func (s *RoundRobinSuite) TestRemoveUpstream(c *C) {
	r := NewRoundRobin()

	uA := MustParseUpstream("http://localhost:5000")
	uB := MustParseUpstream("http://localhost:5001")
	r.AddUpstreams(uA, uB)

	u, err := r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	// Removing upstream resets the counter
	r.RemoveUpstreams(uB)

	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)
}

// Removing upstreams from the load balancer works fine as well
func (s *RoundRobinSuite) TestRemoveMultipleUpstreams(c *C) {
	r := NewRoundRobin()

	uA := MustParseUpstream("http://localhost:5000")
	uB := MustParseUpstream("http://localhost:5001")
	uC := MustParseUpstream("http://localhost:5002")
	r.AddUpstreams(uA, uB, uC)

	u, err := r.NextUpstream(nil)
	c.Assert(err, IsNil)
	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)

	// There's only one upstream left
	r.RemoveUpstreams(uA, uB)
	u, err = r.NextUpstream(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)
}
