package main

import (
	"github.com/mailgun/vulcan"
	"log"
	"math/rand"
	"net/http"
	"time"
	"tux21b.org/v1/gocql"
)

func main() {
	vulcan.LogMessage("Vulcan starting")
	rand.Seed(time.Now().UTC().UnixNano())
	controlServers := []string{"http://localhost:5000/auth"}
	throttlerConfig := vulcan.CassandraConfig{
		Servers:     []string{"localhost"},
		Keyspace:    "vulcan_dev",
		Consistency: gocql.One,
	}
	backend, err := vulcan.NewCassandraBackend(
		throttlerConfig,
		&vulcan.RealTime{})
	if err != nil {
		log.Fatalf("Failed to init proxy, error: %s", err)
	}

	loadBalancer := vulcan.NewRandomLoadBalancer()
	if err != nil {
		log.Fatalf("Failed to init proxy, error: %s", err)
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
		log.Fatalf("Failed to init proxy, error: %s", err)
	}
	vulcan.LogMessage("Vulcan started: %q", handler)
	log.Fatal(s.ListenAndServe())
}
