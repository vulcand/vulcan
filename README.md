Warning!
--------
**Vulcan is under heavy development and API is likely to change!**
**Beware when integrating and using it at this stage!**

[![Build Status](https://travis-ci.org/mailgun/vulcan.png)](https://travis-ci.org/mailgun/vulcan)
[![Build Status](https://drone.io/github.com/mailgun/vulcan/status.png)](https://drone.io/github.com/mailgun/vulcan/latest)

Vulcan is a HTTP proxy that you program in JavaScript:

```javascript
function handler(request){
        return {upstreams: ["http://localhost:5000"]}
}
```

It supports rate limiting and failover:

```javascript
function handler(request){
        return {
           failover: true,
           upstreams: ["http://localhost:5000"],
           rates: {"*": ["10 requests/second", "1000 KB/second"]}
         }
}
```

Service discovery:

```javascript
function handler(request){
        return {
           upstreams: discover("/upstreams"),
           rates: {"*": ["10 requests/second", "1000 KB/second"]}
         }
}
```

And many more advanced features you'd need when writing APIs, like Cache control, External authorization, Metrics and Failure detection. Read on!


Vulcan
------

Programmable HTTP reverse proxy for creating JSON-based API services with:

* Rate limiting
* Load balancing
* Early error detection, failover and alerting
* Metrics
* Dynamic service discovery

Rationale
---------

There's a room for specialized HTTP Proxy that simplifies API services on all stages of lifetime - from simple one box playground services to highly loaded HA clusters serving requests.

Quick Start
-----------

Vulcan is controlled by Javascript snippets:

```javascript
function handler(request){
        return {upstreams: ["http://localhost:5000"}
}
```






__Development setup__

Mailing list: https://groups.google.com/forum/#!forum/vulcan-proxy

__Install go__

(http://golang.org/doc/install)

__Get vulcan and install deps__
 
```bash
# set your GOPATH to something reasonable.
export GOPATH=~/projects/vulcan
cd $GOPATH
go get github.com/mailgun/vulcan

make -C ./src/github.com/mailgun/vulcan deps
cd ./src/github.com/mailgun/vulcan
```

__Run in devmode__
 
```bash 
make run
```

__Cassandra__

Cassandra-based throttling is a generally good idea, as it provides reliable distributed
counters that can be shared between multiple instances of vulcan. Vulcan provides auto garbage collection
and cleanup of the counters.

Tested on versions >= 1.2.5

Usage
-------

```bash
vulcan \
       -h=0.0.0.0\                  # interface to bind to
       -p=4000\                     # port to listen on
       -c=http://localhost:5000 \   # control server url#1
       -c=http://localhost:5001 \   # control server url#2, for redundancy
       -stderrthreshold=INFO \      # log info, from glog
       -logtostderr=true \          # log to stderror
       -logcleanup=24h \            # clean up logs every 24 hours
       -log_dir=/var/log/           # keep log files in this folder
       -pid=/var/run/vulcan.pid     # create pid file
       -lb=roundrobin \             # use round robin load balancer
       -b=cassandra \               # use cassandra for throttling
       -cscleanup=true \            # cleanup old counters
       -cscleanuptime=19:05 \       # cleanup counters 19:05 UTC every day
       -csnode=localhost  \         # cassandra node, can be multiple
       -cskeyspace=vulcan_dev       # cassandra keyspace
```

Development
-----------
To run server in development mode:

```bash
make run
```

To run tests

```bash
make test
```

To run tests with coverage:

```bash
make coverage
```

To cleanup temp folders

```bash
make clean
```

Status
------
Initial development done, loadtesting at the moment and fixing quirks. 
