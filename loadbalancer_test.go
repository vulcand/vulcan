package vulcan

import (
	. "launchpad.net/gocheck"
)

func (s *MainSuite) TestRandomLoadBalancerFailsOnNoUpstreams(c *C) {
	lb := NewRandomLoadBalancer()

	stats := []*UpstreamStats{}
	_, err := lb.chooseUpstream(stats)
	c.Assert(err, NotNil)
}

func (s *MainSuite) TestRandomLoadBalancerSingleUpstream(c *C) {
	lb := NewRandomLoadBalancer()

	u, err := NewUpstream("http://google.com", nil, nil)
	c.Assert(err, IsNil)

	stats := []*UpstreamStats{
		&UpstreamStats{
			upstream: u,
		},
	}
	chosen, err := lb.chooseUpstream(stats)
	c.Assert(err, IsNil)

	c.Assert(u, Equals, chosen)
}

func (s *MainSuite) TestRandomLoadBalancerMutlipleUpstreams(c *C) {
	lb := NewRandomLoadBalancer()

	u, err := NewUpstream("http://google.com", nil, nil)
	c.Assert(err, IsNil)

	u2, err := NewUpstream("http://yahoo.com", nil, nil)
	c.Assert(err, IsNil)

	stats := []*UpstreamStats{
		&UpstreamStats{
			upstream: u,
		},
		&UpstreamStats{
			upstream: u2,
		},
	}
	chosen, err := lb.chooseUpstream(stats)
	c.Assert(err, IsNil)
	c.Assert(chosen, NotNil)
}
