# -*- test-case-name: vulcan.test.test_auth -*-

"""
Authorization module for reverse proxy.
"""

import json
from StringIO import StringIO
from contextlib import closing

from twisted.internet import defer
from twisted.python import log
from twisted.internet.protocol import Protocol
from twisted.web.client import ResponseDone

import random
import treq

from vulcan import config
from vulcan.routing import AuthResponse
from vulcan.errors import AuthorizationFailed


def authorize(request):
    """Calls auth server to authorize the given request.
    Chooses the auth server randomly.
    """

    url = random.choice(config["auth"]["urls"])
    log.msg(
        format=b"Auth request: url='%(url)s', params=%(params)s",
        url=url,
        params=request.to_json())
    d = treq.get(
        url,
        params=request.to_json(),
        timeout=config["auth"]["timeout_seconds"])
    finished = defer.Deferred()
    collector = _ResponseCollector(finished)
    d.addCallback(collector.responseReceived)
    d.addErrback(collector.responseFailed)
    return finished


class _ResponseCollector(Protocol):
    """Collects the response and the response body.
    Converts the body to json.
    """

    def __init__(self, finished):
        self.finished = finished
        self.stream = StringIO()

    def responseReceived(self, response):
        self.response = response
        self.response.deliverBody(self)

    def responseFailed(self, failure):
        log.err(failure)
        self.finished.errback(failure)

    def dataReceived(self, data):
        self.stream.write(data)

    def connectionLost(self, reason):
        with closing(self.stream) as s:
            if not reason.check(ResponseDone):
                self.finished.errback(reason)
                return

            content = s.getvalue()
            r = self.response
            if r.code >= 300 or r.code < 200:
                self.finished.errback(
                    AuthorizationFailed(r.code, r.phrase, content))
                return

            try:
                data = json.loads(content)
            except:
                self.finished.errback(
                    ValueError("Expected json, got: %s" % (content,)))
                return

            try:
                authResponse = AuthResponse.from_json(data)
            except:
                self.finished.errback()
                return

            self.finished.callback(authResponse)
