package route

import (
	. "github.com/mailgun/vulcan/location"
	. "github.com/mailgun/vulcan/request"
)

// Router matches incoming request to a specific location
type Router interface {
	// if error is not nil, the request wll be aborted
	// and error will be proxied to client
	Route(req Request) (Location, error)
}

// Helper router that always returns the same load balancer
type MatchAll struct {
	Location Location
}

func (m *MatchAll) Route(req Request) (Location, error) {
	return m.Location, nil
}
