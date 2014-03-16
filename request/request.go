package request

import (
	"github.com/mailgun/vulcan/upstream"
	"net/http"
	"time"
)

// Wrapper around http request that provides more info about http.Request
type Request interface {
	GetCurrentUpstream() upstream.Upstream // Returns upstream assigned to the reuqest by load balancer if any, can be nil
	GetHttpRequest() *http.Request         // Original http request
	GetId() int64                          // Request id that is unique to this running process
	GetHistory() []Attempt                 // History of attempts to proxy the request, can be empty
}

// Provides information about attempts to proxy the request to upstream
type Attempt struct {
	Upstream upstream.Upstream // Upstream used for proxying
	Error    error             // Error (can be nil)
	Duration time.Duration     // Recorded duration of the request
}

type BaseRequest struct {
	CurrentUpstream upstream.Upstream
	HttpRequest     *http.Request
	Id              int64
	History         []Attempt
}

func (br *BaseRequest) GetCurrentUpstream() upstream.Upstream {
	return br.CurrentUpstream
}

func (br *BaseRequest) GetHttpRequest() *http.Request {
	return br.HttpRequest
}

func (br *BaseRequest) GetId() int64 {
	return br.Id
}

func (br *BaseRequest) GetHistory() []Attempt {
	return br.History
}
