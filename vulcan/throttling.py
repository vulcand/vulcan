# -*- test-case-name: vulcan.test.test_throttling -*-

from time import time
import struct

from twisted.internet import defer
from twisted.python import log

from vulcan.utils import safe_format, shuffled
from vulcan.cassandra import client
from vulcan.errors import RateLimitReached


@defer.inlineCallbacks
def get_upstream(request):
    try:
        for token in request.tokens:
            throttled = yield _get_rates(token.id, token.rates)
            if any(throttled):
                raise RateLimitReached(
                    _retry_seconds(max(throttled)))

        retries = []
        for u in shuffled(request.upstreams):
            throttled = yield _get_rates(u.url, token.rates)
            if any(throttled):
                retries.append(_retry_seconds(max(throttled)))
            else:
                _update_rates(u.url, u.rates)
                for token in request.tokens:
                    _update_rates(token.id, token.rates)
                defer.returnValue(u)

        if len(retries) == len(request.upstreams):
            raise RateLimitReached(min(retries))

    except RateLimitReached:
        raise

    except Exception:
        log.err(safe_format("Failed to throttle: {}", request))


@defer.inlineCallbacks
def _get_rates(key, rates):
    out = []
    for rate in rates:
        throttled = yield _is_throttled(key, rate)
        out.append(throttled)
    defer.returnValue(out)


def _update_rates(key, rates):
    for rate in rates:
        _update_rate(key, rate)


@defer.inlineCallbacks
def _is_throttled(key, rate):
    result = yield client.execute_cql3_query(
        safe_format(
            "select counter from hits where hit='{}'",
            _hit(key, rate)))

    defer.returnValue(_result_to_int(result))


def _update_rate(key, rate):
    client.execute_cql3_query(
        safe_format(
            "update hits using ttl {} "
            "set counter = counter + 1 where hit='{}'",
            rate.period_as_seconds, _hit(key, rate))).addErrback(log.err)


def _hit(key, rate):
    return safe_format("{}_{}_{}", key, rate.period, _now())


def _result_to_int(result):
    val = 0
    for row in result.rows:
        val += struct.unpack('>Q', row.columns[0].value)[0]
    return val

def _retry_seconds(throttled):
    now = _now()
    return now/throttled.rate.period_as_seconds + throttled.rate.period - now

def _now():
    return int(time.time())

class ThrottledRate(object):
    def __init__(self, rate, throttled):
        self.rate = rate
        self.throttled = throttled

    def __nonzero__(self):
        return self.throttled

    def __cmp__(self, other):
        s1 = self.rate.period_as_seconds
        s2 = other.rate.period_as_seconds

        if s1 < s2:
            return -1
        elif s1 == s2:
            return 0
        else:
            return 1
