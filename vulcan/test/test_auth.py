from . import *

import json

import treq
from treq.test.util import TestCase

# from twisted.trial.unittest import TestCase
from twisted.internet.error import ConnectionRefusedError
from twisted.internet import defer
from twisted.python.failure import Failure
from twisted.web.client import ResponseDone, ResponseFailed
from twisted.web.http import FORBIDDEN, OK

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
        self.assertEquals(auth.CACHE, {})

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_auth_server_down(self, log_err, treq_get):
        treq_get.return_value = defer.fail(ConnectionRefusedError())
        d = auth.authorize(request_params())
        self.assertFailure(d, CommunicationFailed)
        self.assertTrue(log_err.called)
        self.assertEquals(auth.CACHE, {})

    @patch.object(treq, 'get')
    def test_wrong_creds(self, treq_get):
        self.setUpResponse(FORBIDDEN)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        data = {"message": "Wrong API key"}
        self.protocol.dataReceived(json.dumps(data))
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.assertFailure(d, AuthorizationFailed)

        # the failure was cached
        self.assertEqual(
            auth.CACHE.values()[-1].value,
            AuthorizationFailed(FORBIDDEN, RESPONSES[FORBIDDEN],
                                json.dumps(data)))

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_wrong_creds_fail_receive_reason(self, log_err, treq_get):
        self.setUpResponse(FORBIDDEN)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        self.protocol.connectionLost(Failure(ResponseFailed("Bam!")))
        self.assertFailure(d, AuthorizationFailed)
        self.assertTrue(log_err.called)

        # the failure was cached
        self.assertEqual(
            auth.CACHE.values()[-1].value,
            AuthorizationFailed(FORBIDDEN, RESPONSES[FORBIDDEN]))

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_pass_auth_fail_receive_response_body(self, log_err, treq_get):
        self.setUpResponse(OK)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        self.protocol.connectionLost(Failure(ResponseFailed("Bam!")))
        self.assertFailure(d, CommunicationFailed)
        self.assertTrue(log_err.called)
        self.assertEquals(auth.CACHE, {})

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_pass_auth_receive_bad_json(self, log_err, treq_get):
        self.setUpResponse(OK)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        self.protocol.dataReceived("not json")
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.assertFailure(d, CommunicationFailed)
        self.assertTrue(log_err.called)
        self.assertEquals(auth.CACHE, {})

    @patch.object(treq, 'get')
    def test_auth_pass(self, treq_get):
        self.setUpResponse(OK)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        data = {"auth_token": 123, "upstream": "localhost:8080"}
        self.protocol.dataReceived(json.dumps(data))
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.successResultOf(d, data)
        # success is cached
        self.assertEqual(auth.CACHE.values()[-1], data)

    @patch.object(auth.CACHE, 'get', Mock())
    @patch.object(treq, 'get')
    def test_auth_cached(self, treq_get):
        auth.authorize(request_params())
        self.assertFalse(treq_get.called)


def request_params():
    return {
        "username": "user",
        "password": "secret",
        "protocol": "http",
        "method": "get",
        "uri": "http://localhost/test",
        "length": 0
        }
