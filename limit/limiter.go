package limit

import (
	. "github.com/mailgun/vulcan/callback"
	. "github.com/mailgun/vulcan/request"
	"time"
)

// Limiter is an interface for request limiters (e.g. rate/connection) limiters
type Limiter interface {
	// In case if limiter wants to reject request, it should return an error, this error
	// will be proxied to the client.
	// In case if lmimiter wants to delay request, it should return duration > 0
	// Otherwise limiter should return (0, nil) to allow request to proceed
	Limit(r Request) (time.Duration, error)
	Before
	After
}
