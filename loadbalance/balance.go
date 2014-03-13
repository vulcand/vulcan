package loadbalance

import (
	. "github.com/mailgun/vulcan/upstream"
	"net/http"
)

type LoadBalancer interface {
	NextUpstream(req *http.Request) (Upstream, error)
	ReportFailure(u Upstream, err error)
}
