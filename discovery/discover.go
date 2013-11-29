package discovery

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
)

type Service interface {
	Get(key string) (string, error)
}

type Etcd struct {
	client *etcd.Client
}

func NewEtcd(machines []string) *Etcd {
	return &Etcd{
		client: etcd.NewClient(machines),
	}
}

func (e *Etcd) Get(key string) (string, error) {
	response, err := e.client.Get(key, false, true)
	if err != nil {
		return "", err
	}
	if response == nil {
		return "", fmt.Errorf("Not found")
	}
	return response.Value, nil
}
