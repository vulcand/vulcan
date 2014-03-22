package location

import (
	"fmt"
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/loadbalance"
	"net"
	"net/http"
	"time"
)

// Location with built in failover and load balancing support
type HttpLocation struct {
	transport *http.Transport
	settings  HttpLocationSettings
}

type HttpLocationSettings struct {
	Timeouts struct {
		Read time.Duration // Socket read timeout (before we receive the first reply header)
		Dial time.Duration // Socket connect timeout
	}
	LoadBalancer LoadBalancer // Load balancing algorithm
	Limiter      Limiter      // Rate limiting algorithm
}

func NewHttpLocation(s HttpLocationSettings) (Location, error) {
	s, err := parseSettings(s)
	if err != nil {
		return nil, err
	}
	return &HttpLocation{
		transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, s.Timeouts.Dial)
			},
			ResponseHeaderTimeout: s.Timeouts.Read,
		},
		settings: s,
	}, nil
}

func (f *HttpLocation) GetLoadBalancer() LoadBalancer {
	return f.settings.LoadBalancer
}

func (f *HttpLocation) GetLimiter() Limiter {
	return f.settings.Limiter
}

func (f *HttpLocation) GetTransport() *http.Transport {
	return f.transport
}

// Standard dial and read timeouts, can be overriden when supplying location
const (
	DefaultHttpReadTimeout = time.Duration(10) * time.Second
	DefaultHttpDialTimeout = time.Duration(10) * time.Second
)

func parseSettings(s HttpLocationSettings) (HttpLocationSettings, error) {
	if s.Timeouts.Read <= time.Duration(0) {
		s.Timeouts.Read = DefaultHttpReadTimeout
	}
	if s.Timeouts.Dial <= time.Duration(0) {
		s.Timeouts.Dial = DefaultHttpDialTimeout
	}
	if s.LoadBalancer == nil {
		return s, fmt.Errorf("Provide load balancer")
	}
	return s, nil
}
