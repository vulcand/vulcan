from . import *

import time
import struct

from treq.test.util import TestCase

from telephus.cassandra.ttypes import *
from telephus.client import CassandraClient

from twisted.internet import defer, task
from twisted import internet
from twisted.python import log
from twisted.python.failure import Failure

from vulcan import throttling
from vulcan.throttling import get_upstream, ThrottledRate
from vulcan.cassandra import ResponsiveCassandraClient
from vulcan import cassandra
from vulcan.errors import RateLimitReached
import vulcan
from vulcan.routing import AuthResponse


class ThrottlingTest(TestCase):
    def test_no_rates(self):
        self.successResultOf(get_upstream(_auth_response_with_no_rates),
                             _auth_response_with_no_rates.upstreams[0])

    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    def test_tokens_rate_limit_reached(self, query):
        query.return_value = _cassandra_counters
        self.assertFailure(
            get_upstream(_auth_response_low_tokens_rate), RateLimitReached)

    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    @patch.object(throttling, '_update_rates')
    def test_upstreams_rate_limit_reached(self, _update_rates, query):
        query.return_value = _cassandra_counters
        self.assertFailure(
            get_upstream(_auth_response_low_upstreams_rate), RateLimitReached)
        self.assertEquals(0, _update_rates.call_count)

    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    @patch.object(throttling, '_update_rates')
    def test_rate_limit_not_reached(self, _update_rates, query):
        query.return_value = _cassandra_counters
        self.successResultOf(get_upstream(_auth_response),
                             _auth_response.upstreams[0])
        self.assertEquals(
            [call(_auth_response.upstreams[0].url,
                  _auth_response.upstreams[0].rates),
             call(_auth_response.tokens[0].id,
                  _auth_response.tokens[0].rates)],
            _update_rates.call_args_list)

    @patch.object(log, 'err')
    def test_update_rates_crash(self, log_err):
        """
        Test that if updating rates crashes rate limit check still passes.
        """
        with patch.object(throttling.client, 'execute_cql3_query') as query:
            f = Failure(Exception("Bam!"))
            query.return_value = defer.fail(f)
            _crashing_update_rates = throttling._update_rates

        with patch.object(throttling.client, 'execute_cql3_query') as query:
            query.side_effect = lambda *args, **kwargs: defer.succeed(
                _cassandra_counters)

            with patch.object(throttling, '_update_rates') as _update_rates:
                _update_rates.side_effect = _crashing_update_rates
                self.successResultOf(get_upstream(_auth_response),
                                     _auth_response.upstreams[0])
                log_err.assert_called_once_with(f)


_cassandra_counters = CqlResult(
    rows=[
        CqlRow(
            columns=[
                Column(timestamp=None, name='counter',
                       value='\x00\x00\x00\x00\x00\x00\x00\x01', ttl=None)
                ],
            key=''),
        CqlRow(
            columns=[
                Column(timestamp=None, name='counter',
                       value='\x00\x00\x00\x00\x00\x00\x00\x01', ttl=None)
                ],
            key='')
        ],
    type=1, num=None, schema=CqlMetadata(
        default_value_type='UTF8Type',
        value_types={
            'counter': 'org.apache.cassandra.db.marshal.CounterColumnType'},
        default_name_type='UTF8Type', name_types={'counter': 'UTF8Type'}))


_auth_response_with_no_rates = AuthResponse.from_json(
    {"tokens": [{"id": "abc"}],
     "upstreams": [{"url": "http://127.0.0.1:5000"}]})


_auth_response_low_tokens_rate = AuthResponse.from_json(
    {"tokens": [{"id": "abc",
                 "rates": [{"value": 2, "period": "minute"}]
                 }],
     "upstreams": [{"url": "http://127.0.0.1:5000",
                    "rates": [{"value": 1800, "period": "hour"}]
                    }],
     "headers": {"X-Real-Ip": "1.2.3.4"}})


_auth_response_low_upstreams_rate = AuthResponse.from_json(
    {"tokens": [{"id": "abc",
                 "rates": [{"value": 300, "period": "minute"}]
                 }],
     "upstreams": [{"url": "http://127.0.0.1:5000",
                    "rates": [{"value": 2, "period": "hour"}]
                    }],
     "headers": {"X-Real-Ip": "1.2.3.4"}})


_auth_response = AuthResponse.from_json(
    {"tokens": [{"id": "abc",
                 "rates": [{"value": 300, "period": "minute"}]
                 }],
     "upstreams": [{"url": "http://127.0.0.1:5000",
                    "rates": [{"value": 1800, "period": "hour"}]
                    }],
     "headers": {"X-Real-Ip": "1.2.3.4"}})
