package roundrobin

import (
	"fmt"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
)

type RoundRobin struct {
	mutex     *sync.Mutex
	index     int
	endpoints []Endpoint
}

func NewRoundRobin(endpoints ...Endpoint) *RoundRobin {
	rr := &RoundRobin{
		mutex: &sync.Mutex{},
	}
	rr.AddEndpoints(endpoints...)
	return rr
}

func (rr *RoundRobin) NextEndpoint(req Request) (Endpoint, error) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	for i := 0; i < len(rr.endpoints); i++ {
		u := rr.endpoints[rr.index]
		rr.index = (rr.index + 1) % len(rr.endpoints)
		return u, nil
	}
	// We did full circle and found nothing
	return nil, fmt.Errorf("No available endpoints!")
}

func (r *RoundRobin) AddEndpoints(endpoints ...Endpoint) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.endpoints = append(r.endpoints, endpoints...)
	r.index = 0
	return nil
}

func (rr *RoundRobin) RemoveEndpoints(endpoints ...Endpoint) error {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// Collect endpoints to remove
	indexes := make(map[int]bool)
	for _, r := range endpoints {
		for i, u := range rr.endpoints {
			if u.GetId() == r.GetId() {
				indexes[i] = true
			}
		}
	}

	// Iterate over endpoints and remove the indexes marked for deletion
	idx := 0
	newEndpoints := make([]Endpoint, len(rr.endpoints)-len(indexes))
	for i, u := range rr.endpoints {
		if !indexes[i] {
			newEndpoints[idx] = u
			idx += 1
		}
	}
	rr.endpoints = newEndpoints
	// Reset the index because it's obviously invalid now
	rr.index = 0
	return nil
}

func (r *RoundRobin) Before(Request) (*http.Response, error) {
	return nil, nil
}

func (r *RoundRobin) After(Request, *http.Response, error) error {
	return nil
}
