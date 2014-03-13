package watch

import (
	"net/http"
)

type RequestWatcher interface {
	RequestStarted(r *http.Request)
	// Should be called by the user to notify that this request has been completed
	// error can be provided if the request has been resulted in error
	// and the response if there was any
	RequestEnded(req *http.Request, re *http.Response, err error)
}
