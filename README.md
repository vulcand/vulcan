vulcan
======

Introduction
-------------
HTTP and SMTP reverse proxy with authorization and rate limiting capabilities

Usage
-----

Download source files and install the egg:
```
$ cd vulcan
$ python setup.py develop
```

You'll also need:

* cassandra (I used version 1.2.5)
* expiringdict (from our github repository, it could be installed automatically by vulcan's setup.py but you'll
need environment variables `MG_COLABORATOR` and `MG_COLABORATOR_PASSWORD` i.e. username/password for our github
private repo)
* thrift (I used 1.0.0-dev from github)
* telephus (I used 1.0.0-beta1 from github)
* create keyspace "Keyspace1" in cassandra (probably it should be changed to "Development" or smth) and `hits`
and `limits` databases (bellow are queries in cql3):
 * ```create table hits (hit text, ts int, counter counter, primary key (hit, ts));```
 * ```create table limits (id uuid, auth_token text, protocol text, method text, path text, data_size int, period int, threshold int, primary key (id));```
* coverage
* trialcoverage (not sure if we need this one)

Obviosly all this should be automated.

To run server:

```
$ cd vulcan
$ python vulcandaemon.py -f development.ini
```

To run tests with coverage:

```
$ coverage run `which trial` ./vulcan
$ coverage report
```


General request workflow
------------------------

Request gets to reverse proxy (see vulcandaemon.py, httpserver.py, smtpserver.py).
Request parameters are extracted from the request and sent to authorization server. Parameters to extract:

* authentication credentials i.e. username and password
* URI
* protocol (SMTP/HTTP)
* method (POST/GET/DELETE/PUT)
* length 

Authorization server responds with JSON that has authorization token which uniquely identifies the requester and
upstream string i.e. list of servers the request could be forwarded to:

```
{"auth_token": "qwerty123", "upstream": "10.241.0.25:3001,10.241.0.26:3002"}

```
For implementation details see auth.py

**NOTE:** Probably we'll need to change that. With such implementation the authorization server needs to know about all
possible services and which request should go to which service/upstream. It seems more reasonable if the reverse proxy
has an interface to register/unregister services/upstreams. Services/upstreams should be cached and upstreams
should be updated  both in cache and in the database for all reverse proxies. This way services that turn off/on
upstreams during deployment could be deployed without waiting for the change to propagate from the database to the
caches.

The request is checked against the rate limiting database (see throttling.py).
The request is proxied to the corresponding upstream and response is returned to the requester.

What if some upstream down? This information should be cached in upstream.py cache and the request shouldn't be sent
to the server. upstream.py is responsible for picking up the server from upstream to send request to.

In case of SMTP server the request is transformed into HTTP request and proxied to the application that sends it out.

Main modules and their status
-----------------------------

*module*          | *purpose*              | *status*                                                                     
------------------|------------------------|------------------------------------------------------------------------------
auth.py           | authorize requests     | implemented, tested manually, covered with tests, documented                 
throttling.py     | rate limit requests    | implemented, tested manually                                                 
httpserver.py     | http reverse proxy     | implemented, tested manually, partially documented                           
smtpserver.py     | smtp reverse proxy     | very raw implementation, partially documented                                
vulcandaemon.py   | starts reverse proxy   | implemented, tested manually, probably no automated tests needed, documented 

Caveats
-------

When connection to cassandra database is down looks like the exception misses errback somehow and the request can't
be completed:

```
$ sudo service cassandra stop
xss =  -ea -javaagent:/usr/share/cassandra/lib/jamm-0.2.5.jar -XX:+UseThreadPriorities -XX:ThreadPriorityPolicy=42 -Xms1927M -Xmx1927M -Xmn400M -XX:+HeapDumpOnOutOfMemoryError -Xss180k
$ curl -v --user api:apikey http://localhost:8080/v2/definbox.com/log
* About to connect() to localhost port 8080 (#0)
*   Trying 127.0.0.1...
* connected
* Connected to localhost (127.0.0.1) port 8080 (#0)
* Server auth using Basic with user 'api'
> GET /v2/definbox.com/log HTTP/1.1
> Authorization: Basic YXBpOmFwaWtleQ==
> User-Agent: curl/7.27.0
> Host: localhost:8080
> Accept: */*
> 
* additional stuff not fine transfer.c:1037: 0 0
* additional stuff not fine transfer.c:1037: 0 0
* additional stuff not fine transfer.c:1037: 0 0
* additional stuff not fine transfer.c:1037: 0 0
* additional stuff not fine transfer.c:1037: 0 0
* additional stuff not fine transfer.c:1037: 0 0
* additional stuff not fine transfer.c:1037: 0 0
^C
$
```
Meanwhile on the server side:
```
$ python vulcandaemon.py -f development.ini
Thrift pool connection to <CassandraNode localhost:9160 @0x3894cf8> failed
Traceback (most recent call last):
Failure: twisted.internet.error.ConnectionRefusedError: Connection was refused by other side: 111: Connection refused.
Thrift pool connection to <CassandraNode 127.0.0.1:9160 @0x3b34248> failed
Traceback (most recent call last):
Failure: twisted.internet.error.ConnectionRefusedError: Connection was refused by other side: 111: Connection refused.
Thrift pool connection to <CassandraNode localhost:9160 @0x3894cf8> failed
Traceback (most recent call last):
Failure: twisted.internet.error.ConnectionRefusedError: Connection was refused by other side: 111: Connection refused.
Thrift pool connection to <CassandraNode 127.0.0.1:9160 @0x3b34248> failed
Traceback (most recent call last):
Failure: twisted.internet.error.ConnectionRefusedError: Connection was refused by other side: 111: Connection refused.
```
