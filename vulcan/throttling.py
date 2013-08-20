# -*- test-case-name: vulcan.test.test_throttling -*-

from time import time
import struct

import regex as re

from twisted.internet import defer
from twisted.python import log

from expiringdict import ExpiringDict

from vulcan import config
from vulcan.utils import safe_format
from vulcan.cassandra import client
from vulcan.errors import RateLimitReached


CACHE = ExpiringDict(max_len=100, max_age_seconds=60)
# db tables
LIMITS = "limits"
DEFAULTS = "defaults"


@defer.inlineCallbacks
def get_limits(table=LIMITS):
    limits = CACHE.get(table, [])
    if limits:
        defer.returnValue(limits)

    result = yield client.execute_cql3_query(
        safe_format("select * from {}", table))
    for row in result.rows:
        limit = {column.name: column.value for column in row.columns}
        limit["data_size"] = struct.unpack('>I', limit['data_size'])[0]
        limit["threshold"] = struct.unpack('>I', limit['threshold'])[0]
        limit["period"] = struct.unpack('>I', limit['period'])[0]
        limits.append(limit)

    CACHE[table] = limits
    defer.returnValue(limits)


@defer.inlineCallbacks
def check_and_update_rates(request_params):
    # check custom limits, usually set per account
    limits = yield get_limits(LIMITS)
    limits = _match_limits(request_params, limits)
    if not limits:
        limits = yield get_limits(DEFAULTS)
        limits = _match_limits(request_params, limits)
    yield _run_checks(request_params, limits)


def _match_limits(request_params, limits):
    return [
        limit for limit in limits
        if limit["data_size"] <= request_params["length"] and
        re.match(limit["auth_token"], request_params["auth_token"]) and
        re.match(limit["protocol"], request_params["protocol"],
                 re.IGNORECASE) and
        re.match(limit["method"], request_params["method"],
                 re.IGNORECASE) and
        re.match(limit["uri"], request_params["uri"]) and
        re.match(limit["ip"], request_params["ip"])]


@defer.inlineCallbacks
def _run_checks(request_params, limits):
    for limit in limits:
        yield _check_and_update_rate(request_params, limit)


@defer.inlineCallbacks
def _check_and_update_rate(request_params, limit, *args):
    hits = _hits_spec(request_params["auth_token"], limit)
    counters = yield _get_hits_counters(hits)
    _check_rate_against_limit(request_params, limit, counters)
    _update_usage(hits["hit"], hits["timerange"][1], limit["period"])


@defer.inlineCallbacks
def _get_hits_counters(hits):
    counters = yield client.execute_cql3_query(
        safe_format(
            "select counter from hits where hit='{}' and "
            "ts >= {} and ts <= {}",
            hits['hit'], hits["timerange"][0], hits["timerange"][1]))
    defer.returnValue(counters)


def _check_rate_against_limit(request_params, limit, result):
    rate = 0
    for row in result.rows:
        rate += struct.unpack('>Q', row.columns[0].value)[0]

    if rate >= limit["threshold"]:
        raise RateLimitReached(request_params, limit)


def _update_usage(hit, ts, period):
    """
    Saves request's id/hash called ``hit`` with timestamp ``ts``
    for the next ``period`` seconds. Ignores any exceptions.
    """
    client.execute_cql3_query(
        safe_format(
            "update hits using ttl {} "
            "set counter = counter + 1 where hit='{}' and ts={}",
            period, hit, ts)).addErrback(log.err)


def _hits_spec(auth_token, limit):
    now = int(time())
    time_in_buckets = now / int(config['bucket_size'])
    period_in_buckets = limit['period'] / int(config['bucket_size'])
    start_ts = (time_in_buckets -
                period_in_buckets) * int(config['bucket_size'])
    end_ts = time_in_buckets * int(config['bucket_size'])

    hit = safe_format(
        "{ip} {auth_token} {protocol} {method} {uri} {data_size}",
        ip=limit["ip"],
        auth_token=auth_token,
        protocol=limit['protocol'],
        method=limit['method'],
        uri=limit['uri'],
        period=limit['period'],
        data_size=limit['data_size'])

    return {"hit": hit, "timerange": [start_ts, end_ts]}
