package vulcan

import (
	"github.com/mailgun/vulcan/loadbalance"
	"net/url"
)

type Endpoint struct {
	upstream *Upstream
	id       string
	active   bool
}

func newEndpoint(upstream *Upstream, active bool) *Endpoint {
	normalizedUrl := &url.URL{
		Scheme: upstream.Url.Scheme,
		Host:   upstream.Url.Host,
		Path:   upstream.Url.Path,
	}
	return &Endpoint{
		id:       normalizedUrl.String(),
		upstream: upstream,
		active:   active,
	}
}

func (e *Endpoint) Id() string {
	return e.id
}

func (e *Endpoint) IsActive() bool {
	return e.active
}

func endpointsFromUpstreams(upstreams []*Upstream) []loadbalance.Endpoint {
	endpoints := make([]loadbalance.Endpoint, len(upstreams))
	for i, upstream := range upstreams {
		endpoints[i] = newEndpoint(upstream, true)
	}
	return endpoints
}

func endpointsFromStats(upstreamStats []*UpstreamStats) []loadbalance.Endpoint {
	endpoints := make([]loadbalance.Endpoint, len(upstreamStats))
	for i, us := range upstreamStats {
		endpoints[i] = newEndpoint(us.upstream, !us.ExceededLimits())
	}
	return endpoints
}
