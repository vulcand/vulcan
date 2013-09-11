# coding: utf-8

from mock import patch, Mock

import json

from twisted.trial.unittest import TestCase

from twisted.internet import defer
from twisted.internet.error import ConnectionRefusedError
from twisted.web.http import (SERVICE_UNAVAILABLE, UNAUTHORIZED,
                              INTERNAL_SERVER_ERROR)
from twisted.test import proto_helpers
from twisted.web.http_headers import Headers
from twisted.web._newclient import Request, HTTPClientParser

from vulcan.errors import (AuthorizationFailed, RateLimitReached, RESPONSES,
                           TOO_MANY_REQUESTS)
from vulcan.proxy import HTTPFactory, ThrottlingRequest
from vulcan.utils import to_utf8
from vulcan import throttling
from vulcan import auth
from vulcan.routing import AuthResponse


_auth_response = AuthResponse.from_json(
    {"tokens": [{"id": "abc",
                 "rates": [{"value": 400, "period": "minute"}]
                 }],
     "upstreams": [{"url": "http://127.0.0.1:5000/path?key=val",
                    "rates": [{"value": 1800, "period": "hour"}],
                    }]})


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
        }}]})


class HTTPServerTest(TestCase):

    def setUp(self):
        self.memoryReactor = proto_helpers.MemoryReactor()
        ThrottlingRequest.reactor = self.memoryReactor
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
        protocol = HTTPClientParser(request, lambda rest: None)

        protocol.makeConnection(proto_helpers.StringTransport())
        protocol.dataReceived(value)

        collect = proto_helpers.AccumulatingProtocol()
        protocol.response.deliverBody(collect)
        return protocol.response, collect.data

    def test_noAuthHeader(self):
        """Make sure that non authenticated requests are rejected
        right away
        """
        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("\r\n")

        response, body = self.parseResponse(self.transport.value())
        self.assertEquals(401, response.code)
        self.assertEquals('Unauthorized', response.phrase)

        # this means that we have not actually connected to the upstream
        # or auth server
        self.assertEqual(len(self.memoryReactor.tcpClients), 0)

    @patch.object(auth, 'authorize')
    def test_wrongCredentials(self, authorize):
        """Reject the request if auth server rejected the request with non
        200 OK response.
        """
        data = "Wrong API key"
        e = AuthorizationFailed(UNAUTHORIZED, RESPONSES[UNAUTHORIZED], data)
        authorize.return_value = defer.fail(e)

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        response, body = self.parseResponse(self.transport.value())

        self.assertEquals(UNAUTHORIZED, response.code)
        self.assertEquals(RESPONSES[UNAUTHORIZED], response.phrase)
        self.assertEquals(json.loads(body), {'error': data})

        self.assertEqual(len(self.memoryReactor.tcpClients), 0)

    @patch.object(throttling, 'get_upstream')
    @patch.object(auth, 'authorize')
    def test_success(self, authorize, get_upstream):
        """Request is authorized, not throttled and passed to the upstream
        """
        authorize.return_value = defer.succeed(_auth_response)
        get_upstream.return_value = defer.succeed(_auth_response.upstreams[0])

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(len(self.memoryReactor.tcpClients), 1)

        client = self.memoryReactor.tcpClients[0]
        self.assertEquals(client[0], '127.0.0.1')
        self.assertEquals(client[1], 5000)
        self.assertEquals(client[2].rest, '/path?key=val')

    @patch.object(throttling, 'get_upstream')
    @patch.object(auth, 'authorize')
    def test_successWithHeaders(self, authorize, get_upstream):
        """Auth server adds headers to the successful request that is proxied
        to the upstream.
        """
        authorize.return_value = defer.succeed(_auth_response_with_headers)
        get_upstream.return_value = defer.succeed(
            _auth_response_with_headers.upstreams[0])

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(len(self.memoryReactor.tcpClients), 1)

        client = self.memoryReactor.tcpClients[0]
        self.assertEquals(client[0], '127.0.0.1')
        self.assertEquals(client[1], 5000)

        factory = client[2]
        self.assertEquals(factory.rest, '/path?key=val')
        self.assertEquals({
            'authorization': 'Basic YXBpOmFwaWtleQ==',
            'x-my-header': to_utf8(u'Юникод')}, factory.headers)

    @patch.object(throttling, 'get_upstream')
    @patch.object(auth, 'authorize')
    def test_clientConnectionFailed(self, authorize, get_upstream):
        """Make sure we returned the proper code when we'v failed to
        connect to the upstream.
        """
        authorize.return_value = defer.succeed(_auth_response)
        get_upstream.return_value = defer.succeed(_auth_response.upstreams[0])

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(len(self.memoryReactor.tcpClients), 1)
        factory = self.memoryReactor.tcpClients[0][2]
        factory.clientConnectionFailed(Mock(), Mock())

        response, body = self.parseResponse(self.transport.value())
        self.assertEquals(SERVICE_UNAVAILABLE, response.code)
        self.assertEquals(
            RESPONSES[SERVICE_UNAVAILABLE], response.phrase)
        self.flushLoggedErrors(ConnectionRefusedError)

    @patch.object(throttling, 'get_upstream')
    @patch.object(auth, 'authorize')
    def test_unexpectedException(self, authorize, get_upstream):
        """In this case we should return proper internal server error
        to signal the trouble with vulcan.
        """
        authorize.side_effect = lambda *args: defer.fail(RuntimeError("Fail"))

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(len(self.memoryReactor.tcpClients), 0)

        response, body = self.parseResponse(self.transport.value())
        self.assertEquals(INTERNAL_SERVER_ERROR, response.code)
        self.assertEquals(
            RESPONSES[INTERNAL_SERVER_ERROR], response.phrase)
        self.flushLoggedErrors(RuntimeError)

    @patch.object(throttling, 'get_upstream')
    @patch.object(auth, 'authorize')
    def test_rateLimitReached(self, authorize, get_upstream):
        """Cassandra says that this client is asking vulcan too
        frequently. Throttle the upstream.
        """
        authorize.return_value = defer.succeed(_auth_response)

        get_upstream.side_effect = lambda *args: defer.fail(
            RateLimitReached(retry_seconds=10))

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        self.assertEquals(len(self.memoryReactor.tcpClients), 0)

        response, body = self.parseResponse(self.transport.value())
        self.assertEquals(TOO_MANY_REQUESTS, response.code)
        self.assertEquals(
            'Rate limit reached. Retry in 10 seconds', response.phrase)
        self.assertEquals({
            'error': 'Rate limit reached. Retry in 10 seconds',
            'retry_seconds': 10}, json.loads(body))
