package limit

import (
	. "github.com/mailgun/vulcan/watch"
	"net/http"
	"time"
)

// Limiter is an interface for request limiters (e.g. rate/connection) limiters
type Limiter interface {
	// returns time after what this request can proceed
	// in case if request is allowed to go right away, returns 0
	Accept(r *http.Request) (time.Duration, error)
	// Allows Limiter to watch requests stats so it can make
	// limiting decisions based off that (e.g. request size/ip), etc.
	RequestWatcher
}
