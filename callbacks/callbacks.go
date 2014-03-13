package callbacks

import (
	. "github.com/mailgun/vulcan/upstream"
	"net/http"
)

// Called by proxy before the request is going to be proxied to the upstream
// In case if function Before returns an error, request will be rejected
// This is handy in cases if users would like to introduce some auth middleware
type Before interface {
	Before(upstream Upstream, req *http.Request, attempt int) error
}

// Called by proxy right after the request was executed and response was received,
// Can be used to alter the response from upstream.
type After interface {
	After(upstream Upstream, req *http.Request, response *http.Response) error
}
