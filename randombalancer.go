package vulcan

import (
	"fmt"
	"math/rand"
	"net/url"
	"time"
)

type RandomLoadBalancer struct {
}

func NewRandomLoadBalancer() *RandomLoadBalancer {
	rand.Seed(time.Now().UTC().UnixNano())
	return &RandomLoadBalancer{}
}

func (lb *RandomLoadBalancer) sortedUpstreamsByStats(stats []*UpstreamStats) ([]*Upstream, error) {
	in := make([]*Upstream, len(stats))
	for index, s := range stats {
		in[index] = s.upstream
	}
	return lb.sortedUpstreams(in)
}

func (lb *RandomLoadBalancer) sortedUpstreams(upstreams []*Upstream) ([]*Upstream, error) {
	if len(upstreams) <= 0 {
		return nil, fmt.Errorf("Need something to shuffle")
	}
	out := make([]*Upstream, len(upstreams))
	indexes := rand.Perm(len(upstreams))
	for index, shuffled := range indexes {
		out[index] = upstreams[shuffled]
	}
	return out, nil
}

func (lb *RandomLoadBalancer) sortedUrls(urls []*url.URL) ([]*url.URL, error) {
	if len(urls) <= 0 {
		return nil, fmt.Errorf("Need something to shuffle")
	}
	out := make([]*url.URL, len(urls))
	indexes := rand.Perm(len(urls))
	for index, shuffled := range indexes {
		out[index] = urls[shuffled]
	}
	return out, nil
}
