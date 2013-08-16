# -*- coding: utf-8 -*-

from . import *

from twisted.internet import defer
from twisted.python import log

from treq.test.util import TestCase

from telephus.cassandra.ttypes import *

from vulcan import upstream
from vulcan.upstream import pick_server
from vulcan import routing
from vulcan.cassandra import ResponsiveCassandraClient
from vulcan.errors import TimeoutError
import vulcan


class UpstreamTest(TestCase):
    def setUp(self):
        routing.CACHE.clear()

    def test_pick_server(self):
        self.assertIn(pick_server('10.241.0.25:3001,10.241.0.26:3002'),
                      ['10.241.0.25:3001', '10.241.0.26:3002'])
        with patch.object(upstream, 'is_active', Mock(return_value=False)):
            self.assertIsNone(pick_server('10.241.0.25:3001'))

    @patch.object(routing.CACHE, 'get')
    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    def test_get_servers_cached(self, query, get_from_cache):
        get_from_cache.return_value = {"upstream": "localhost:80"}
        self.successResultOf(upstream.get_servers("api"), "localhost:80")
        self.assertEquals(0, query.call_count)

    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    def test_get_servers_success(self, query):
        query.return_value = defer.succeed(_services)
        self.successResultOf(upstream.get_servers("api"), "localhost:5001")

    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    @patch.object(log, 'err')
    def test_get_servers_timeout(self, log_err, query):
        e = TimeoutError()
        query.side_effect = e
        with patch.object(vulcan, 'config', {}):
            self.assertFailure(upstream.get_servers("api"), TimeoutError)
            log_err.assert_called_once_with(e, "All Cassandra nodes are down")

        log_err.reset_mock()

        # fallback to config
        with patch.object(upstream, 'config', {"api": "localhost:80"}):
            self.successResultOf(upstream.get_servers("api"), "localhost:80")
            log_err.assert_called_once_with(e, "All Cassandra nodes are down")

    @patch.object(ResponsiveCassandraClient, 'execute_cql3_query')
    def test_get_servers_exception(self, query):
        query.side_effect = MyException()
        with patch.object(vulcan, 'config', {}):
            self.assertFailure(upstream.get_servers("api"), MyException)

        # fallback to config
        with patch.object(upstream, 'config', {"api": "localhost:80"}):
            self.successResultOf(upstream.get_servers("api"), "localhost:80")


_services = CqlResult(
    rows=[
        CqlRow(
            columns=[
                Column(timestamp=None, name='path',
                       value='.*/messages', ttl=None),
                Column(timestamp=None, name='upstream',
                       value='localhost:5001', ttl=None)],
            key='')],
    type=1, num=None,
    schema=CqlMetadata(
        default_value_type='UTF8Type',
        value_types={'path': 'org.apache.cassandra.db.marshal.UTF8Type',
                     'name': 'org.apache.cassandra.db.marshal.UTF8Type',
                     'upstream': 'org.apache.cassandra.db.marshal.UTF8Type'},
        default_name_type='UTF8Type',
        name_types={'path': 'UTF8Type',
                    'name': 'UTF8Type',
                    'upstream': 'UTF8Type'}))


class MyException(Exception):
    pass
