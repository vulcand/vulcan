from mock import patch, Mock

import json

import treq
from treq.test.util import TestCase

from twisted.internet.error import ConnectionRefusedError
from twisted.internet import defer
from twisted.python.failure import Failure
from twisted.web.client import ResponseDone
from twisted.web.http import FORBIDDEN, OK
from twisted.python import log

from vulcan import auth
from vulcan.errors import AuthorizationFailed
from vulcan.routing import AuthRequest, AuthResponse


class AuthTest(TestCase):
    def setUpResponse(self, code):
        self.response = Mock(code=code)
        self.protocol = None

        def deliverBody(protocol):
            self.protocol = protocol

        self.response.deliverBody.side_effect = deliverBody

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_auth_server_down(self, log_err, treq_get):
        treq_get.return_value = defer.fail(ConnectionRefusedError())
        d = auth.authorize(_request)
        self.assertFailure(d, ConnectionRefusedError)
        self.flushLoggedErrors()

    @patch.object(treq, 'get')
    def test_4xx(self, treq_get):
        self.setUpResponse(FORBIDDEN)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(_request)
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.assertFailure(d, AuthorizationFailed)
        self.flushLoggedErrors()

    @patch.object(treq, 'get')
    def test_auth_pass(self, treq_get):
        self.setUpResponse(OK)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(_request)
        data = {"tokens": [{"id": "abc"}],
                "upstreams": [{"url": "http://localhost:8080"}]}

        self.protocol.dataReceived(json.dumps(data))
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.successResultOf(d, AuthResponse.from_json(data))
        self.flushLoggedErrors()


_request = AuthRequest(
    "api", "secret", "http", "get", "http://localhost/test", 0, "1.2.3.4")
