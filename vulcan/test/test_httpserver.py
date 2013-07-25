from . import *

import json

from treq.test.util import TestCase

from twisted.internet import defer
from twisted.python.failure import Failure
from twisted.web.http import FORBIDDEN
from twisted.web.proxy import reactor
from twisted.test import proto_helpers

from vulcan.errors import AuthorizationFailed, RESPONSES
from vulcan.httpserver import HTTPFactory, RestrictedChannel
from vulcan import httpserver


class HTTPServerTest(TestCase):
    def setUp(self):
        factory = HTTPFactory()
        self.protocol = factory.buildProtocol(('127.0.0.1', 0))
        self.transport = proto_helpers.StringTransport()
        self.protocol.makeConnection(self.transport)

    @patch.object(RestrictedChannel, 'proxyPass')
    def test_no_auth_header(self, proxyPass):
        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("\r\n")
        status_line = self.transport.value().splitlines()[0]
        self.assertEquals("HTTP/1.1 401 Unauthorized", status_line)
        self.assertFalse(proxyPass.called)

    @patch.object(httpserver, 'authorize')
    @patch.object(RestrictedChannel, 'proxyPass')
    def test_wrong_credentials(self, proxyPass, authorize):
        data = {"message": "Wrong API key"}
        d = defer.fail(
            Failure(AuthorizationFailed(FORBIDDEN, RESPONSES[FORBIDDEN],
                                        json.dumps(data))))
        authorize.return_value = d

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        status_line = self.transport.value().splitlines()[0]
        self.assertEquals(
            "HTTP/1.1 {code} {message}".format(
                code=FORBIDDEN,
                message=RESPONSES[FORBIDDEN]),
            status_line)
        self.assertIn(json.dumps(data), self.transport.value())
        self.assertFalse(proxyPass.called)

    @patch.object(httpserver, 'authorize')
    @patch.object(reactor, 'connectTCP')
    def test_success(self, connectTCP, authorize):
        data = {"auth_token": u"abc", "upstream": u"10.241.0.25:3000"}
        d = defer.succeed(data)
        authorize.return_value = d
        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")
        self.assertEquals("10.241.0.25", connectTCP.call_args[0][0])
        self.assertIsInstance(connectTCP.call_args[0][0], str,
                              "Host should be an encoded bytestring")
        self.assertEquals(3000, connectTCP.call_args[0][1])
