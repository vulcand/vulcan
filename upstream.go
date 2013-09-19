package vulcan

import (
	"net/url"
)

type Upstream struct {
	Url     *url.URL
	Rates   []*Rate
	Headers map[string][]string
}

func NewUpstream(inUrl string, rates []*Rate, headers map[string][]string) (*Upstream, error) {
	//To ensure that upstream is correct url
	parsedUrl, err := parseUrl(inUrl)
	if err != nil {
		return nil, err
	}

	return &Upstream{
		Url:     parsedUrl,
		Rates:   rates,
		Headers: headers,
	}, nil
}

func (upstream *Upstream) Id() string {
	url := &url.URL{
		Scheme: upstream.Url.Scheme,
		Host:   upstream.Url.Host,
		Path:   upstream.Url.Path,
	}
	return url.String()
}
