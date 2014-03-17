package route

import (
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/loadbalance"
	. "github.com/mailgun/vulcan/request"
)

// Router matches incoming request to a specific location
type Router interface {
	// if error is not nil, the request wll be aborted
	// and error will be proxied to client
	Route(req Request) (Location, error)
}

// Location defines the load balancer and limiter
type Location interface {
	GetLoadBalancer() LoadBalancer
	GetLimiter() Limiter
}

type BaseLocation struct {
	LoadBalancer LoadBalancer
	Limiter      Limiter
}

func (b *BaseLocation) GetLoadBalancer() LoadBalancer {
	return b.LoadBalancer
}

func (b *BaseLocation) GetLimiter() Limiter {
	return b.Limiter
}

// Helper router that always returns the same load balancer
type MatchAll struct {
	Location Location
}

func (m *MatchAll) Route(req Request) (Location, error) {
	return m.Location, nil
}
