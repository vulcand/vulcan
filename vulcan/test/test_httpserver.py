# coding: utf-8

from . import *

import json
from StringIO import StringIO

from twisted.trial.unittest import TestCase

from twisted.internet import defer, task
from twisted.web.http import SERVICE_UNAVAILABLE, UNAUTHORIZED
from twisted.web.proxy import reactor
from twisted.test import proto_helpers
from twisted.python import log
from twisted.web.http_headers import Headers
from twisted.web._newclient import Request, HTTPClientParser

from vulcan.errors import (AuthorizationFailed, RateLimitReached, RESPONSES,
                           TOO_MANY_REQUESTS)
from vulcan.httpserver import (HTTPFactory, DynamicallyRoutedRequest)
from vulcan.utils import to_utf8
from vulcan import httpserver
from vulcan import httpserver as hs
from vulcan import throttling
from vulcan.routing import AuthResponse, Upstream



class HTTPServerTest(TestCase):
    def setUp(self):
        factory = HTTPFactory()
        self.protocol = factory.buildProtocol(('127.0.0.1', 0))
        self.transport = proto_helpers.StringTransport()
        self.protocol.makeConnection(self.transport)


    def parseResponse(self, value):
        """Utility function that parses responses using
        Twisted HTTPClientParser.
        """
        _boringHeaders = Headers({'host': ['example.com']})
        request = Request('GET', '/', _boringHeaders, None)
        protocol = HTTPClientParser(request, None)

        protocol.makeConnection(proto_helpers.StringTransport())
        protocol.dataReceived(value)

        collect = proto_helpers.AccumulatingProtocol()
        protocol.response.deliverBody(collect)
        return protocol.response, collect.data


    @patch.object(DynamicallyRoutedRequest, 'processWhenReady')
    def test_noAuthHeader(self, processWhenReady):
        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("\r\n")

        response, body = self.parseResponse(self.transport.value())
        self.assertEquals(401, response.code)
        self.assertEquals('Unauthorized', response.phrase)

        self.assertEquals(0, processWhenReady.call_count)


    @patch.object(httpserver.auth, 'authorize')
    @patch.object(DynamicallyRoutedRequest, 'processWhenReady')
    @patch.object(log, 'msg')
    def test_wrongCredentials(self, log_msg, processWhenReady, authorize):
        data = "Wrong API key"
        e = AuthorizationFailed(UNAUTHORIZED, RESPONSES[UNAUTHORIZED], data)
        authorize.return_value = defer.fail(e)

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(1, log_msg.call_count)
        response, body = self.parseResponse(self.transport.value())

        self.assertEquals(UNAUTHORIZED, response.code)
        self.assertEquals(RESPONSES[UNAUTHORIZED], response.phrase)
        self.assertEquals(json.loads(body), {'error': data})

        self.assertEquals(0, processWhenReady.call_count)

    @patch.object(reactor, 'connectTCP')
    @patch.object(throttling, 'get_upstream')
    @patch.object(httpserver.auth, 'authorize')
    def test_success(self, authorize, get_upstream, connectTCP):
        authorize.return_value = defer.succeed(_auth_response)
        get_upstream.return_value = defer.succeed(_auth_response.upstreams[0])

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(_auth_response.upstreams[0].host,
                          connectTCP.call_args[0][0])
        self.assertIsInstance(connectTCP.call_args[0][0], str,
                              "Host should be an encoded bytestring")
        self.assertEquals(_auth_response.upstreams[0].port,
                          connectTCP.call_args[0][1])
        # for GET requests with params auth server should return
        # upstream(s) URL(s) with possibly rewritten network location, path
        # and query string, the query string should be passed on
        # to the proxied server
        self.assertEquals("/path?key=val", connectTCP.call_args[0][2].rest)


    @patch.object(reactor, 'connectTCP')
    @patch.object(throttling, 'get_upstream')
    @patch.object(httpserver.auth, 'authorize')
    def test_successWithHeaders(self, authorize, get_upstream, connectTCP):
        """Makes sure headers from upstream are set when request
        is successfully proxied.
        """
        authorize.return_value = defer.succeed(_auth_response_with_headers)
        get_upstream.return_value = defer.succeed(
            _auth_response_with_headers.upstreams[0])

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(_auth_response.upstreams[0].host,
                          connectTCP.call_args[0][0])
        self.assertIsInstance(connectTCP.call_args[0][0], str,
                              "Host should be an encoded bytestring")
        self.assertEquals(_auth_response.upstreams[0].port,
                          connectTCP.call_args[0][1])
        # for GET requests with params auth server should return
        # upstream(s) URL(s) with possibly rewritten network location, path
        # and query string, the query string should be passed on
        # to the proxied server
        factory = connectTCP.call_args[0][2]
        self.assertEquals("/path?key=val", factory.rest)
        self.assertEquals({
                'authorization': 'Basic YXBpOmFwaWtleQ==',
                'x-my-header': to_utf8(u'Юникод')}, factory.headers)


    @patch.object(reactor, 'connectTCP')
    @patch.object(throttling, 'get_upstream')
    def test_requestReceivedBeforeChecks(self,
                                            get_upstream,
                                            connectTCP):
        self.clock = task.Clock()

        def delayed_auth(*args, **kwargs):
            d = defer.Deferred()
            self.clock.callLater(2, d.callback, _auth_response)
            return d

        with patch.object(httpserver.auth, 'authorize', delayed_auth):
            get_upstream.return_value = defer.succeed(
                _auth_response.upstreams[0])

            self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
            self.protocol.dataReceived(
                "Authorization: Basic YXBpOmFwaWtleQ==\r\n")
            self.protocol.dataReceived("\r\n")

            self.clock.advance(5)

            self.assertEquals(_auth_response.upstreams[0].host,
                              connectTCP.call_args[0][0])
            self.assertIsInstance(connectTCP.call_args[0][0], str,
                                  "Host should be an encoded bytestring")
            self.assertEquals(_auth_response.upstreams[0].port,
                              connectTCP.call_args[0][1])


    @patch.object(throttling, 'get_upstream')
    @patch.object(httpserver.auth, 'authorize')
    def test_clientConnectionFailed(self, authorize, get_upstream):
        authorize.return_value = defer.succeed(_auth_response)

        get_upstream.return_value = defer.succeed(_bad_upstream)

        # had to overwrite factories/protocols for testing purposes
        # mocks set before reactor.callLater() won't work afterwords
        class ReportingProxyClientFactory(hs.ReportingProxyClientFactory):
            def clientConnectionFailed(self_, connector, reason):
                with patch.object(log, 'err') as log_err:
                    # save log.err mock to run checks on it later
                    self_.father.log_err = log_err
                    # code after the next line will never be run
                    hs.ReportingProxyClientFactory.clientConnectionFailed(
                        self_, connector, reason)

        class DynamicallyRoutedRequest(hs.DynamicallyRoutedRequest):
            proxyClientFactoryClass = ReportingProxyClientFactory

            def finish(self_, *args, **kwargs):
                # finally our assertions

                # we log that vulcan couldn't connect to the proxied server
                # e.g. if vulcan tries to connect on the wrong port
                self.assertTrue(self_.log_err.called)
                response, body = self.parseResponse(self.transport.value())
                self.assertEquals(SERVICE_UNAVAILABLE, response.code)
                self.assertEquals(
                    RESPONSES[SERVICE_UNAVAILABLE], response.phrase)

                hs.DynamicallyRoutedRequest.finish(self_, *args, **kwargs)

        class RestrictedReverseProxy(hs.RestrictedChannel):
            requestFactory = DynamicallyRoutedRequest

        class HTTPFactory(hs.HTTPFactory):
            def buildProtocol(self_, addr):
                return RestrictedReverseProxy()

        factory = HTTPFactory()
        self.protocol = factory.buildProtocol(('127.0.0.1', 0))
        self.transport = proto_helpers.StringTransport()
        self.protocol.makeConnection(self.transport)

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived(
            "Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")


    @patch.object(reactor, 'connectTCP')
    @patch.object(httpserver.auth, 'authorize')
    @patch.object(log, 'err')
    def test_exception_when_processing(self, log_err, authorize,
                                       connectTCP):
        e = Exception("Bam!")
        authorize.side_effect = lambda *args: defer.fail(e)

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertTrue(log_err.called)

    @patch.object(DynamicallyRoutedRequest, 'processWhenReady')
    @patch.object(throttling, 'get_upstream')
    @patch.object(httpserver.auth, 'authorize')
    def test_rate_limit_reached(self, authorize,
                                get_upstream, processWhenReady):
        authorize.return_value = defer.succeed(_auth_response)

        get_upstream.side_effect = lambda *args: defer.fail(
            RateLimitReached(retry_seconds=10))

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        response, body = self.parseResponse(self.transport.value())
        self.assertEquals(TOO_MANY_REQUESTS, response.code)
        self.assertEquals('Rate limit reached. Retry in 10 seconds', response.phrase)
        self.assertEquals(
            {
                'error': 'Rate limit reached. Retry in 10 seconds',
                'retry-seconds': 10}, json.loads(body))

        self.assertEquals(0, processWhenReady.call_count)


_auth_response = AuthResponse.from_json(
    {"tokens": [{"id": "abc",
                 "rates": [{"value": 400, "period": "minute"}]
                 }],
     "upstreams": [{"url": "http://127.0.0.1:5000/path?key=val",
                    "rates": [{"value": 1800, "period": "hour"}],
                    }],
     "headers": {"X-Real-Ip": "1.2.3.4"}})


_auth_response_with_headers = AuthResponse.from_json({
    "tokens": [{
        "id": "abc",
        "rates": [{"value": 400, "period": "minute"}]
    }],
    "upstreams": [{
         "url": "http://127.0.0.1:5000/path?key=val",
         "rates": [{"value": 1800, "period": "hour"}],
         "headers": {
             "X-My-Header": u'Юникод'
          }
    }],
    "headers": {
        "X-Real-Ip": "1.2.3.4"}})


_bad_upstream = Upstream.from_json(
    {"url": "http://127.0.0.1:69",
     "rates": [{"value": 1800, "period": "hour"}]
     })
