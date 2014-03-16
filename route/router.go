package route

import (
	. "github.com/mailgun/vulcan/loadbalance"
	. "github.com/mailgun/vulcan/request"
)

// Router is the interface for routing incoming requests
// It takes incoming request as a parameter and returns load balancer that provides
// access to upstreams
type Router interface {
	// Matches request to a group of upstreams identified by string
	Route(req Request) (LoadBalancer, error)
}

// Helper router that always returns the same load balancer
type MatchAll struct {
	Balancer LoadBalancer
}

func (m *MatchAll) Route(req Request) (LoadBalancer, error) {
	return m.Balancer, nil
}
