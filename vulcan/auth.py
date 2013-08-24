# -*- test-case-name: vulcan.test.test_auth -*-

"""
Authorization module for reverse proxy.

>>> from vulcan auth import authorize
>>>
>>> request_params = {
...     "username": "user",
...     "password": "secret",
...     "protocol": "http",
...     "method": "get",
...     "uri": "http://localhost/test",
...     "length": 0
...     }
>>> d = authorize(request_params)
"""
import json

from twisted.internet import defer

import random
import treq

from vulcan import config
from vulcan.routing import AuthResponse
from vulcan.errors import AuthorizationFailed


@defer.inlineCallbacks
def authorize(request):
    """Authorize request based on the parameters extracted from the request.

    >>> request_params = {
    ...     "username": "user",
    ...     "password": "secret",
    ...     "protocol": "http",
    ...     "method": "get",
    ...     "uri": "http://localhost/test",
    ...     "length": 0
    ...     }
    >>> d = authorize(request_params)

    Authorization server should return JSON with authorization token and
    a string of upstream servers:

    '{"auth_token": "abc", "upstream": "10.241.0.25:3000,10.241.0.25:3001"}'

    The token is a string uniquely identifying the requester and
    the upstream represents servers the request could be proxied to.
    Successful response i.e. dictionary decoded from the JSON string is cached.

    If authorization server response isn't a valid JSON string a Failure
    instance with CommunicationFailed exception is returned. The error is
    logged.

    NOTE: no checks are made to make sure that the response has the
    required fields or that the fields have a valid format.

    Authorization server should return an error response if the request
    couldn't be authorized, optionaly providing the reason e.g. as JSON:

    '{"message": "Wrong API key"}'

    If authorization server responds with 4xx code a Failure instance with
    AuthorizationFailed exception is returned. The failure is cached.
    The failure isn't logged.

    If authorization server responds with 5xx code a Failure instance with
    CommunicationFailed exception is returned. The failure isn't cached by
    auth module. The response is logged.

    If some unexpected error happens during request processing (e.g. if
    authorization server is unavailable) a Failure instance with
    CommunicationFailed exception is returned. The error is logged.
    """

    url = random.choice(config["auth"]["urls"])
    r = yield treq.get(url, params=request.to_json())
    content = yield treq.content(r)

    if r.code >= 300 or r.code < 200:
        raise AuthorizationFailed(r.code, r.phrase, content)

    defer.returnValue(AuthResponse.from_json(json.loads(content)))

