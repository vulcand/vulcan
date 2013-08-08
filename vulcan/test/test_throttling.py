from . import *

import time
import struct

from treq.test.util import TestCase

from telephus.cassandra.ttypes import *
from telephus.client import CassandraClient

from twisted.internet import defer
from twisted.python.failure import Failure
from twisted.python import log

from vulcan import throttling
from vulcan.throttling import (ResponsiveCassandraClient, _match_limits,
                               check_and_update_rates,
                               _check_rate_against_limit, get_limits)
from vulcan.errors import RateLimitReached, CommunicationFailed, TimeoutError
from vulcan import timeout
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
                               'uri': '.*'}]
                             )

    def test_match_limits(self):
        # limit and request exactly match
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
        get_limits.return_value = defer.succeed([])
        self.successResultOf(check_and_update_rates(_request_params()),
                             None)
        self.assertFalse(_check_and_update_rate.called)

        get_limits.return_value = defer.succeed([_limit()])
        with patch.object(throttling, '_match_limits', Mock(return_value=[])):
            self.successResultOf(check_and_update_rates(_request_params()),
                                 None)
        self.assertFalse(_check_and_update_rate.called)

    @patch.object(throttling, '_update_usage')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_rate_limit_reached(self, get_limits, _get_hits_counters,
                                _update_usage):
        get_limits.return_value = defer.succeed([_limit(data_size=100),
                                                 _limit()])
        _get_hits_counters.return_value = defer.succeed(_cassandra_hits)
        self.assertFailure(
            check_and_update_rates(_request_params()), RateLimitReached)
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
            "update hits using  ttl 30 set counter = counter + 1 "
            "where hit='abc http get http://localhost/test 0' and "
            "ts=40",
            query.call_args[0][1].lower())

    @patch.object(log, 'err')
    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(throttling.client, 'execute_cql3_query')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_update_usage_failure(self, get_limits, _get_hits_counters,
                                  query, log_err):
        get_limits.return_value = defer.succeed([_limit(threshold=3)])
        _get_hits_counters.return_value = defer.succeed(_cassandra_hits)
        # this query will be run only inside _update_usage
        query.return_value = defer.fail(Exception("Bam!!!"))
        self.successResultOf(check_and_update_rates(_request_params()), None)
        self.assertEquals(1, log_err.call_count)

    @patch.object(log, 'err')
    @patch.object(throttling, '_check_rate_against_limit')
    @patch.object(throttling, '_get_hits_counters')
    @patch.object(throttling, 'get_limits')
    def test_check_and_update_rate(self, get_limits, _get_hits_counters,
                                   _check_rate_against_limit, log_err):
        get_limits.return_value = defer.succeed([_limit(threshold=3)])
        _get_hits_counters.return_value = defer.succeed(_cassandra_hits)
        _check_rate_against_limit.return_value = defer.fail(Exception("Bam4!"))

        self.assertFailure(check_and_update_rates(_request_params()),
                           CommunicationFailed)
        self.assertEquals(1, log_err.call_count)

    @patch.object(CassandraClient, 'execute_cql3_query')
    @patch.object(log, 'err')
    def test_communication_failed(self, log_err, query):
        query.return_value = Exception("Bam!")
        self.assertFailure(get_limits(), CommunicationFailed)
        self.assertEquals(1, log_err.call_count)

    @patch.object(throttling.client, 'execute_cql3_query')
    @patch.object(log, 'err')
    def test_timeout(self, log_err, query):
        query.return_value = defer.fail(TimeoutError("Bam!"))
        d = get_limits()
        self.successResultOf(d, [])
        self.assertEquals(1, log_err.call_count)

    @patch.object(log, 'err')
    @patch.object(struct, 'unpack')
    def test_check_rate_against_limit(self, unpack, log_err):
        unpack.side_effect = Exception("Bam!")
        _check_rate_against_limit(_request_params(), _limit(), _cassandra_hits)
        self.assertEquals(1, log_err.call_count)

    @patch.object(throttling.client, 'execute_cql3_query')
    def test_get_hits_counters(self, query):
        throttling._get_hits_counters({"hit": "hit hash", "timerange": [1, 2]})

    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(vulcan, 'config', {'bucket_size': 2})
    def test_hits_spec(self):
        self.assertEquals(
            {"hit": "auth_token http get http://localhost/test 0",
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
        "threshold": 2
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
                Column(timestamp=None, name='uri', value='.*', ttl=None)
                ], key='')
        ], type=1, num=None, schema=CqlMetadata(
        default_value_type='UTF8Type',
        value_types={
            'protocol': 'org.apache.cassandra.db.marshal.UTF8Type',
            'auth_token': 'org.apache.cassandra.db.marshal.UTF8Type',
            'uri': 'org.apache.cassandra.db.marshal.UTF8Type',
            'period': 'org.apache.cassandra.db.marshal.Int32Type',
            'method': 'org.apache.cassandra.db.marshal.UTF8Type',
            'data_size': 'org.apache.cassandra.db.marshal.Int32Type',
            'threshold': 'org.apache.cassandra.db.marshal.Int32Type',
            'id': 'org.apache.cassandra.db.marshal.UUIDType'},
        default_name_type='UTF8Type',
        name_types={'protocol': 'UTF8Type', 'auth_token': 'UTF8Type',
                    'uri': 'UTF8Type', 'period': 'UTF8Type',
                    'method': 'UTF8Type', 'data_size': 'UTF8Type',
                    'threshold': 'UTF8Type', 'id': 'UTF8Type'}))


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
