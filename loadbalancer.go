package vulcan

import (
	"fmt"
	"math/rand"
	"time"
)

type LoadBalancer interface {
	chooseUpstream([]*UpstreamStats) (*Upstream, error)
}

type RandomLoadBalancer struct {
}

func NewRandomLoadBalancer() *RandomLoadBalancer {
	rand.Seed(time.Now().UTC().UnixNano())
	return &RandomLoadBalancer{}
}

func (lb *RandomLoadBalancer) chooseUpstream(stats []*UpstreamStats) (*Upstream, error) {
	if len(stats) <= 0 {
		return nil, fmt.Errorf("Having no upstreams to choose from is not ok")
	}
	index := randomRange(0, len(stats))
	return stats[index].upstream, nil
}
