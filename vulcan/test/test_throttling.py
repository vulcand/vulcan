from . import *

import time

from treq.test.util import TestCase

from telephus.cassandra.ttypes import *

from twisted.internet import defer

from vulcan import throttling
from vulcan.throttling import ResponsiveCassandraClient
import vulcan


class ThrottlingTest(TestCase):
    def setUp(self):
        throttling.CACHE.clear()

    @patch.object(throttling.CACHE, 'get', Mock())
    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    def test_limits_cached(self, query):
        throttling.get_limits()
        self.assertFalse(query.called)

    @patch.object(throttling.client, 'execute_cql3_query')
    def test_limits_values_converted(self, query):
        query.return_value = defer.succeed(_cassandra_limits())
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

    @patch.object(throttling, 'time', Mock(return_value=40.5))
    @patch.object(vulcan, 'config', {'bucket_size': 2})
    def test_hits_spec(self):
        self.assertEquals(
            {"hit": "auth_token http get http://localhost/test 0",
             "timerange": [10, 40]},
            throttling._hits_spec("auth_token", _limit()))


def _limit():
    return {
        "period": 30,
        "protocol": "http",
        "method": "get",
        "uri": "http://localhost/test",
        "data_size": 0
        }


def _cassandra_limits():
    return CqlResult(rows=[
            CqlRow(columns=[
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
