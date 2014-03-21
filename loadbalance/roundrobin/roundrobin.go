package roundrobin

import (
	"fmt"
	. "github.com/mailgun/vulcan/request"
	. "github.com/mailgun/vulcan/upstream"
	"net/http"
	"sync"
)

type RoundRobin struct {
	mutex     *sync.Mutex
	index     int
	upstreams []Upstream
}

func NewRoundRobin(upstreams ...Upstream) *RoundRobin {
	rr := &RoundRobin{
		mutex: &sync.Mutex{},
	}
	rr.AddUpstreams(upstreams...)
	return rr
}

func (rr *RoundRobin) NextUpstream(req Request) (Upstream, error) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	for i := 0; i < len(rr.upstreams); i++ {
		u := rr.upstreams[rr.index]
		rr.index = (rr.index + 1) % len(rr.upstreams)
		return u, nil
	}
	// We did full circle and found nothing
	return nil, fmt.Errorf("No available endpoints!")
}

func (r *RoundRobin) AddUpstreams(upstreams ...Upstream) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.upstreams = append(r.upstreams, upstreams...)
	r.index = 0
	return nil
}

func (rr *RoundRobin) RemoveUpstreams(upstreams ...Upstream) error {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// Collect upstreams to remove
	indexes := make(map[int]bool)
	for _, r := range upstreams {
		for i, u := range rr.upstreams {
			if u.GetId() == r.GetId() {
				indexes[i] = true
			}
		}
	}

	// Iterate over upstreams and remove the indexes marked for deletion
	idx := 0
	newUpstreams := make([]Upstream, len(rr.upstreams)-len(indexes))
	for i, u := range rr.upstreams {
		if !indexes[i] {
			newUpstreams[idx] = u
			idx += 1
		}
	}
	rr.upstreams = newUpstreams
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
