[![Build Status](https://travis-ci.org/mailgun/vulcan.png)](https://travis-ci.org/mailgun/vulcan)

Vulcan
------

HTTP reverse proxy that supports authorization, rate limiting, load balancing and failover.

Rationale
---------

* Request routing and throttling should be dynamic and programmatic task.
* Proxy should take the pain out of the services failover, authentication, letting services behind it to be simple.

Request flow
------------

* Client request arrives to the Vulcan.
* Vulcan extracts request information and asks control server what to do with the request.
* Vulcan denies or throttles and routes the request according to the instructions from the control server.
* If the upstream fails, Vulcan can optionally forward the request to the next upstream.

Authorization
-------------

Vulcan sends the following request info to the control server:

* HTTP auth username and password
* URI
* protocol (SMTP/HTTP)
* method (POST/GET/DELETE/PUT)
* request length
* ip
* headers (JSON encoded dictionary)

Control server can deny the request by responding with non 200 response code. 
In this case the exact control server response will be proxied to the client.
Otherwise, control server replies with JSON understood by the proxy. See Routing section for details.

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

In case if all upstreams are busy or tokens rates are not allowing the request to proceed, Vulcan replies with json-encoded response:

```javascript
{
        "retry-seconds": 20,
        ...
}

```

Vulcan tells client when the next request can succeed, so clients can embrace this data and reschedule the request in 20 seconds. Note that this
is an estimate and does not guarantee that request will succeed, it guarantees that request would not succeed if executed before waiting given amount
of seconds. It allows not to waste resources and keep trying.

Failover
--------

* In case if control server fails, vulcan automatically queries the next available server.
* In case of upstream being slow or unresponsive, Vulcan can retry the request with the next upstream. 

This option turned on by the failover flag in the control response:


```javascript
{
        "failover": true,
        ...
}

```

* In this case Vulcan will retry the request on the next upstream selected by the load balancer. 

__Note__

Failover allows fast deployments of the underlying applications, however it requires that the request would be idempotent, i.e. can be safely retried several times. Read more about the term here: http://stackoverflow.com/questions/1077412/what-is-an-idempotent-operation

E.g. most of the GET requests are idempotent as they don't change the app state, however you should be more careful with POST requests,
as they may do some damage if repeated.

Failovers can also lead to the cascading failures. Imagine some bad request killing your service, in this case failover will kill all upstreams! That's why make sure you return limited amount of upstreams with the control response in case of failover to limit the potential damage.

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
