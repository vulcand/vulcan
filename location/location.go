package location

import (
	. "github.com/mailgun/vulcan/request"
	"net/http"
)

// Location accepts proxy request and returns response or results in error
type Location interface {
	RoundTrip(Request) (*http.Response, error)
}

// Lcation used in tests
type Loc struct {
	Name string
}

func (*Loc) RoundTrip(Request) (*http.Response, error) {
	return nil, nil
}
