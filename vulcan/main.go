package main

import (
	"github.com/golang/glog"
	"github.com/mailgun/gocql"
	"github.com/mailgun/vulcan"
	"net/http"
	"time"
)

func main() {
	glog.Info("Vulcan starting")
	controlServers := []string{"http://localhost:5000/auth"}
	cassandraConfig := vulcan.CassandraConfig{
		Servers:     []string{"localhost"},
		Keyspace:    "vulcan_dev",
		Consistency: gocql.One,
	}
	backend, err := vulcan.NewCassandraBackend(
		cassandraConfig,
		&vulcan.RealTime{})
	if err != nil {
		glog.Fatalf("Failed to init proxy, error:", err)
	}

	loadBalancer := vulcan.NewRandomLoadBalancer()
	if err != nil {
		glog.Fatalf("Failed to init proxy, error:", err)
	}

	proxySettings := &vulcan.ProxySettings{
		ControlServers:   controlServers,
		ThrottlerBackend: backend,
		LoadBalancer:     loadBalancer,
	}

	handler, err := vulcan.NewReverseProxy(proxySettings)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if err != nil {
		glog.Fatalf("Failed to init proxy, error:", err)
	}
	glog.Info("Vulcan started:", handler)
	glog.Fatal(s.ListenAndServe())
}
