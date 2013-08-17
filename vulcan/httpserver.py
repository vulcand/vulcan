# -*- test-case-name: vulcan.test.test_httpserver -*-

import json
from collections import namedtuple
from functools import partial

import treq

from twisted.web import http
from twisted.web.http import (HTTPChannel, HTTPFactory as StandardHTTPFactory,
                              OK, UNAUTHORIZED, SERVICE_UNAVAILABLE)

from twisted.web.proxy import (ReverseProxyRequest, ProxyClientFactory,
                               ProxyClient)
from twisted.internet.defer import maybeDeferred
from twisted.internet import defer
from twisted.web.error import Error
from twisted.python.failure import Failure
from twisted.python import log

from vulcan import auth
from vulcan.upstream import pick_server
from vulcan import config
from vulcan.errors import (TOO_MANY_REQUESTS, RESPONSES, RateLimitReached,
                           AuthorizationFailed, CommunicationFailed)
from vulcan.utils import to_utf8, safe_format
from vulcan.throttling import check_and_update_rates


EndpointFactory = namedtuple('EndpointFactory', ['host', 'port'])
IP_HEADER = "X-Real-IP"


class RestrictedChannel(HTTPChannel):
    # authorization module
    auth = auth

    def allHeadersReceived(self):
        HTTPChannel.allHeadersReceived(self)
        request = self.requests[-1]

        # by now we already know request's HTTP method, version and URI
        # let's fill it in or we won't be able to finish unreceived request
        request.uri = self._path
        request.clientproto = self._version
        request.method = self._command

        if request.getHeader("Authorization"):
            d = self.auth.authorize(
                {
                    'username': request.getUser(),
                    'password': request.getPassword(),
                    'protocol': request.clientproto,
                    'method': request.method,
                    'uri': request.uri,
                    'length': request.getHeader("Content-Length") or 0
                })

            # pass request to callbacks to finish it later
            # we receive and process requests asynchronously
            # so self.requests[-1] could point to a different request
            # by the time we access it
            d.addCallback(partial(self.checkAndUpdateRates, request))
            d.addCallback(partial(self.proxyPass, request))
            d.addErrback(partial(self.errorToHTTPResponse, request))
        else:
            request.setResponseCode(UNAUTHORIZED, RESPONSES[UNAUTHORIZED])
            request.setHeader('WWW-Authenticate',
                              'basic realm="%s"' % config['realm'])
            request.write("")
            request.finishUnreceived()

    def errorToHTTPResponse(self, request, failure):
        """Converts errors into http responses.
        """
        if isinstance(failure.value, RateLimitReached):
            request.setResponseCode(TOO_MANY_REQUESTS,
                                    RESPONSES[TOO_MANY_REQUESTS])
            request.write(failure.getErrorMessage() or "")
        elif isinstance(failure.value, AuthorizationFailed):
            request.setResponseCode(failure.value.status,
                                    failure.value.message)
            request.write(failure.value.response or "")
        else:
            # unknown exception we haven't logged before
            if not isinstance(failure.value, CommunicationFailed):
                log.err(failure)

            request.setResponseCode(SERVICE_UNAVAILABLE,
                                    RESPONSES[SERVICE_UNAVAILABLE])
            request.write("")

        request.finishUnreceived()

    @defer.inlineCallbacks
    def checkAndUpdateRates(self, request, settings):
        """
        Checks if request exceeds allowed request's rate. Raises
        RateLimitReached exception if it does and updates request's rate
        otherwise. Ignores all other exceptions.
        """
        try:
            request_params = dict(
                auth_token=settings["auth_token"],
                protocol=request.clientproto,
                method=request.method,
                uri=request.uri,
                length=request.getHeader("Content-Length") or 0,
                ip=request.getHeader(IP_HEADER) or "")
            r = yield check_and_update_rates(request_params)
        except RateLimitReached:
            raise
        except Exception, e:
            log.err(e)
        defer.returnValue(settings)

    def proxyPass(self, request, settings):
        host, port = pick_server(settings['upstream']).split(":")
        port = int(port)
        # treq converts upstream string we got from server to unicode
        # host should be an encoded bytestring since we're sending it
        # over network
        host = to_utf8(host)
        request.factory = EndpointFactory(host, port)
        request.processWhenReady()


class ReportingProxyClientFactory(ProxyClientFactory):
    protocol = ProxyClient

    def clientConnectionFailed(self, connector, reason):
        """
        Report a connection failure in a response to the incoming request as
        an error.
        """
        log.err(safe_format("couldn't connect to {}: {} {}",
                            connector.getDestination(),
                            reason.getErrorMessage(),
                            reason.getTraceback()))
        self.father.setResponseCode(SERVICE_UNAVAILABLE,
                                    RESPONSES[SERVICE_UNAVAILABLE])
        self.father.write("")
        self.father.finish()


class DynamicallyRoutedRequest(ReverseProxyRequest):
    proxyClientFactoryClass = ReportingProxyClientFactory

    def process(self):
        """
        Normally we'd handle this request by connecting to the proxied server
        and forwarding it there, then forwarding the response back as
        the response to this request.

        But since we select the proxied server dynamically based on
        the authorization results which might not been received yet
        we do nothing here.
        """
        if hasattr(self, "factory"):
            self._process()
        else:
            self.received = True

    def processWhenReady(self):
        """
        Handle this request by connecting to the proxied server
        and forwarding it there, then forwarding the response back as
        the response to this request.
        """
        if hasattr(self, "received"):
            self._process()

    def _process(self):
        """
        Handle this request by connecting to the proxied server and forwarding
        it there, then forwarding the response back as the response to this
        request.

        Copy of ReverseProxyRequest's process() method except that
        it doesn't set Host header to proxied server hostname.
        """
        clientFactory = self.proxyClientFactoryClass(
            self.method, self.uri, self.clientproto, self.getAllHeaders(),
            self.content.read(), self)
        self.reactor.connectTCP(self.factory.host, self.factory.port,
                                clientFactory)

    def finishUnreceived(self):
        """Finishes request that isn't fully received.

        If request isn't fully received then
        calling request.finish() won't close the connection
        we need to call loseConnection() explicitly
        """
        self.transport.loseConnection()


class RestrictedReverseProxy(RestrictedChannel):
    requestFactory = DynamicallyRoutedRequest


class HTTPFactory(StandardHTTPFactory):
    def buildProtocol(self, addr):
        return RestrictedReverseProxy()
