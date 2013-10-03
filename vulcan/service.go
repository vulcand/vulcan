package main

import (
	"flag"
	"fmt"
	"github.com/mailgun/gocql"
	"github.com/mailgun/vulcan"
)

type ListOptions []string

func (o *ListOptions) String() string {
	return fmt.Sprint(*o)
}

func (o *ListOptions) Set(value string) error {
	*o = append(*o, value)
	return nil
}

type ServiceOptions struct {
	// Pid path
	pidPath string
	// Control servers to bind to
	controlServers ListOptions
	backend        string
	loadBalancer   string

	// Host and port to bind to
	host     string
	httpPort int
	// Cassandra specific stuff
	cassandraServers  ListOptions
	cassandraKeyspace string
}

func parseOptions() (*ServiceOptions, error) {
	options := &ServiceOptions{}

	flag.Var(&options.controlServers, "c", "HTTP control server url")
	flag.StringVar(&options.backend, "b", "memory", "Backend type e.g. 'cassandra' or 'memory'")
	flag.StringVar(&options.loadBalancer, "lb", "cassandra", "Loadbalancer algo, e.g. 'random'")

	flag.StringVar(&options.host, "h", "localhost", "Host to bind to")
	flag.IntVar(&options.httpPort, "p", 8080, "HTTP port to bind to")

	flag.StringVar(&options.pidPath, "pid", "", "pid file path")

	flag.Var(&options.cassandraServers, "csnode", "Cassandra nodes to connect to")
	flag.StringVar(&options.cassandraKeyspace, "cskeyspace", "", "Cassandra keyspace")

	flag.Parse()

	return options, nil
}

func initProxy(options *ServiceOptions) (*vulcan.ReverseProxy, error) {
	var backend vulcan.Backend
	var err error

	if options.backend == "cassandra" {
		cassandraConfig := vulcan.CassandraConfig{
			Servers:     options.cassandraServers,
			Keyspace:    options.cassandraKeyspace,
			Consistency: gocql.One,
		}
		backend, err = vulcan.NewCassandraBackend(
			cassandraConfig, &vulcan.RealTime{})
		if err != nil {
			return nil, err
		}
	} else if options.backend == "memory" {
		backend, err = vulcan.NewMemoryBackend(&vulcan.RealTime{})
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("Unsupported backend")
	}

	var loadBalancer vulcan.LoadBalancer
	if options.loadBalancer == "random" {
		loadBalancer = vulcan.NewRandomLoadBalancer()
	} else {
		return nil, fmt.Errorf("Unsupported loadbalancing algo")
	}

	proxySettings := &vulcan.ProxySettings{
		ControlServers:   options.controlServers,
		ThrottlerBackend: backend,
		LoadBalancer:     loadBalancer,
	}

	return vulcan.NewReverseProxy(proxySettings)
}
