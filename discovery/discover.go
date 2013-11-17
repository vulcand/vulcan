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
	responses, err := e.client.Get(key)
	if err != nil {
		return "", err
	}
	if len(responses) == 0 {
		return "", fmt.Errorf("Not found")
	}
	return responses[0].Value, nil
}
