package callbacks

import (
	"github.com/mailgun/vulcan/loadbalance"
	"net/http"
)

type Before interface {
	Before(upstream loadbalance.Upstream, req *http.Request, attempt int) error
}

type After interface {
	After(upstream loadbalance.Upstream, req *http.Request, response *http.Response) error
}
