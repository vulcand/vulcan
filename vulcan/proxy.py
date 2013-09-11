# -*- test-case-name: vulcan.test.test_proxy -*-
"""This module implements proxy with authentication and cassandra-based
throttling. Auth is done via calling Auth HTTP service provided
to vulcan at startup.
"""
import json

from twisted.web.http import (HTTPChannel,
                              HTTPFactory as StandardHTTPFactory,
                              UNAUTHORIZED, SERVICE_UNAVAILABLE,
                              INTERNAL_SERVER_ERROR,
                              Request)

from twisted.web.proxy import (ProxyClientFactory,
                               ProxyClient)

from twisted.internet import reactor
from twisted.python import log

from vulcan import auth
from vulcan import config
from vulcan import throttling
from vulcan.errors import (RESPONSES,
                           RateLimitReached,
                           AuthorizationFailed)

from vulcan.routing import AuthRequest
from twisted.python.compat import intToBytes


class ThrottlingRequest(Request):
    """This request extends standard http request to
    support three additional phases:

    * Request is authenticated through external Auth service.
    * Request is checked against Cassandra counters to see if we
    need to throttle it
    * Request is proxied to the upstream returned by the Auth service.
    """

    reactor = reactor

    def __init__(self, *args, **kwargs):
        Request.__init__(self, *args, **kwargs)
        self._requestFinished = False
        self.notifyFinish().addBoth(self._finished)

    def process(self):
        try:
            if not self.getHeader(b'Authorization'):
                self.setHeader(
                    b'WWW-Authenticate',
                    b'basic realm="%s"' % config['auth']['realm'])
                self._writeError(UNAUTHORIZED, RESPONSES[UNAUTHORIZED])
                return

            # parse request
            self._authRequest = AuthRequest.from_http_request(self)

            # pass the request to auth server to get instructions
            d = auth.authorize(self._authRequest)
            d.addCallback(self._authGranted)
            d.addErrback(self._authFailed)
        except:
            self._writeInternalError()
            log.err(
                None, "Exception when processing: %s" % (self._authRequest,))

    def _authGranted(self, response):
        log.msg(
            format=b"Authorization granted: %(request)s %(response)s",
            request=self._authRequest,
            response=response)

        d = throttling.get_upstream(response)
        d.addCallback(self._upstreamReceived)
        d.addErrback(self._upstreamFailed)

    def _authFailed(self, e):
        if e.check(AuthorizationFailed):
            log.msg(
                format=b"Authorization failed: %(request)s %(reason)s",
                request=self._authRequest,
                reason=e.value)
            self._writeError(
                e.value.status, e.value.message, e.value.response)
        else:
            log.err(e)
            self._writeInternalError()

    def _upstreamReceived(self, upstream):
        log.msg(format=b"Got upstream: %(upstream)s", upstream=upstream)

        for header, value in upstream.headers.encoded.iteritems():
            self.requestHeaders.setRawHeaders(header, [value])

        clientFactory = ReportingProxyClientFactory(
            self.method,
            upstream.uri,
            self.clientproto,
            self.getAllHeaders(),
            self.content.read(),
            self)

        self.reactor.connectTCP(
            upstream.host, upstream.port, clientFactory)

    def _upstreamFailed(self, e):
        if e.check(RateLimitReached):
            log.msg(
                format=b"Rate limiting: %(request)s",
                request=self._authRequest)
            self._writeError(
                e.value.status,
                e.value.message,
                retry_seconds=e.value.retry_seconds)
        else:
            log.err(e)
            self._writeInternalError()

    def _writeError(self, code, message, response=None, **kwargs):
        if self._requestFinished:
            log.msg(b"WriteError: Connection was dropped, no need to reply")
            return

        self.setResponseCode(code, message)
        self.setHeader(b'Content-Type', b'application/json')
        error = {'error': response or message}
        error.update(kwargs)
        body = json.dumps(error)
        self.setHeader(b'Content-Length', intToBytes(len(body)))
        Request.write(self, body)
        self.finish()

    def _writeInternalError(self):
        self._writeError(
            INTERNAL_SERVER_ERROR,
            RESPONSES[INTERNAL_SERVER_ERROR])

    def _finished(self, ignored):
        """
        Record the end of the response generation for the request being
        serviced.
        """
        self._requestFinished = True


class ReportingProxyClientFactory(ProxyClientFactory):
    protocol = ProxyClient
    noisy = False

    def clientConnectionFailed(self, connector, reason):
        """
        Report a connection failure in a response to the incoming request as
        an error.
        """
        log.err(
            reason, b"Could not connect to %s" % (connector.getDestination(),))
        self.father._writeError(
            SERVICE_UNAVAILABLE, RESPONSES[SERVICE_UNAVAILABLE])


class HTTPFactory(StandardHTTPFactory):
    noisy = False

    def buildProtocol(self, addr):
        channel = HTTPChannel()
        channel.requestFactory = ThrottlingRequest
        return channel
