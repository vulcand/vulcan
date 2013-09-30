package vulcan

import (
	. "launchpad.net/gocheck"
	"net/url"
)

func (s *MainSuite) TestRandomLoadBalancerFailsOnNoData(c *C) {
	lb := NewRandomLoadBalancer()

	stats := []*UpstreamStats{}
	_, err := lb.sortedUpstreamsByStats(stats)
	c.Assert(err, NotNil)

	upstreams := []*Upstream{}
	_, err = lb.sortedUpstreams(upstreams)
	c.Assert(err, NotNil)

	urls := []*url.URL{}
	_, err = lb.sortedUrls(urls)
	c.Assert(err, NotNil)
}

func (s *MainSuite) TestRandomLoadBalancerSortedUpstreams(c *C) {
	lb := NewRandomLoadBalancer()

	// To test shuffling of upstreams
	upstream1, err := NewUpstream("http://google.com", nil, nil)
	c.Assert(err, IsNil)
	upstream2, err := NewUpstream("http://yahoo.com", nil, nil)
	c.Assert(err, IsNil)

	// to test shuffling of stats
	stats := []*UpstreamStats{
		&UpstreamStats{
			upstream: upstream1,
		},
		&UpstreamStats{
			upstream: upstream2,
		},
	}

	sorted, err := lb.sortedUpstreamsByStats(stats)
	c.Assert(err, IsNil)
	c.Assert(sorted, NotNil)
	c.Assert(len(sorted), Equals, len(stats))

	count1, count2 := 0, 0
	for _, s := range sorted {
		c.Assert(s, NotNil)
		if s == upstream1 {
			count1 += 1
		} else {
			count2 += 1
		}
	}

	c.Assert(count1, Equals, 1)
	c.Assert(count2, Equals, 1)
}

func (s *MainSuite) TestRandomLoadBalancerSortedUrls(c *C) {
	lb := NewRandomLoadBalancer()

	// To test shuffling of urls
	url1, err := url.Parse("http://google.com")
	c.Assert(err, IsNil)
	url2, err := url.Parse("http://yahoo.com")
	c.Assert(err, IsNil)
	urls := []*url.URL{url1, url2}

	sorted, err := lb.sortedUrls(urls)
	c.Assert(err, IsNil)
	c.Assert(sorted, NotNil)
	c.Assert(len(sorted), Equals, len(urls))

	count1, count2 := 0, 0
	for _, s := range sorted {
		c.Assert(s, NotNil)
		if s == url1 {
			count1 += 1
		} else {
			count2 += 1
		}
	}

	c.Assert(count1, Equals, 1)
	c.Assert(count2, Equals, 1)
}
