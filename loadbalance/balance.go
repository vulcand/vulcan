package loadbalance

import (
	. "github.com/mailgun/vulcan/callback"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/request"
)

type LoadBalancer interface {
	NextEndpoint(req Request) (Endpoint, error)
	Before
	After
}
