[![Build Status](https://travis-ci.org/mailgun/vulcan.png)](https://travis-ci.org/mailgun/vulcan)
[![Build Status](https://drone.io/github.com/mailgun/vulcan/status.png)](https://drone.io/github.com/mailgun/vulcan/latest)
[![Coverage Status](https://coveralls.io/repos/mailgun/vulcan/badge.png?branch=master)](https://coveralls.io/r/mailgun/vulcan?branch=master)

Status
=======

Refactoring the entire thing based on everyone's feedback: moving fast, breaking things mode.
Don't use it in production or staging or basically anywhere yet.


Reverse proxy library
----------------------

Vulcan is a low level library that provides reverse proxy functionality to golang programs

Used by: https://github.com/mailgun/vulcand

It comes with rate limiting, request routing and load balancing algorithims on board as well as extensible interfaces.
It does not provide any simplified config file format or running program and serves as a core library for other programs

Example
-----------

```go

// Create a round robin load balancer and add two upstreams
rr, _ := roundrobin.NewRoundRobin()
rr.AddEndpoint(MustParseUrl("http://localhost:5000"))
rr.AddEndpoint(MustParseUrl("http://localhost:5001"))

// Create http location with the round robin load balancer and two upstreams above
location, _ := httploc.NewLocation(rr)

// Create a proxy handler that routes all requests to the same location
proxy := vulcan.NewProxy(&route.ConstRouter{
    Location: location,
})

// Start serving requests using proxy as a handler
server := &http.Server{
		Addr:           addr,
		Handler:        proxy,
	}
server.ListenAndServe()

```

