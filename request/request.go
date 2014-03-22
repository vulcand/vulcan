package request

import (
	"github.com/mailgun/vulcan/endpoint"
	"net/http"
	"time"
)

// Wrapper around http request that provides more info about http.Request
type Request interface {
	GetCurrentEndpoint() endpoint.Endpoint // Returns endpoint assigned to the reuqest by load balancer if any, can be nil
	GetHttpRequest() *http.Request         // Original http request
	GetId() int64                          // Request id that is unique to this running process
	GetAttempts() []Attempt                // History of attempts to proxy the request, can be empty
}

// Provides information about attempts to proxy the request to endpoint
type Attempt struct {
	Endpoint endpoint.Endpoint // Endpoint used for proxying
	Error    error             // Error (can be nil)
	Duration time.Duration     // Recorded duration of the request
}

type BaseRequest struct {
	CurrentEndpoint endpoint.Endpoint
	HttpRequest     *http.Request
	Id              int64
	History         []Attempt
}

func (br *BaseRequest) GetCurrentEndpoint() endpoint.Endpoint {
	return br.CurrentEndpoint
}

func (br *BaseRequest) GetHttpRequest() *http.Request {
	return br.HttpRequest
}

func (br *BaseRequest) GetId() int64 {
	return br.Id
}

func (br *BaseRequest) GetAttempts() []Attempt {
	return br.History
}

func (br *BaseRequest) AddAttempt(a Attempt) {
	br.History = append(br.History, a)
}
