vulcan
------

HTTP and SMTP reverse proxy with authorization and rate limiting capabilities

Installation
------------

Download source files and install the egg:
```
$ cd vulcan
$ python setup.py develop
```

You'll also need:

* cassandra (Tested on versions >= 1.2.5)
* create keyspaces "dev" and "test" in cassandra, with the following tables:

```sql
USE dev;

CREATE TABLE hits (
      hit text PRIMARY KEY,
      counter counter
    ) WITH COMPACT STORAGE;
```

Usage
-----
To run server in devmode

```
make rundev
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

General request workflow
------------------------

Request gets to reverse proxy (see vulcandaemon.py, httpserver.py, smtpserver.py).
Request parameters are extracted from the request and sent to authorization server. Parameters to extract:

* authentication credentials i.e. username and password
* URI
* protocol (SMTP/HTTP)
* method (POST/GET/DELETE/PUT)
* request length 
* ip

Authorization server responds with JSON that has authorization tokens that tellsvulcan how it should throttle and upstreams:

```
        tokens=[
            {
                'id': 'hello',
                'rates': [
                    {'value': 10, 'period': 'minute'}
                ]
            }
       ],
       upstreams=[
            {
                'url': 'http://localhost:5000/upstream',
                'rates': [
                    {'value': 2, 'period': 'minute'}
                 ]
            },
            {
                'url': 'http://localhost:5000/upstream2',
                'rates': [
                    {'value': 4, 'period': 'minute'}
                 ]
            }
       ])

```

* In this example all requests regardless of parameters and auth will be throttled by the same token hello, with maximum 10 hits per minute total.
* The request can be routed to one of the two upstreams, the first upstream allows max 2 requests per minute, the second one allows 4 requests per minute.

Auth server example
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
                    {'value': 10, 'period': 'minute'}
                ]
            }
       ],
       upstreams=[
            {
                'url': 'http://localhost:5000/upstream',
                'rates': [
                    {'value': 2, 'period': 'minute'}
                 ]
            },
            {
                'url': 'http://localhost:5000/upstream2',
                'rates': [
                    {'value': 4, 'period': 'minute'}
                 ]
            }
       ])

@app.route('/upstream')
def upstream():
    print request.args
    return 'Upstream: Hello World!'

@app.route('/upstream2')
def upstream2():
    print request.args
    return 'Upstream2: Hello World!'

if __name__ == '__main__':
    app.run()
```

Status
-----------------------------

Undergoing development. 100% test coverage. Authorization and rate-limiting for HTTP are implemented and under active
testing.
