package location

import (
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/loadbalance"
	"net/http"
)

// Location defines the load balancer and limiter
type Location interface {
	GetLoadBalancer() LoadBalancer // Load balancer provides upstreams
	GetLimiter() Limiter           // Custom rate limiter, may be null
	GetTransport() *http.Transport // Transport that provides customized timeout and connection settings
}
