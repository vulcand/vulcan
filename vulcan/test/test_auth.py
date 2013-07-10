from . import *

import treq

from twisted.trial.unittest import TestCase
from twisted.internet.error import ConnectionRefusedError
from twisted.internet import defer

from vulcan import auth
from vulcan.errors import CommunicationFailed
from vulcan import log

class AuthTest(TestCase):
    def setUp(self):
        auth.CACHE.clear()

    def test_on_bad_auth_params(self):
        self.assertRaises(Exception, auth.authorize, {})

    @patch.object(treq, 'get')
    @patch.object(log, 'err')
    def test_on_auth_server_down(self, err, get):
        get.return_value = defer.fail(ConnectionRefusedError())
        d = auth.authorize(request_params())
        self.assertFailure(d, CommunicationFailed)
        self.assertTrue(err.called)


def request_params():
    return {
        "username": "user",
        "password": "secret",
        "protocol": "http",
        "method": "get",
        "uri": "http://localhost/test",
        "length": 0
        }
