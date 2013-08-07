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

from functools import partial

from twisted.internet import defer, threads
from twisted.python.failure import Failure
from twisted.web.http import RESPONSES
from twisted.python import log

import treq

from expiringdict import ExpiringDict

from vulcan.utils import safe_format
from vulcan.upstream import get_servers, pick_server
from vulcan.errors import CommunicationFailed, AuthorizationFailed
from vulcan import config


CACHE = ExpiringDict(max_len=100, max_age_seconds=60)


def authorize(request_params):
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
    hit = safe_format(
        "{username} {password} {protocol} {method} {uri} {data_size}",
        username=request_params["username"],
        password=request_params["password"],
        protocol=request_params['protocol'],
        method=request_params['method'],
        uri=request_params['uri'],
        data_size=request_params['length'])

    auth_result = CACHE.get(hit)
    if auth_result:
        return defer.succeed(auth_result)
    else:
        url = ("http://" + pick_server(get_servers("authorization")) +
               config["auth_path"])
        d = treq.get(url, params=request_params)
        d.addCallback(partial(_authorization_received, hit))
        d.addErrback(_errback)
        return d


def _errback(failure):
    if isinstance(failure.value, (AuthorizationFailed, CommunicationFailed)):
        return failure
    else:
        log.err(failure)
        return Failure(CommunicationFailed())


def _authorization_received(hit, response):
    if response.code >= 400 and response.code < 500:
        d = treq.content(response)
        d.addCallback(partial(_authorization_failed, hit, response.code))
        d.addErrback(partial(_failed_receive_auth_failure_reason,
                             hit, response.code))
    elif response.code >= 500:
        log.err(safe_format("Authorization for request {} failed with code {}",
                            hit, response.code))
        d = treq.content(response)
        d.addCallback(partial(_authorization_failed_with_5xx,
                              hit, response.code))
        d.addErrback(_errback)
    else:
        d = treq.json_content(response)
        d.addCallback(partial(_authorization_succeeded, hit))

    return d


def _authorization_failed_with_5xx(hit, code, reason):
    log.err(safe_format(
            "Authorization for request {} failed with code {} and reason {}",
            hit, code, reason))
    return Failure(CommunicationFailed())


def _failed_receive_auth_failure_reason(hit, code, failure):
    if isinstance(failure.value, AuthorizationFailed):
        return failure

    log.err(failure)
    failure = Failure(AuthorizationFailed(code, RESPONSES[code]))
    CACHE[hit] = failure
    return failure


def _authorization_failed(hit, code, reason):
    failure = Failure(AuthorizationFailed(code, RESPONSES[code], reason))
    CACHE[hit] = failure
    return failure


def _authorization_succeeded(hit, result):
    CACHE[hit] = result
    return result
