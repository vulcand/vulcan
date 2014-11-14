// Interfaces for request limiting
package limit

import "github.com/mailgun/vulcan/middleware"

// Limiter is an interface for request limiters (e.g. rate/connection) limiters
type Limiter interface {
	// In case if limiter wants to reject request, it should return http response
	// will be proxied to the client.
	// In case if limiter returns an error, it will be treated as a request error and will
	// potentially activate failure recovery and failover algorithms.
	// In case if lmimiter wants to delay request, it should return duration > 0
	// Otherwise limiter should return (0, nil) to allow request to proceed
	middleware.Middleware
}
