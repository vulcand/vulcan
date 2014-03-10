package loadbalance

import (
	"net/http"
)

type Router interface {
	// Takes request and returns upstream group to return
	Route(req *http.Request) (string, error)
}

type MatchAll struct {
	Group string
}

func (sr *MatchAll) Route(req *http.Request) (string, error) {
	return sr.Group, nil
}
