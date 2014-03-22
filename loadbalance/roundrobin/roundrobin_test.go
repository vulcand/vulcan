package roundrobin

import (
	. "github.com/mailgun/vulcan/endpoint"
	. "launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type RoundRobinSuite struct {
}

var _ = Suite(&RoundRobinSuite{})

func (s *RoundRobinSuite) SetUpSuite(c *C) {
}

func (s *RoundRobinSuite) TestNoEndpoints(c *C) {
	r := NewRoundRobin()
	_, err := r.NextEndpoint(nil)
	c.Assert(err, NotNil)
}

// Subsequent calls to load balancer with 1 endpoint are ok
func (s *RoundRobinSuite) TestSingleEndpoint(c *C) {
	r := NewRoundRobin()

	u := MustParseUrl("http://localhost:5000")
	r.AddEndpoints(u)

	u2, err := r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u2, Equals, u)

	u3, err := r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u3, Equals, u)
}

// Make sure that load balancer round robins requests
func (s *RoundRobinSuite) TestMultipleEndpoints(c *C) {
	r := NewRoundRobin()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoints(uA, uB)

	u, err := r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)

	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)
}

// Make sure that adding endpoints during load balancing works fine
func (s *RoundRobinSuite) TestAddEndpoints(c *C) {
	r := NewRoundRobin()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoints(uA)

	u, err := r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	r.AddEndpoints(uB)

	// index was reset after altering endpoints
	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)
}

// Removing endpoints from the load balancer works fine as well
func (s *RoundRobinSuite) TestRemoveEndpoint(c *C) {
	r := NewRoundRobin()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoints(uA, uB)

	u, err := r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	// Removing endpoint resets the counter
	r.RemoveEndpoints(uB)

	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)
}

// Removing endpoints from the load balancer works fine as well
func (s *RoundRobinSuite) TestRemoveMultipleEndpoints(c *C) {
	r := NewRoundRobin()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	uC := MustParseUrl("http://localhost:5002")
	r.AddEndpoints(uA, uB, uC)

	u, err := r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)

	// There's only one endpoint left
	r.RemoveEndpoints(uA, uB)
	u, err = r.NextEndpoint(nil)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)
}
