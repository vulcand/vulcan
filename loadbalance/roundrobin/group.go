package roundrobin

import (
	"fmt"
	. "github.com/mailgun/vulcan/upstream"
)

type group struct {
	index     int
	upstreams []Upstream
}

func newGroup() *group {
	return &group{}
}

func (g *group) next() (Upstream, error) {
	for i := 0; i < len(g.upstreams); i++ {
		u := g.upstreams[g.index]
		g.index = (g.index + 1) % len(g.upstreams)
		return u, nil
	}
	// That means that we did full circle and found nothing
	return nil, fmt.Errorf("No available endpoints!")
}

func (g *group) addUpstreams(upstreams []Upstream) {
	g.upstreams = append(g.upstreams, upstreams...)
}

func (g *group) removeUpstreams(upstreams []Upstream) {
	// Collect upstreams to remove
	indexes := make(map[int]bool)
	for _, r := range upstreams {
		for i, u := range g.upstreams {
			if u.GetId() == r.GetId() {
				indexes[i] = true
			}
		}
	}

	// Iterate over upstreams and remove the indexes marked for deletion
	idx := 0
	newUpstreams := make([]Upstream, len(g.upstreams)-len(indexes))
	for i, u := range g.upstreams {
		if !indexes[i] {
			newUpstreams[idx] = u
			idx += 1
		}
	}
	g.upstreams = newUpstreams
	g.index = 0
}
