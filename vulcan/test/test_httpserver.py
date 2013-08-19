from . import *

import json

from twisted.trial.unittest import TestCase

from twisted.internet import defer, task
from twisted.python.failure import Failure
from twisted.web.http import SERVICE_UNAVAILABLE, UNAUTHORIZED
from twisted.web.proxy import reactor
from twisted.test import proto_helpers
from twisted.python import log

from vulcan.errors import (AuthorizationFailed, RateLimitReached, RESPONSES,
                           TOO_MANY_REQUESTS, CommunicationFailed)
from vulcan.httpserver import HTTPFactory, RestrictedChannel
from vulcan import httpserver
from vulcan import httpserver as hs
from vulcan import throttling as th


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

    @patch.object(hs.auth, 'authorize')
    @patch.object(RestrictedChannel, 'proxyPass')
    def test_wrong_credentials(self, proxyPass, authorize):
        data = {"message": "Wrong API key"}
        d = defer.fail(
            Failure(AuthorizationFailed(UNAUTHORIZED, RESPONSES[UNAUTHORIZED],
                                        json.dumps(data))))
        authorize.return_value = d

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        status_line = self.transport.value().splitlines()[0]
        self.assertEquals(
            "HTTP/1.1 {code} {message}".format(
                code=UNAUTHORIZED,
                message=RESPONSES[UNAUTHORIZED]),
            status_line)
        self.assertIn(json.dumps(data), self.transport.value())
        self.assertFalse(proxyPass.called)

    @patch.object(reactor, 'connectTCP')
    @patch.object(th, 'get_limits')
    @patch.object(th, '_run_checks')
    @patch.object(hs.auth, 'authorize')
    def test_success(self, authorize, run_checks, get_limits, connectTCP):
        authorize.return_value = defer.succeed(
            {"auth_token": u"abc", "upstream": u"10.241.0.25:3000"})

        get_limits.return_value = defer.succeed([_limit()])
        run_checks.return_value = defer.succeed(None)
        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")
        self.assertEquals("10.241.0.25", connectTCP.call_args[0][0])
        self.assertIsInstance(connectTCP.call_args[0][0], str,
                              "Host should be an encoded bytestring")
        self.assertEquals(3000, connectTCP.call_args[0][1])
        self.assertEquals([_limit()], run_checks.call_args[0][1])

    def test_errorToHTTPResponse(self):
        request = Mock()
        request_params = {
            "method": "HTTP",
            "uri": "http:localhost"
            }
        limit = {
            "threshold": 10,
            "period": 60
            }
        rc = RestrictedChannel()

        # RateLimitReached
        f = Failure(RateLimitReached(request_params, limit))
        rc.errorToHTTPResponse(request, f)
        request.setResponseCode.assert_called_once_with(
            TOO_MANY_REQUESTS,
            RESPONSES[TOO_MANY_REQUESTS])
        request.write.assert_called_once_with(f.getErrorMessage())
        self.assertEquals(1, request.finishUnreceived.call_count)
        request.reset_mock()

        # AuthorizationFailed
        f = Failure(AuthorizationFailed(UNAUTHORIZED, RESPONSES[UNAUTHORIZED],
                                        "Wrong API key"))
        rc.errorToHTTPResponse(request, f)
        request.setResponseCode.assert_called_once_with(
            UNAUTHORIZED,
            RESPONSES[UNAUTHORIZED])
        request.write.assert_called_once_with(f.value.response)
        self.assertEquals(1, request.finishUnreceived.call_count)

        request.reset_mock()

        # AuthorizationFailed with no response body
        f = Failure(AuthorizationFailed(UNAUTHORIZED, RESPONSES[UNAUTHORIZED],
                                        response=None))
        rc.errorToHTTPResponse(request, f)
        request.setResponseCode.assert_called_once_with(
            UNAUTHORIZED,
            RESPONSES[UNAUTHORIZED])
        request.write.assert_called_once_with("")
        self.assertEquals(1, request.finishUnreceived.call_count)

        request.reset_mock()

        # CommunicationFailed
        f = Failure(CommunicationFailed(Exception("Bam!")))
        rc.errorToHTTPResponse(request, f)
        request.setResponseCode.assert_called_once_with(
            SERVICE_UNAVAILABLE,
            RESPONSES[SERVICE_UNAVAILABLE])
        request.write.assert_called_once_with("")
        self.assertEquals(1, request.finishUnreceived.call_count)

        request.reset_mock()

        # unexpected error
        f = Failure(Exception("Bam!"))
        with patch.object(log, "err") as log_err:
            rc.errorToHTTPResponse(request, f)
        request.setResponseCode.assert_called_once_with(
            SERVICE_UNAVAILABLE,
            RESPONSES[SERVICE_UNAVAILABLE])
        request.write.assert_called_once_with("")
        log_err.assert_called_once_with(f)
        self.assertEquals(1, request.finishUnreceived.call_count)

    @patch.object(reactor, 'connectTCP')
    @patch.object(httpserver, 'check_and_update_rates')
    def test_request_received_before_checks(self,
                                            check_and_update_rates,
                                            connectTCP):
        self.clock = task.Clock()
        data = {"auth_token": u"abc", "upstream": u"10.241.0.25:3000"}

        def delayed_auth(*args, **kwargs):
            d = defer.Deferred()
            self.clock.callLater(2, d.callback, data)
            return d

        with patch.object(hs.auth, 'authorize', delayed_auth):
            check_and_update_rates.return_value = defer.succeed(None)
            self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
            self.protocol.dataReceived(
                "Authorization: Basic YXBpOmFwaWtleQ==\r\n")
            self.protocol.dataReceived("\r\n")
            self.clock.advance(5)
            self.assertEquals("10.241.0.25", connectTCP.call_args[0][0])
            self.assertIsInstance(connectTCP.call_args[0][0], str,
                                  "Host should be an encoded bytestring")
            self.assertEquals(3000, connectTCP.call_args[0][1])

    @patch.object(hs, 'check_and_update_rates')
    @patch.object(hs.auth, 'authorize')
    def test_clientConnectionFailed(self, authorize, check_and_update_rates):
        # assume there is nobody listening on this port
        data = {"auth_token": u"abc", "upstream": u"127.0.0.1:69"}
        d = defer.succeed(data)
        authorize.return_value = d

        check_and_update_rates.return_value = defer.succeed(None)

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

        class DynamicallyRoutedRequest(httpserver.DynamicallyRoutedRequest):
            proxyClientFactoryClass = ReportingProxyClientFactory

            def finish(self_, *args, **kwargs):
                # finally our assertions

                # we log that vulcan couldn't connect to the proxied server
                # e.g. if vulcan tries to connect on the wrong port
                self.assertTrue(self_.log_err.called)
                status_line = self.transport.value().splitlines()[0]
                self.assertEquals(
                    "HTTP/1.1 {code} {message}".format(
                        code=SERVICE_UNAVAILABLE,
                        message=RESPONSES[SERVICE_UNAVAILABLE]),
                    status_line)
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
    @patch.object(hs, 'check_and_update_rates')
    @patch.object(hs.auth, 'authorize')
    @patch.object(log, 'err')
    def test_throttling_crashes(self, log_err, authorize,
                                check_and_update_rates, connectTCP):
        authorize.return_value = defer.succeed(
            {"auth_token": u"abc", "upstream": u"10.241.0.25:3000"})

        e = Exception("Bam!")

        check_and_update_rates.side_effect = lambda *args: defer.fail(e)

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        log_err.assert_called_once_with(e)
        self.assertEquals("10.241.0.25", connectTCP.call_args[0][0])
        self.assertIsInstance(connectTCP.call_args[0][0], str,
                              "Host should be an encoded bytestring")
        self.assertEquals(3000, connectTCP.call_args[0][1])

    @patch.object(RestrictedChannel, 'proxyPass')
    @patch.object(hs, 'check_and_update_rates')
    @patch.object(hs.auth, 'authorize')
    def test_rate_limit_reached(self, authorize,
                                check_and_update_rates, proxyPass):
        authorize.return_value = defer.succeed(
            {"auth_token": u"abc", "upstream": u"10.241.0.25:3000"})

        check_and_update_rates.side_effect = lambda *args: defer.fail(
            RateLimitReached(_request_params(), _limit()))

        self.protocol.dataReceived("GET /foo/bar HTTP/1.1\r\n")
        self.protocol.dataReceived("Authorization: Basic YXBpOmFwaWtleQ==\r\n")
        self.protocol.dataReceived("\r\n")

        status_line = self.transport.value().splitlines()[0]
        self.assertEquals(
            "HTTP/1.1 {code} {message}".format(
                code=TOO_MANY_REQUESTS,
                message=RESPONSES[TOO_MANY_REQUESTS]),
            status_line)

        self.assertFalse(proxyPass.called)


def _limit(**kwargs):
    d = {
        "auth_token": "abc",
        "period": 30,
        "protocol": "http",
        "method": "get",
        "uri": "/foo/bar",
        "data_size": 0,
        "threshold": 2,
        "ip": ".*"
        }
    d.update(kwargs)
    return d


def _request_params(**kwargs):
    d = {
        "auth_token": "abc",
        "protocol": "http",
        "method": "get",
        "uri": "http://localhost/test",
        "length": 0,
        "ip": "127.0.0.1"
        }
    d.update(kwargs)
    return d
