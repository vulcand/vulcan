[![Build Status](https://travis-ci.org/mailgun/vulcan.png)](https://travis-ci.org/mailgun/vulcan)
[![Build Status](https://drone.io/github.com/mailgun/vulcan/status.png)](https://drone.io/github.com/mailgun/vulcan/latest)
[![Coverage Status](https://coveralls.io/repos/mailgun/vulcan/badge.png?branch=master)](https://coveralls.io/r/mailgun/vulcan?branch=master)

Status
=======
Don't use it in production, early adopters and hackers are welcome


Reverse proxy library
----------------------

Vulcan is a low level library that provides reverse proxy functionality to golang programs.
It comes with rate limiting, request routing and load balancing algorithims on board as well as extensible interfaces.
It does not provide any simplified config file format or running program and serves as a core library for other programs

Example
-----------

```go

// Set load balancer and two upstreams
loadBalancer := NewRoundRobin(NewUpstreamFromString("http://localhost:5000", "http://localhost:5001"))

// Set up rate limiter with 1 request per second with bursts up to 5 requests per second per client ip
rateLimiter := NewClientIpLimiter(Rate{1, time.Second}, 5)

// Set up location with load balancer and rate limiter created above
location := &Location{LoadBalancer: loadBalancer, Limiter: rateLimiter}

// Route all requests to this location
router := &MatchAll{Location: location}

// Create proxy
proxy, err := NewReverseProxy(ProxySettings{Router: router})

// Start serving requests using proxy as a handler
server := &http.Server{
		Addr:           addr,
		Handler:        proxy,
	}
server.ListenAndServe()

```

