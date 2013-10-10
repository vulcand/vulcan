package main

import (
	"flag"
	"fmt"
	"github.com/mailgun/gocql"
	"github.com/mailgun/vulcan"
	"regexp"
	"strconv"
	"time"
)

type ListOptions []string

func (o *ListOptions) String() string {
	return fmt.Sprint(*o)
}

func (o *ListOptions) Set(value string) error {
	*o = append(*o, value)
	return nil
}

type CleanupOptions struct {
	T *vulcan.CleanupTime
}

func (o *CleanupOptions) String() string {
	if o.T != nil {
		return fmt.Sprintf("%0d:%0d", o.T.Hour, o.T.Minute)
	}
	return "not set"
}

func (o *CleanupOptions) Set(value string) error {
	re := regexp.MustCompile(`(?P<hour>\d+):(?P<minute>\d+)`)
	values := re.FindStringSubmatch(value)
	if values == nil {
		return fmt.Errorf("Invalid format, expected HH:MM")
	}
	hour, err := strconv.Atoi(values[1])
	if err != nil {
		return err
	}
	minute, err := strconv.Atoi(values[2])
	if err != nil {
		return err
	}
	o.T = &vulcan.CleanupTime{Hour: hour, Minute: minute}
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
	cassandraServers        ListOptions
	cassandraKeyspace       string
	cassandraCleanup        bool
	cassandraCleanupOptions CleanupOptions

	// How often should we clean up golang old logs
	cleanupPeriod time.Duration
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

	flag.BoolVar(&options.cassandraCleanup, "cscleanup", false, "Whethere to perform periodic cassandra cleanups")
	flag.Var(&options.cassandraCleanupOptions, "cscleanuptime", "Cassandra cleanup utc time of day in form: HH:MM")

	flag.DurationVar(&options.cleanupPeriod, "logcleanup", time.Duration(24)*time.Hour, "How often should we remove unused golang logs (e.g. 24h, 1h, 7h)")

	flag.Parse()

	return options, nil
}

func initProxy(options *ServiceOptions) (*vulcan.ReverseProxy, error) {
	var backend vulcan.Backend
	var err error

	if options.backend == "cassandra" {
		cassandraConfig := &vulcan.CassandraConfig{
			Servers:       options.cassandraServers,
			Keyspace:      options.cassandraKeyspace,
			Consistency:   gocql.One,
			LaunchCleanup: options.cassandraCleanup,
			CleanupTime:   options.cassandraCleanupOptions.T,
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
