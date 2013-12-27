package discovery

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/rackspace/gophercloud"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Rackspace struct {
	accessProvider gophercloud.AccessProvider
	mu             sync.RWMutex
	servers        []gophercloud.Server
	protocol       string
	port           string
	region         string
	metadataKey    string
}

const DEFAULT_REGION = "dfw"
const DEFAULT_PORT = ""
const DEFAULT_PROTOCOL = "http"
const DEFAULT_METADATA_KEY = "rax:auto_scaling_group_id"

func NewRackspaceFromUrl(u *url.URL) (*Rackspace, error) {
	var port = DEFAULT_PORT
	var region = DEFAULT_REGION
	var protocol = DEFAULT_PROTOCOL
	var metadataKey = DEFAULT_METADATA_KEY
	var username = ""
	var apiKey = ""

	if os.Getenv("OS_REGION_NAME") != "" {
		region = strings.ToLower(os.Getenv("OS_REGION_NAME"))
	}

	qs := u.Query()

	if qs.Get("port") != "" {
		port = qs.Get("port")
	}

	if qs.Get("protocol") != "" {
		protocol = qs.Get("protocol")
	}

	if qs.Get("region") != "" {
		region = strings.ToLower(qs.Get("region"))
	}

	if qs.Get("metadatakey") != "" {
		metadataKey = qs.Get("metadatakey")
	}

	if os.Getenv("OS_USERNAME") != "" {
		username = os.Getenv("OS_USERNAME")
	}

	if os.Getenv("OS_PASSWORD") != "" {
		apiKey = os.Getenv("OS_PASSWORD")
	}

	if username == "" && u.User != nil {
		username = u.User.Username()
	} else if username == "" && u.User == nil {
		return nil, fmt.Errorf("Missing Username for Rackspace provider.")
	}

	if apiKey == "" && u.User != nil {
		var ok bool
		apiKey, ok = u.User.Password()
		if !ok {
			return nil, fmt.Errorf("Missing API Key for Rackspace provider.")
		}
	}

	auth := gophercloud.AuthOptions{
		Username:    username,
		ApiKey:      apiKey,
		AllowReauth: true}

	var identityRegion = "rackspace-us"

	if region == "lon" {
		identityRegion = "rackspace-uk"
	}

	ap, err := gophercloud.Authenticate(identityRegion, auth)

	if err != nil {
		return nil, err
	}

	r := &Rackspace{accessProvider: ap,
		region:      region,
		port:        port,
		protocol:    protocol,
		metadataKey: metadataKey,
	}

	err = r.UpdateCache()

	if err != nil {
		return nil, err
	}

	go r.Watch()

	return r, nil
}

func (r *Rackspace) UpdateCache() error {
	api, err := gophercloud.ServersApi(r.accessProvider,
		gophercloud.ApiCriteria{
			Name:      "cloudServersOpenStack",
			VersionId: "2",
			UrlChoice: gophercloud.PublicURL,
			Region:    r.region,
		})

	if err != nil {
		return err
	}

	servers, err := api.ListServers()

	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = servers

	return nil
}

func (r *Rackspace) Watch() {
	for {
		// TOOD: expotential backoff on no changes?
		time.Sleep(30000 * time.Millisecond)

		err := r.UpdateCache()
		if err != nil {
			glog.Errorf("Error fetching servers from Rackspace: %v", err)
		}
	}
}

func (r *Rackspace) serverToUrl(s gophercloud.Server) string {
	// TODO: support servicenet?
	if r.port != "" {
		return fmt.Sprintf("%s://%s:%s/", r.protocol, s.AccessIPv4, r.port)
	}

	return fmt.Sprintf("%s://%s/", r.protocol, s.AccessIPv4)
}

func (r *Rackspace) Get(serviceName string) ([]string, error) {
	var out = []string{}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, s := range r.servers {
		if s.Status != "ACTIVE" {
			continue
		}
		if val, ok := s.Metadata[r.metadataKey]; ok {
			if strings.Contains(val, serviceName) {
				us := r.serverToUrl(s)
				out = append(out, us)
			}
		}
	}

	return out, nil
}
