package vulcan

import "net/url"

type LoadBalancer interface {
	sortedUpstreamsByStats(stats []*UpstreamStats) ([]*Upstream, error)
	sortedUpstreams(upstreams []*Upstream) ([]*Upstream, error)
	sortedUrls(urls []*url.URL) ([]*url.URL, error)
}
