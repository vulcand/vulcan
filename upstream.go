package vulcan

import (
	"encoding/json"
	"net/url"
)

// This object is used for unmarshalling
// from json
type UpstreamObj struct {
	Url     string
	Rates   []RateObj
	Headers map[string][]string
}

type Upstream struct {
	Url     *url.URL
	Rates   []Rate
	Headers map[string][]string
}

func NewUpstream(inUrl string, rates []Rate, headers map[string][]string) (*Upstream, error) {
	//To ensure that upstream is correct url
	parsedUrl, err := url.Parse(inUrl)
	if err != nil {
		return nil, err
	}

	return &Upstream{
		Url:     parsedUrl,
		Rates:   rates,
		Headers: headers,
	}, nil
}

func upstreamFromJson(bytes []byte) (*Upstream, error) {
	var u UpstreamObj
	err := json.Unmarshal(bytes, &u)
	if err != nil {
		return nil, err
	}

	rates, err := ratesFromJsonList(u.Rates)
	if err != nil {
		return nil, err
	}
	return NewUpstream(u.Url, rates, u.Headers)
}

func upstreamsFromJsonList(inUpstreams []UpstreamObj) ([]Upstream, error) {
	upstreams := make([]Upstream, len(inUpstreams))
	for i, upstreamObj := range inUpstreams {
		rates, err := ratesFromJsonList(upstreamObj.Rates)
		if err != nil {
			return nil, err
		}
		upstream, err := NewUpstream(
			upstreamObj.Url, rates, upstreamObj.Headers)
		if err != nil {
			return nil, err
		}
		upstreams[i] = *upstream
	}
	return upstreams, nil
}
