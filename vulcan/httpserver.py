# -*- test-case-name: vulcan.test.test_httpserver -*-

from urlparse import urlparse

from twisted.web.http import (HTTPChannel, HTTPFactory as StandardHTTPFactory,
                              UNAUTHORIZED, SERVICE_UNAVAILABLE)
from twisted.web.proxy import (ReverseProxyRequest, ProxyClientFactory,
                               ProxyClient)
from twisted.internet import defer
from twisted.python import log

from vulcan import auth
from vulcan import config
from vulcan import throttling
from vulcan.errors import (TOO_MANY_REQUESTS,
                           RESPONSES,
                           RateLimitReached,
                           AuthorizationFailed)

from vulcan.utils import safe_format
from vulcan.routing import AuthRequest


RETRY_IN_SECONDS = "X-Retry-In-Seconds"


class RestrictedChannel(HTTPChannel):
    # authorization module
    auth = auth

    @defer.inlineCallbacks
    def allHeadersReceived(self):
        HTTPChannel.allHeadersReceived(self)
        request = self.requests[-1]

        # by now we already know request's HTTP method, version and URI
        # let's fill it in or we won't be able to finish unreceived request
        request.uri = self._path
        request.clientproto = self._version
        request.method = self._command

        if not request.getHeader("Authorization"):
            request.setResponseCode(
                UNAUTHORIZED, RESPONSES[UNAUTHORIZED])
            request.setHeader(
                'WWW-Authenticate',
                'basic realm="%s"' % config['auth']['realm'])

            request.write("")
            request.finishUnreceived()
            return

        try:
            _request = AuthRequest.from_http_request(request)
            r = yield self.auth.authorize(_request)

            upstream = yield throttling.get_upstream(r)

            request.factory = upstream
            request.processWhenReady()

        except AuthorizationFailed, e:
            log.msg("Authorization failed: %s" % (_request,))
            request.setResponseCode(e.status, e.message)
            request.write(e.response or "")
            request.finishUnreceived()

        except RateLimitReached, e:
            log.msg("Rate limiting: %s" % (_request,))
            request.setResponseCode(
                TOO_MANY_REQUESTS,
                RESPONSES[TOO_MANY_REQUESTS])
            request.setHeader(RETRY_IN_SECONDS, str(e.retry_seconds))
            request.write(str(e))
            request.finishUnreceived()

        except Exception:
            log.err("Exception when processing request: %s" % (_request,))
            log.err()
            request.setResponseCode(
                SERVICE_UNAVAILABLE,
                RESPONSES[SERVICE_UNAVAILABLE])
            request.write("")
            request.finishUnreceived()


class ReportingProxyClientFactory(ProxyClientFactory):
    protocol = ProxyClient
    noisy = False

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
            self.method, self.factory.uri, self.clientproto,
            self.getAllHeaders(), self.content.read(), self)

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
    noisy = False

    def buildProtocol(self, addr):
        return RestrictedReverseProxy()
