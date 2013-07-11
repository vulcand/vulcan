from functools import partial

from twisted.internet import defer, threads
from twisted.python.failure import Failure
from twisted.web.http import RESPONSES

import treq

from expiringdict import ExpiringDict

from vulcan.utils import safe_format
from vulcan.upstream import get_servers, pick_server
from vulcan.errors import CommunicationFailed, AuthorizationFailed
from vulcan import config, log


CACHE = ExpiringDict(max_len=100, max_age_seconds=60)


def authorize(request_params):
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
    if isinstance(failure.value, AuthorizationFailed):
        return failure
    else:
        log.err(failure)
        return Failure(CommunicationFailed())


def _authorization_received(hit, response):
    if response.code >= 400:
        d = treq.content(response)
        d.addCallback(partial(_authorization_failed, hit, response.code))
        d.addErrback(partial(_failed_receive_auth_failure_reason,
                             hit, response.code))
    else:
        d = treq.json_content(response)
        d.addCallback(partial(_authorization_succeeded, hit))

    return d


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
