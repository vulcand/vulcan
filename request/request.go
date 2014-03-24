package request

import (
	"github.com/mailgun/vulcan/netutils"
	"net/http"
)

// Wrapper around http request that provides more info about http.Request
type Request interface {
	GetHttpRequest() *http.Request // Original http request
	GetId() int64                  // Request id that is unique to this running process
	GetBody() netutils.MultiReader // Request body fully read and stored in effective manner (buffered to disk for large requests)
}

type BaseRequest struct {
	HttpRequest *http.Request
	Id          int64
	Body        netutils.MultiReader
}

func (br *BaseRequest) GetHttpRequest() *http.Request {
	return br.HttpRequest
}

func (br *BaseRequest) GetId() int64 {
	return br.Id
}

func (br *BaseRequest) GetBody() netutils.MultiReader {
	return br.Body
}
