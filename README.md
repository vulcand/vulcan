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
* create keyspace "Keyspace1" in cassandra (probably it should be changed to "Development" or smth) and `hits`
and `limits` databases (bellow are queries in cql3):
 * ```create table hits (hit text, ts int, counter counter, primary key (hit, ts));```
 * ```create table limits (id uuid, auth_token text, protocol text, method text, path text, data_size int, period int, threshold int, primary key (id));```

To run server:

```
$ cd vulcan
$ python vulcandaemon.py -f development.ini
```

To run tests with coverage:

```
$ cd vulcan
$ coverage run --sosurce=vulcan `which trial` vulcan
$ coverage report --show-missing
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

Main modules and their status
-----------------------------

All modules implemented, tested manually, 100% covered with tests and documented (at least partially):

```
$ coverage report --show-missing
Name                          Stmts   Miss  Cover   Missing
-----------------------------------------------------------
vulcan/__init__                   3      0   100%   
vulcan/auth                      58      0   100%   
vulcan/errors                    18      0   100%   
vulcan/httpserver                83      0   100%   
vulcan/logging                   23      0   100%   
vulcan/test/__init__             12      0   100%   
vulcan/test/test_auth           113      0   100%   
vulcan/test/test_httpserver     131      0   100%   
vulcan/test/test_logging         61      0   100%   
vulcan/test/test_throttling     103      0   100%   
vulcan/test/test_timeout         47      0   100%   
vulcan/test/test_upstream         9      0   100%   
vulcan/test/test_utils           24      0   100%   
vulcan/throttling                99      0   100%   
vulcan/timeout                   23      0   100%   
vulcan/upstream                  10      0   100%   
vulcan/utils                     32      0   100%   
-----------------------------------------------------------
TOTAL                           849      0   100%   
```
