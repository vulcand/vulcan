package discovery

import (
	"fmt"
	"github.com/golang/glog"
	"net/url"
	"strings"
)

type Service interface {
	Get(key string) ([]string, error)
}

type DisabledDiscovery struct{}

func NewDisabledDiscovery() *DisabledDiscovery {
	return &DisabledDiscovery{}
}

func (d *DisabledDiscovery) Get(serviceName string) ([]string, error) {
	return []string{}, nil
}

func New(discoveryUrl string) (Service, error) {

	if !strings.Contains(discoveryUrl, ":") {
		discoveryUrl = discoveryUrl + "://"
	}

	u, err := url.Parse(discoveryUrl)

	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "disabled":
		return NewDisabledDiscovery(), nil
	case "rackspace":
		return NewRackspaceFromUrl(u)
	case "etcd":
		hosts := strings.Split(u.Host, ",")
		return NewEtcd(hosts), nil
	default:
		glog.Errorf("Bad URL for discovery: %s", discoveryUrl)
		return nil, fmt.Errorf("invalid configuration: Unknown discovery scheme: %s", u.Scheme)
	}

	return nil, nil
}
