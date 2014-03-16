package loadbalance

import (
	. "github.com/mailgun/vulcan/callback"
	. "github.com/mailgun/vulcan/request"
	. "github.com/mailgun/vulcan/upstream"
)

type LoadBalancer interface {
	NextUpstream(req Request) (Upstream, error)
	Before
	After
}
