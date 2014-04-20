package loadbalance

import (
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/middleware"
	. "github.com/mailgun/vulcan/request"
)

type LoadBalancer interface {
	NextEndpoint(req Request) (Endpoint, error)
	Middleware
	Observer
}
