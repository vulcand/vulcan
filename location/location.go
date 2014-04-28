package location

import (
	"github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcan/request"
	"net/http"
)

// Location accepts proxy request and returns response or results in error
type Location interface {
	GetId() string
	RoundTrip(Request) (*http.Response, error)
}

// Lcation used in tests
type Loc struct {
	Id   string
	Name string
}

func (*Loc) RoundTrip(Request) (*http.Response, error) {
	return nil, nil
}

func (l *Loc) GetId() string {
	return l.Id
}

// The simplest HTTP location implementation that adds no additional logic
// on top of simple http round trip function call
type ConstHttpLocation struct {
	Url string
}

func (l *ConstHttpLocation) RoundTrip(r Request) (*http.Response, error) {
	req := r.GetHttpRequest()
	req.URL = netutils.MustParseUrl(l.Url)
	return http.DefaultTransport.RoundTrip(req)
}

func (l *ConstHttpLocation) GetId() string {
	return l.Url
}
