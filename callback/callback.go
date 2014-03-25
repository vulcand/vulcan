package callback

import (
	. "github.com/mailgun/vulcan/request"
	"net/http"
)

// Called by proxy before the request is going to be proxied to the upstream selected by the load balancer
// In case if function Before returns an error, request will be rejected
// This is handy in cases if users would like to introduce some auth middleware
// In case if function returns a non nil response, proxy will return the response without proxying to the upstream
// In case if function returns nil response and nil error request will be proxied right away
// It's ok to modify request headers and body in the callback
type Before interface {
	Before(r Request) (*http.Response, error)
}

// Called by proxy right after the request was executed and response was received,
// Can be used to modify the response from upstream.
type After interface {
	// If request has been completed and response has been received,
	// response will be non nil, otherwise err will be non nil
	// In case if after callback returns error, this error will be streamed
	// to the client instead of the response or any other error and no other
	// callbacks will be executed
	After(r Request) error
}
