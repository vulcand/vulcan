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
from vulcan import cassandra
from vulcan.errors import RateLimitReached
from vulcan.routing import AuthResponse, Rate


class ThrottlingTest(TestCase):
    def test_no_rates(self):
        r = AuthResponse.from_json(
            {"tokens": [{"id": "abc"}],
             "upstreams": [{"url": "http://1.2.3.4:80"}]})
        self.successResultOf(get_upstream(r), r.upstreams[0])

    def test_throttled_rate_comparison(self):
        self.assertEquals(ThrottledRate(rate=Rate(1, "second"), count=1),
                          ThrottledRate(rate=Rate(1, "second"), count=2))
        self.assertGreater(ThrottledRate(rate=Rate(1, "hour"), count=1),
                           ThrottledRate(rate=Rate(1, "second"), count=2))
        self.assertLess(ThrottledRate(rate=Rate(1, "hour"), count=1),
                        ThrottledRate(rate=Rate(1, "day"), count=2))

    @patch.object(throttling, '_now', Mock(return_value=1))
    @patch.object(CassandraClient, 'execute_cql3_query')
    def test_tokens_rate_limit_reached(self, query):
        r = AuthResponse.from_json(
            {"tokens": [{"id": "abc",
                         "rates": [{"value": 1, "period": "minute"},
                                   {"value": 2, "period": "hour"},
                                   {"value": 4, "period": "day"}]
                         }],
             "upstreams": [{"url": "http://1.2.3.4:80"}]})
        query.side_effect = lambda *args, **kwargs: defer.succeed(_2_hits)
        d = get_upstream(r)

        def check(failure):
            e = failure.value
            # only 2 limits are reached: minute and hour limit
            # we pick the limit with the greatest rate period i.e. hour limit
            # and retry when it expires
            self.assertEquals(RateLimitReached(retry_seconds=3599), e)

        d.addErrback(check)

    @patch.object(throttling, '_now', Mock(return_value=1))
    @patch.object(CassandraClient, 'execute_cql3_query')
    @patch.object(throttling, '_update_rates')
    def test_upstreams_rate_limit_reached(self, _update_rates, query):
        r = AuthResponse.from_json(
            {"tokens": [{"id": "abc"}],
             "upstreams": [{"url": "http://1.2.3.4:80",
                            "rates": [{"value": 1, "period": "minute"},
                                      {"value": 2, "period": "hour"},
                                      {"value": 4, "period": "day"}]
                            },
                           {"url": "http://1.2.3.4:90",
                            "rates": [{"value": 1, "period": "minute"}],
                            }]})
        query.side_effect = lambda *args, **kwargs: defer.succeed(_2_hits)
        d = get_upstream(r)

        def check(failure):
            e = failure.value
            # we check 2 upstreams
            # 1.2.3.4:80 could be used after this hour ends
            # 1.2.3.4:90 could be used after this minute ends
            # it's 1 second now so we could retry in 59 seconds
            self.assertEquals(RateLimitReached(retry_seconds=59), e)
            # don't update any rates because no requests to the proxied server
            # have been made
            self.assertEquals(0, _update_rates.call_count)

        d.addErrback(check)

    @patch.object(CassandraClient, 'execute_cql3_query')
    @patch.object(throttling, '_update_rates')
    def test_rate_limit_not_reached(self, _update_rates, query):
        query.side_effect = lambda *args, **kwargs: defer.succeed(_2_hits)
        self.successResultOf(get_upstream(_auth_response),
                             _auth_response.upstreams[0])
        self.assertEquals(
            [call(_auth_response.upstreams[0].url,
                  _auth_response.upstreams[0].rates),
             call(_auth_response.tokens[0].id,
                  _auth_response.tokens[0].rates)],
            _update_rates.call_args_list)

    @patch.object(throttling, 'log')
    @patch.object(throttling, '_get_rates')
    def test_exception(self, _get_rates, log):
        e = Exception("Bam!")
        _get_rates.side_effect = lambda *args, **kw: defer.fail(e)
        self.successResultOf(get_upstream(_auth_response),
                             _auth_response.upstreams[0])
        self.assertTrue(log.err.called)

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
                _2_hits)

            with patch.object(throttling, '_update_rates') as _update_rates:
                _update_rates.side_effect = _crashing_update_rates
                self.successResultOf(get_upstream(_auth_response),
                                     _auth_response.upstreams[0])
                log_err.assert_called_once_with(f)

    @patch.object(cassandra, 'CONN_TIMEOUT', 10)
    @patch.object(log, 'err')
    @patch.object(throttling.time, 'time', Mock(return_value=40.5))
    @patch.object(throttling.client, 'execute_cql3_query')
    @patch.object(throttling, '_get_rates')
    def test_update_rates_slow(self, _get_rates, update_rate_query, log_err):
        """
        Test that updating rates doesn't affect the time we require
        to check them.
        """
        self.clock = task.Clock()

        # retrieving rates takes 2 seconds
        def get_rates(key, rates):
            d = defer.Deferred()
            internet.reactor.callLater(
                2, d.callback,
                # make counter always less than the limit
                [ThrottledRate(rate=rates[0], count=rates[0].value - 1)])
            return d

        _get_rates.side_effect = get_rates

        # updating rates takes 5 seconds
        updated_rates = []

        def update_rate(*args, **kwargs):
            d = defer.Deferred()
            internet.reactor.callLater(5, d.callback, None)
            updated_rates.append(d)
            return d

        update_rate_query.side_effect = update_rate

        with patch.object(internet, 'reactor', self.clock):
            # we make deferred call
            d = get_upstream(_auth_response)
            self.assertFalse(d.called)

            # get rates for tokens in 2 seconds
            self.clock.advance(2)

            # get rates for upstreams in 2 seconds
            self.clock.advance(2)

            # we already got upstream
            self.assertTrue(d.called)
            # rates aren't updated yet
            self.assertFalse(any([ur.called for ur in updated_rates]))
            # usage stats will be updated only in 5 seconds
            self.clock.advance(5)
            self.assertTrue(all([ur.called for ur in updated_rates]))


_2_hits = CqlResult(
    rows=[
        CqlRow(
            columns=[
                Column(timestamp=None, name='counter',
                       value='\x00\x00\x00\x00\x00\x00\x00\x02', ttl=None)
                ],
            key=''),
        ],
    type=1, num=None, schema=CqlMetadata(
        default_value_type='UTF8Type',
        value_types={
            'counter': 'org.apache.cassandra.db.marshal.CounterColumnType'},
        default_name_type='UTF8Type', name_types={'counter': 'UTF8Type'}))


_auth_response = AuthResponse.from_json(
    {"tokens": [{"id": "abc",
                 "rates": [{"value": 300, "period": "minute"}]
                 }],
     "upstreams": [{"url": "http://127.0.0.1:5000",
                    "rates": [{"value": 1800, "period": "hour"}]
                    }],
     "headers": {"X-Real-Ip": "1.2.3.4"}})
