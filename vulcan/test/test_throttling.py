from . import *

import time
import struct

from treq.test.util import TestCase

from telephus.cassandra.ttypes import *
from telephus.client import CassandraClient

from twisted.internet import defer, task
from twisted import internet
from twisted.python import log

from vulcan import throttling
from vulcan.throttling import (_match_limits, check_and_update_rates,
                               _check_rate_against_limit, get_limits)
from vulcan.cassandra import ResponsiveCassandraClient
from vulcan import cassandra
from vulcan.errors import RateLimitReached
import vulcan


class ThrottlingTest(TestCase):
    def setUp(self):
        throttling.CACHE.clear()

    @patch.object(throttling.CACHE, 'get', Mock())
    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    def test_limits_cached(self, query):
        get_limits()
        self.assertFalse(query.called)

    @patch.object(throttling.client, 'execute_cql3_query')
    def test_limits_values_converted(self, query):
        query.return_value = defer.succeed(_cassandra_limits)
        self.successResultOf(throttling.get_limits(),
                             [{'auth_token': '.*',
                               'data_size': 0,
                               'id': '\x124Vx\x124\x124\x124\x124Vx\x9a\xbc',
                               'method': '.*',
                               'period': 300,
                               'protocol': '.*',
                               'threshold': 2,
                               'uri': '.*',
                               'ip': '127.'}]
                             )

    def test_match_limits(self):
        self.assertEquals([_limit()],
                          _match_limits(_request_params(), [_limit()]))

        # protocol match case insensitive
        self.assertEquals([_limit()],
                          _match_limits(_request_params(protocol="Http"),
                                        [_limit()]),
                          "Protocol match case sensitive")

        # method match case insensitive
        self.assertEquals([_limit()],
                          _match_limits(_request_params(method="Get"),
                                        [_limit()]),
                          "Method match case sensitive")

        # request size matters
        self.assertEquals([], _match_limits(_request_params(length=0),
                                            [_limit(data_size=100)]))

    @patch.object(throttling, '_check_and_update_rate')
    @patch.object(throttling, 'get_limits')
    def test_no_limits(self, get_limits, _check_and_update_rate):
        get_limits.side_effect = lambda *args, **kwargs: defer.succeed([])
        self.successResultOf(check_and_update_rates(_request_params()),
                             None)
        # get_limits called twice:
        # * for custom limits (usually set per account)
        # * for default limits (usually set for all customers)
        self.assertEquals(2, get_limits.call_count)
        get_limits.reset_mock()

        self.assertFalse(_check_and_update_rate.called)
        with patch.object(throttling, '_match_limits', Mock(return_value=[])):
            self.successResultOf(check_and_update_rates(_request_params()),
                                 None)
            self.assertEquals(2, get_limits.call_count)

        self.assertFalse(_check_and_update_rate.called)

    @patch.object(throttling, 'get_limits')
    @patch.object(throttling, '_run_checks')
    def test_custom_limits(self, _run_checks, get_limits):
        """
        Test that if request matches any custom limit no default limit
        will be checked.
        """
        get_limits.return_value = defer.succeed([_limit(auth_token="abc")])
        check_and_update_rates(_request_params())
        # only custom limits are checked
        get_limits.assert_called_once_with(throttling.LIMITS)
        _run_checks.assert_called_once_with(_request_params(),
                                            [_limit(auth_token="abc")])

    @patch.object(throttling, 'get_limits')
    @patch.object(throttling, '_run_checks')
    def test_default_limits(self, _run_checks, get_limits):
        """
        Test that if custom limits are not matched we fallback to
        default limits.
        """
        def f(table):
            # custom limits
            if table == throttling.LIMITS:
                return defer.succeed([_limit(auth_token="abc")])
            elif table == throttling.DEFAULTS:
                return defer.succeed([_limit(auth_token=".*")])

        get_limits.side_effect = f
        check_and_update_rates(_request_params(auth_token="qwerty123"))
        _run_checks.assert_called_once_with(
            _request_params(auth_token="qwerty123"),
            [_limit(auth_token=".*")])

    @patch.object(throttling, '_update_usage')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_rate_limit_reached(self, get_limits, _get_hits_counters,
                                _update_usage):
        # we have 2 limits for requests with length greater then 0 and 100
        # both with threshold 2
        get_limits.return_value = defer.succeed([_limit(data_size=100),
                                                 _limit()])
        # and there are 2 hits (for  any limit that matches)
        _get_hits_counters.return_value = defer.succeed(_cassandra_hits)

        # request matches 2nd limit, now we have 3 hits for it with threshold 2
        # so we should raise RateLimitReached exception
        self.assertFailure(
            check_and_update_rates(_request_params()), RateLimitReached)

        # we don't update usage if we reached rate limit
        self.assertFalse(_update_usage.called)

    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(CassandraClient, 'execute_cql3_query')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_rate_limit_not_reached(self, get_limits, _get_hits_counters,
                                    query):
        get_limits.return_value = defer.succeed([_limit(threshold=3)])
        _get_hits_counters.return_value = defer.succeed(_cassandra_hits)
        self.successResultOf(check_and_update_rates(_request_params()),
                             None)
        self.assertEquals(
            "update hits using ttl 30 set counter = counter + 1 "
            "where hit='127. abc http get http://localhost/test 0' and "
            "ts=40",
            query.call_args[0][1].lower())

    @patch.object(log, 'err')
    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(throttling.client, 'execute_cql3_query')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_update_usage_crash(self, get_limits, _get_hits_counters,
                                  query, log_err):
        """
        Test that if updating usage stats crashes rate limit check still passes.
        """
        get_limits.return_value = defer.succeed([_limit(threshold=3)])
        _get_hits_counters.return_value = defer.succeed(_cassandra_hits)
        query.return_value = defer.fail(Exception("Bam!"))
        self.successResultOf(check_and_update_rates(_request_params()), None)
        self.assertEquals(1, log_err.call_count)

    @patch.object(cassandra, 'CONN_TIMEOUT', 10)
    @patch.object(log, 'err')
    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(throttling.client, 'execute_cql3_query')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_update_usage_slow(self, get_limits, _get_hits_counters,
                                  query, log_err):
        """
        Test that updating usage stats doesn't affect the time we require
        to check rates.
        """
        get_limits.return_value = defer.succeed([_limit(threshold=3)])

        # retrieving hits from cassandra takes 2 seconds
        def g(*args, **kwargs):
            d = defer.Deferred()
            internet.reactor.callLater(2, d.callback, _cassandra_hits)
            return d

        _get_hits_counters.side_effect = g

        # updating usage stats takes 5 seconds
        usage_updated = defer.Deferred()
        def f(*args, **kwargs):
            internet.reactor.callLater(5, usage_updated.callback, None)
            return usage_updated

        query.side_effect = f

        self.clock = task.Clock()
        with patch.object(internet, 'reactor', self.clock):
            # we make deferred call
            d = check_and_update_rates(_request_params())
            self.assertFalse(d.called)
            # and get results in 2 seconds
            self.clock.advance(2)
            self.assertTrue(d.called)
            # usage stats isn't updated yet
            self.assertFalse(usage_updated.called)
            # usage stats will be updated only in 5 seconds
            self.clock.advance(5)
            self.assertTrue(usage_updated.called)

    @patch.object(throttling.client, 'execute_cql3_query')
    def test_get_hits_counters(self, query):
        throttling._get_hits_counters({"hit": "hit hash", "timerange": [1, 2]})

    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(vulcan, 'config', {'bucket_size': 2})
    def test_hits_spec(self):
        self.assertEquals(
            {"hit": "127. auth_token http get http://localhost/test 0",
             "timerange": [10, 40]},
            throttling._hits_spec("auth_token", _limit()))


def _limit(**kwargs):
    d = {
        "auth_token": "abc",
        "period": 30,
        "protocol": "http",
        "method": "get",
        "uri": "http://localhost/test",
        "data_size": 0,
        "threshold": 2,
        "ip": "127."
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


_cassandra_limits = CqlResult(
    rows=[
        CqlRow(
            columns=[
                Column(timestamp=None, name='id',
                       value='\x124Vx\x124\x124\x124\x124Vx\x9a\xbc',
                       ttl=None),
                Column(timestamp=None, name='auth_token',
                       value='.*', ttl=None),
                Column(timestamp=None, name='data_size',
                       value='\x00\x00\x00\x00', ttl=None),
                Column(timestamp=None, name='method',
                       value='.*', ttl=None),
                Column(timestamp=None, name='period',
                       value='\x00\x00\x01,', ttl=None),
                Column(timestamp=None, name='protocol',
                       value='.*', ttl=None),
                Column(timestamp=None, name='threshold',
                       value='\x00\x00\x00\x02', ttl=None),
                Column(timestamp=None, name='uri', value='.*', ttl=None),
                Column(timestamp=None, name='ip', value='127.', ttl=None),
                ], key='')
        ], type=1, num=None, schema=CqlMetadata(
        default_value_type='UTF8Type',
        value_types={
            'protocol': 'org.apache.cassandra.db.marshal.UTF8Type',
            'auth_token': 'org.apache.cassandra.db.marshal.UTF8Type',
            'uri': 'org.apache.cassandra.db.marshal.UTF8Type',
            'ip': 'org.apache.cassandra.db.marshal.UTF8Type',
            'period': 'org.apache.cassandra.db.marshal.Int32Type',
            'method': 'org.apache.cassandra.db.marshal.UTF8Type',
            'data_size': 'org.apache.cassandra.db.marshal.Int32Type',
            'threshold': 'org.apache.cassandra.db.marshal.Int32Type',
            'id': 'org.apache.cassandra.db.marshal.UUIDType'},
        default_name_type='UTF8Type',
        name_types={'protocol': 'UTF8Type', 'auth_token': 'UTF8Type',
                    'uri': 'UTF8Type', 'period': 'UTF8Type',
                    'method': 'UTF8Type', 'data_size': 'UTF8Type',
                    'threshold': 'UTF8Type', 'id': 'UTF8Type',
                    'ip': 'UTF8Type'}))


_cassandra_hits = CqlResult(
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
