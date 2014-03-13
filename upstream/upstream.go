package upstream

import (
	"fmt"
	"github.com/mailgun/vulcan/netutils"
	"net/url"
)

type Upstream interface {
	GetId() string
	GetUrl() *url.URL
}

type UrlUpstream struct {
	url *url.URL
	id  string
}

func NewUpstreamFromString(in string) (*UrlUpstream, error) {
	url, err := netutils.ParseUrl(in)
	if err != nil {
		return nil, err
	}
	return &UrlUpstream{url: url, id: fmt.Sprintf("%s://%s", url.Scheme, url.Host)}, nil
}

func NewUpstream(in *url.URL) (*UrlUpstream, error) {
	if in == nil {
		return nil, fmt.Errorf("Provide upstream")
	}
	return &UrlUpstream{
		url: netutils.CopyUrl(in),
		id:  fmt.Sprintf("%s://%s", in.Scheme, in.Host)}, nil
}

func (u *UrlUpstream) GetId() string {
	return u.id
}

func (u *UrlUpstream) GetUrl() *url.URL {
	return u.url
}
