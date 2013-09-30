[![Build Status](https://travis-ci.org/mailgun/vulcan.png)](https://travis-ci.org/mailgun/vulcan)

Vulcan
------

HTTP reverse proxy with authorization and rate limiting capabilities.

Rationale
---------

* Request routing and throttling should be dynamic and programmatic task.
* Proxy to take the pain out of the services failover, authentication, so services behind it can be all dumb and relaxed letting
proxy to do the heavyliting.

Request flow
------------

* Request gets to the proxy
* Request parameters are extracted from the request and sent to control server as GET request. 
* Proxy analyzes parameters, throttles the request
* If the request is good to go, forwarded to the upstream selected by the load balancer
* If the upstream fails, vulcan can optionally replay the request to the next upstream,
depending on the instructions.

Authorization
-------------

Parameters extracted:

* authentication credentials i.e. username and password
* URI
* protocol (SMTP/HTTP)
* method (POST/GET/DELETE/PUT)
* request length 
* ip
* headers (JSON encoded dictionary)

Control server can deny the request, by responding with non 200 response code, in this case the response will be proxied to the client.
Otherwise, control server replies with JSON understood by the proxy, see Routing section for details

Routing & Throttling
--------------------

If the request is good to go, control server replies with json in the following format:

```javascript
{
        "tokens": [
            {
                "id": "hello",
                "rates": [
                    {"increment": 1, "value": 10, "period": "minute"}
                ]
            }
       ],
       "upstreams": [
            {
                "url": "http://localhost:5000/upstream",
                'rates': [
                    {"increment": 1, "value": 2, "period": "minute"}
                 ]
            },
            {
                "url": "http://localhost:5000/upstream2",
                "rates": [
                    {"increment": 1, "value": 4, "period": "minute"}
                 ]
            }
       ])
}

```

* In this example all requests will be throttled by the same token 'hello', with maximum 10 hits per minute total.
* The request can be routed to one of the two upstreams, the first upstream allows max 2 requests per minute, the second one allows 4 requests per minute.


Failover
--------

* In case if control server fails, vulcan automatically retries the request on the next one
* Vulcan can optionally replay the request to the next upstream, this option turned on by the failover flag in the
control response:


```javascript
{
        "failover": true,
        ...
}

```

* In this case Vulcan will replay request to the next server chosen by the load balancer. As such behavior may lead to the cascading failures,
make sure you return limited amount of upstreams in the request.


Control server example
-------------------

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/auth')
def auth():
    print request.args
    return jsonify(
        tokens=[
            {
                'id': 'hello',
                'rates': [
                    {'increment': 1, 'value': 10, 'period': 'minute'}
                ]
            }
       ],
       upstreams=[
            {
                'url': 'http://localhost:5000/upstream',
                'rates': [
                    {'increment': 1, 'value': 2, 'period': 'minute'}
                 ]
            },
            {
                'url': 'http://localhost:5000/upstream2',
                'rates': [
                    {'increment': 1, 'value': 4, 'period': 'minute'}
                 ]
            }
       ])

@app.route('/upstream')
def upstream():
    return 'Upstream: Hello World!'

@app.route('/upstream2')
def upstream2():
    return 'Upstream2: Hello World!'

if __name__ == '__main__':
    app.run()
```

Installation
------------

__Install go__

(http://golang.org/doc/install)

__Get vulcan and install deps__
 
```bash 
go get github.com/mailgun/vulcan

go get -v github.com/axw/gocov # go test coverage
go install github.com/axw/gocov/gocov # go test coverage 
go get -v github.com/golang/glog # go logging system
go get -v launchpad.net/gocheck # go advanced testing framework
go get -v github.com/mailgun/gocql # go cassandra client
```

__Run in devmode__
 
```bash 
make run
```

__Cassandra__

If you want to use cassandra for throttling (which is a good idea), you'll need:

* cassandra (Tested on versions >= 1.2.5)
* create keyspace with the following table

```sql
CREATE TABLE hits (
      hit text PRIMARY KEY,
      value counter
    ) WITH COMPACT STORAGE;
```

Usage
-------
```bash
vulcan -stderrthreshold=INFO \      # log info, from glog
       -logtostderr=true \          # log to stderror
       -c=http://localhost:5000 \   # control server url#1
       -c=http://localhost:5001 \   # control server url#2, for redundancy
       -b=cassandra \               # use cassandra for throttling
       -lb=random \                 # use random load balancer
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
