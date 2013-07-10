from time import time
import struct
import regex as re
from functools import partial

from twisted.web.error import Error
from twisted.web.http import RESPONSES, FORBIDDEN
from twisted.internet import defer, threads
from twisted.internet.defer import Deferred
from twisted.python.failure import Failure

from telephus.pool import CassandraClusterPool
from telephus.client import CassandraClient

from expiringdict import ExpiringDict

from vulcan import config, log
from vulcan.utils import safe_format
from vulcan.errors import communication_failed, RateLimitReached


CACHE = ExpiringDict(max_len=100, max_age_seconds=60)


def initialize():
    global client, cache
    seed_nodes = []
    servers = config['cassandra'].split(",")
    for s in servers:
        host, port = s.split(":")
        seed_nodes.append((host, int(port)))
    pool = CassandraClusterPool(seed_nodes, keyspace=config['keyspace'])
    pool.startService()
    client = CassandraClient(pool)


def get_limits():
    limits = CACHE.get("limits")
    if limits:
        return defer.succeed(limits)

    limits = client.execute_cql3_query("select * from limits")
    limits.addCallback(_limits_received)
    limits.addErrback(partial(communication_failed, []))
    return limits


def _limits_received(result):
    limits = []
    for row in result.rows:
        limit = {}
        for column in row.columns:
            limit[column.name] = column.value
        limit["data_size"] = struct.unpack('>I', limit['data_size'])[0]
        limit["threshold"] = struct.unpack('>I', limit['threshold'])[0]
        limit["period"] = struct.unpack('>I', limit['period'])[0]
        limits.append(limit)
    CACHE["limits"] = limits
    return limits


def check_and_update_rates(request_params):
    d = get_limits()
    d.addCallback(partial(_match_limits, request_params))
    d.addCallback(partial(_run_checks, request_params))
    return d


def _match_limits(request_params, limits):
    return [limit for limit in limits
            if limit["data_size"] <= request_params["length"] and
            re.match(limit["auth_token"], request_params["auth_token"]) and
            re.match(limit["protocol"], request_params["protocol"]) and
            re.match(limit["method"], request_params["method"]) and
            re.match(limit["uri"], request_params["uri"])]


def _run_checks(request_params, limits):
    d = defer.succeed(None)
    for limit in limits:
        d.addCallback(partial(_check_and_update_rate, request_params, limit))
    return d


def _check_and_update_rate(request_params, limit, _):
    hits = _hits_spec(request_params["auth_token"], limit)
    d = _get_hits_counters(hits)
    d.addCallback(partial(_check_rate_against_limit, request_params, limit))
    d.addCallback(partial(_update_usage,
                          hits["hit"], hits["timerange"][1], limit["period"]))
    d.addErrback(partial(communication_failed, [RateLimitReached]))
    return d


def _get_hits_counters(hits):
    return client.execute_cql3_query(
        safe_format(
            ("select counter from hits where hit='{}' and "
             "ts >= {} and ts <= {}"),
            hits['hit'], hits["timerange"][0], hits["timerange"][1]))


def _check_rate_against_limit(request_params, limit, result):
    try:
        rate = 0
        for row in result.rows:
            rate += struct.unpack('>Q', row.columns[0].value)[0]

        if rate >= limit["threshold"]:
            return Failure(RateLimitReached(request_params, limit))
    except:
        log.err()


def _update_usage(hit, ts, period, _):
    d = client.execute_cql3_query(
        safe_format(
            ("update hits using  ttl {} "
             "set counter = counter + 1 where hit='{}' and ts={}"),
            period, hit, ts))
    d.addErrback(_failed_update_usage)


def _failed_update_usage(failure):
    communication_failed([], failure)


def _hits_spec(auth_token, limit):
    now = int(time())
    time_in_buckets = now / int(config['bucket_size'])
    period_in_buckets = limit['period'] / int(config['bucket_size'])
    start_ts = (time_in_buckets -
                period_in_buckets) * int(config['bucket_size'])
    end_ts = time_in_buckets * int(config['bucket_size'])

    hit = safe_format("{auth_token} {protocol} {method} {uri} {data_size}",
                      auth_token=auth_token,
                      protocol=limit['protocol'],
                      method=limit['method'],
                      uri=limit['uri'],
                      period=limit['period'],
                      data_size=limit['data_size'])

    return {"hit": hit, "timerange": [start_ts, end_ts]}
