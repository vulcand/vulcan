package roundrobin

import (
	"fmt"
	"github.com/mailgun/vulcan/loadbalance"
	"net/http"
	"sync"
)

type RoundRobin struct {
	mutex  *sync.Mutex
	groups map[string]*group
	router loadbalance.Router
}

func NewRoundRobin(router loadbalance.Router) *RoundRobin {
	return &RoundRobin{
		router: router,
		groups: make(map[string]*group),
		mutex:  &sync.Mutex{},
	}
}

func (r *RoundRobin) NextUpstream(req *http.Request) (loadbalance.Upstream, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	groupId, err := r.router.Route(req)
	if err != nil {
		return nil, err
	}
	// Get existing cursor or create new cursor
	group, exists := r.groups[groupId]
	if !exists {
		return nil, fmt.Errorf("Upstream group(%s) not found", groupId)
	}
	return group.next()
}

func (r *RoundRobin) AddUpstreams(groupId string, upstreams ...loadbalance.Upstream) error {
	group, exists := r.groups[groupId]
	if !exists {
		group = newGroup()
		r.groups[groupId] = group
	}
	group.addUpstreams(upstreams)
	return nil
}

func (r *RoundRobin) RemoveUpstreams(groupId string, upstreams ...loadbalance.Upstream) error {
	group, exists := r.groups[groupId]
	if !exists {
		return nil
	}
	group.removeUpstreams(upstreams)
	return nil
}

func (r *RoundRobin) ReportFailure(u loadbalance.Upstream, err error) {
}
