Vulcan
------

HTTP reverse proxy with authorization and rate limiting capabilities.

Rationale
---------

* We want to make routing and throttling to be fun and programmatic task.
* We also want proxy to take the pain out of the services failover, so services behind it can be all dumb and relaxed letting
proxy to do the heavyliting.

General request workflow
------------------------

* Request gets to the proxy
* Request parameters are extracted from the request and sent to control server as GET request. 

Authorization
-------------

Parameters extracted:

* authentication credentials i.e. username and password
* URI
* protocol (SMTP/HTTP)
* method (POST/GET/DELETE/PUT)
* request length 
* ip
* headers

Control server can deny the request, by responding with non 200 response code, in this case the response will be proxied to the client.
Otherwise, control server replies with JSON understood by the proxy, see Routing section for details

Routing & Throttling
--------------------

If the request is good to go, control server replies with something like this:

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

Download source files and install the egg:
```
go get github.com/mailgun/vulcan
```

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
GOMAXPROCS=4 vulcan -stderrthreshold=INFO \      # log info
                    -logtostderr=true \          # log to stderror
                    -c=http://localhost:5000 \   # control server urls
                    -b=cassandra \               # use cassandra for throttling
                    -lb=random \                 # use random load balancer
                    -csnode=localhost  \         # cassandra node (can be multiple)
                    -cskeyspace=vulcan_dev       # cassandra keyspace
```

Development
-----
To run server in devmode

```
make run
```

To run tests

```
make test
```

To run tests with coverage:

```
make coverage
```

To cleanup temp folders

```
make clean
```

Status
------

Undergoing development.
