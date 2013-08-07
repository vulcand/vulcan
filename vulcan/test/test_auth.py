from . import *

import json

import treq
from treq.test.util import TestCase

from twisted.internet.error import ConnectionRefusedError
from twisted.internet import defer
from twisted.python.failure import Failure
from twisted.web.client import ResponseDone, ResponseFailed
from twisted.web.http import FORBIDDEN, SERVICE_UNAVAILABLE, BAD_GATEWAY, OK
from twisted.python import log

from vulcan import auth
from vulcan.errors import CommunicationFailed, AuthorizationFailed, RESPONSES


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
    def test_4xx(self, treq_get):
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
    def test_4xx_fail_receive_response_body(self, log_err, treq_get):
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
    def test_4xx_response_has_no_reason(self, treq_get):
        self.setUpResponse(FORBIDDEN)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.assertFailure(d, AuthorizationFailed)

        # the failure was cached
        self.assertEqual(
            auth.CACHE.values()[-1].value,
            AuthorizationFailed(FORBIDDEN, RESPONSES[FORBIDDEN], ''))

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_5xx(self, log_err, treq_get):
        self.setUpResponse(BAD_GATEWAY)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        data = {"message": "Bad Gateway"}
        self.protocol.dataReceived(json.dumps(data))
        self.protocol.connectionLost(Failure(ResponseDone()))
        self.assertFailure(d, CommunicationFailed)
        # we log twice:
        # * when we got the response code
        # * when we got the response body
        # this way if we fail to receive response body we'll still know that
        # authorization failed with 5xx code
        self.assertEquals(2, log_err.call_count)
        self.assertIn(str(BAD_GATEWAY), log_err.call_args_list[0][0][0])
        self.assertIn(json.dumps(data), log_err.call_args_list[1][0][0])

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_5xx_fail_receive_response_body(self, log_err, treq_get):
        self.setUpResponse(BAD_GATEWAY)
        treq_get.return_value = defer.succeed(self.response)
        d = auth.authorize(request_params())
        failure = Failure(ResponseFailed("Bam!"))
        self.protocol.connectionLost(failure)
        self.assertFailure(d, CommunicationFailed)
        # we log twice:
        # * we log that authorization failed with 5xx code
        # * we log error that prevented us from getting the response body
        self.assertEquals(2, log_err.call_count)
        self.assertIn(str(BAD_GATEWAY), log_err.call_args_list[0][0][0])
        self.assertEquals(failure, log_err.call_args_list[1][0][0])

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
