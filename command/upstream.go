package command

import (
	"fmt"
	"github.com/mailgun/vulcan/netutils"
	"net/url"
	"strconv"
	"strings"
)

// Upstream is HTTP server that will actually serve
// the request that would be proxied
type Upstream struct {
	Scheme string
	Host   string
	Port   int
	Id     string
}

func NewUpstream(
	scheme string, host string, port int) (*Upstream, error) {

	if len(scheme) == 0 {
		return nil, fmt.Errorf("Expected scheme")
	}

	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("Unsupported scheme: %s", scheme)
	}

	return &Upstream{
		Id:     fmt.Sprintf("%s://%s:%d", scheme, host, port),
		Scheme: scheme,
		Host:   host,
		Port:   port,
	}, nil
}

func NewUpstreamFromUrl(url *url.URL) (*Upstream, error) {
	if url == nil {
		return nil, fmt.Errorf("Someone provided nil url. How dare you?")
	}
	values := strings.SplitN(url.Host, ":", 2)
	if len(values) != 2 {
		return nil, fmt.Errorf("Expected host and port in %s", url)
	}
	host := values[0]
	port, err := strconv.Atoi(values[1])
	if err != nil {
		return nil, fmt.Errorf("Expected numeric port in %s", url)
	}
	return NewUpstream(url.Scheme, host, port)
}

func NewUpstreamsFromUrls(hosts []string) ([]*Upstream, error) {
	upstreams := make([]*Upstream, len(hosts))
	for i, host := range hosts {
		u, err := NewUpstreamFromString(host)
		if err != nil {
			return nil, err
		}
		upstreams[i] = u
	}
	return upstreams, nil
}

func NewUpstreamsFromObj(in interface{}) ([]*Upstream, error) {
	upstreamsS, ok := in.([]interface{})
	if !ok {
		return nil, fmt.Errorf("Upstreams: expected array, got %T", in)
	}
	if len(upstreamsS) == 0 {
		return nil, fmt.Errorf("Upstreams: at least one is required")
	}
	upstreams := make([]*Upstream, len(upstreamsS))
	for i, upstreamI := range upstreamsS {
		u, err := NewUpstreamFromObj(upstreamI)
		if err != nil {
			return nil, err
		}
		upstreams[i] = u
	}
	return upstreams, nil
}

func NewUpstreamFromDict(in map[string]interface{}) (*Upstream, error) {
	schemeI, exists := in["scheme"]
	if !exists {
		return nil, fmt.Errorf("Expected scheme")
	}
	scheme, ok := schemeI.(string)
	if !ok {
		return nil, fmt.Errorf("Scheme should be a string")
	}

	hostI, exists := in["host"]
	if !exists {
		return nil, fmt.Errorf("Expected host")
	}
	host, ok := hostI.(string)
	if !ok {
		return nil, fmt.Errorf("Host should be a string")
	}

	portI, exists := in["port"]
	if !exists {
		return nil, fmt.Errorf("Expected port")
	}
	port, ok := portI.(float64)
	if !ok || port != float64(int(port)) {
		return nil, fmt.Errorf("Port should be an integer")
	}

	return NewUpstream(scheme, host, int(port))
}

func NewUpstreamFromString(in string) (*Upstream, error) {
	//To ensure that upstream is correct url
	parsedUrl, err := netutils.ParseUrl(in)
	if err != nil {
		return nil, err
	}
	return NewUpstreamFromUrl(parsedUrl)
}

func (u *Upstream) String() string {
	return fmt.Sprintf("Url(%s://%s:%d)", u.Scheme, u.Host, u.Port)
}

func NewUpstreamFromObj(in interface{}) (*Upstream, error) {
	switch val := in.(type) {
	case map[string]interface{}:
		return NewUpstreamFromDict(val)
	case string:
		return NewUpstreamFromString(val)
	default:
		return nil, fmt.Errorf("Unsupported type %T", val)
	}
}
