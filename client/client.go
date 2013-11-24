package client

import (
	"github.com/mailgun/vulcan/netutils"
	"net/http"
)

type MultiDict map[string][]string

type Client interface {
	Get(w http.ResponseWriter, hosts []string, query MultiDict, auth *netutils.BasicAuth) error
}

func (d MultiDict) Add(key string, value string) {
	vals, exist := d[key]
	if !exist {
		d[key] = []string{value}
	} else {
		d[key] = append(vals, value)
	}
}
