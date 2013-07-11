from . import *

import json

import treq
from treq.test.util import TestCase

# from twisted.trial.unittest import TestCase
from twisted.internet.error import ConnectionRefusedError
from twisted.internet import defer
from twisted.python.failure import Failure
from twisted.web.client import ResponseDone, ResponseFailed
from twisted.web.http import FORBIDDEN

from vulcan import auth
from vulcan.errors import CommunicationFailed, AuthorizationFailed, RESPONSES
from vulcan import log


class AuthTest(TestCase):
    def setUp(self):
        auth.CACHE.clear()

    def setUpResponse(self, code):
        self.response = Mock(code=code)
        self.protocol = None

        def deliverBody(protocol):
            self.protocol = protocol

        self.response.deliverBody.side_effect = deliverBody

    def test_bad_auth_params(self):
        self.assertRaises(Exception, auth.authorize, {})

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_auth_server_down(self, log_err, treq_get):
        treq_get.return_value = defer.fail(ConnectionRefusedError())
        d = auth.authorize(request_params())
        self.assertFailure(d, CommunicationFailed)
        self.assertTrue(log_err.called)

    @patch.object(treq, 'get')
    def test_wrong_creds(self, treq_get):
        self.setUpResponse(FORBIDDEN)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())

        data = json.dumps({"message": "Wrong API key"})
        self.protocol.dataReceived(data)
        self.protocol.connectionLost(Failure(ResponseDone()))

        self.assertFailure(d, AuthorizationFailed)

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_wrong_creds_fail_receive_reason(self, log_err, treq_get):
        self.setUpResponse(FORBIDDEN)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())

        data = json.dumps({"message": "Wrong API key"})
        self.protocol.dataReceived(data)
        self.protocol.connectionLost(Failure(ResponseFailed("Bam!")))

        self.assertFailure(d, AuthorizationFailed)


def request_params():
    return {
        "username": "user",
        "password": "secret",
        "protocol": "http",
        "method": "get",
        "uri": "http://localhost/test",
        "length": 0
        }
