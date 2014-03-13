package route

import (
	"net/http"
)

// Router is the interface for routing incoming requests
// It takes incoming request as a parameter and returns string
// that represents the routing group that request should go to
// the group can be any string, e.g. group of Loadbalance configs
type Router interface {
	// Matches request to a group of upstreams identified by string
	Route(req *http.Request) (string, error)
}

// Matches all requests to the specified
type MatchAll struct {
	Group string
}

func (sr *MatchAll) Route(req *http.Request) (string, error) {
	return sr.Group, nil
}
