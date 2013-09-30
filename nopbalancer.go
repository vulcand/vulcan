package vulcan

import (
	"net/url"
)

// As name suggests, does nothing, returns
// all values as is. Used for testing.
type NopLoadBalancer struct {
}

func (lb *NopLoadBalancer) sortedUpstreamsByStats(stats []*UpstreamStats) ([]*Upstream, error) {
	in := make([]*Upstream, len(stats))
	for index, s := range stats {
		in[index] = s.upstream
	}
	return in, nil
}

func (lb *NopLoadBalancer) sortedUpstreams(upstreams []*Upstream) ([]*Upstream, error) {
	return upstreams, nil
}

func (lb *NopLoadBalancer) sortedUrls(urls []*url.URL) ([]*url.URL, error) {
	return urls, nil
}
