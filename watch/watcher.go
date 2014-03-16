package watch

import (
	. "github.com/mailgun/vulcan/request"
	"net/http"
)

type RequestWatcher interface {
	RequestStarted(Request)
	// Should be called by the user to notify that this request has been completed
	// error can be provided if the request has been resulted in error
	// and the response if there was any
	RequestEnded(Request, *http.Response, error)
}
