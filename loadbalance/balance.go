package loadbalance

import (
	"net/http"
)

type LoadBalancer interface {
	NextUpstream(req *http.Request) (Upstream, error)
	ReportFailure(u Upstream, err error)
}
