from random import randint

from functools import partial

from expiringdict import ExpiringDict

from twisted.internet import defer
from twisted.python import log

from vulcan import config
from vulcan.cassandra import client
from vulcan import config
from vulcan.errors import TimeoutError
from vulcan.utils import safe_format


CACHE = ExpiringDict(max_len=100, max_age_seconds=60)


def pick_server(servers):
    """Randomly selects an active server from upstream.

    >>> pick_server('10.241.0.25:3001,10.241.0.26:3002')
    '10.241.0.25:3001'

    A server is considered non-active if no response was received from it
    (the last time we tried). Non-active servers are cached.

    If there are no active servers in the upstream returns None.
    """
    servers = [s for s in servers.split(",") if is_active(s)]
    if servers:
        return servers[randint(0, len(servers) - 1)]


def is_active(server):
    """Returns True if the server isn't among cached non-active servers.

    A server is cached as non-active when we don't get any response from it.
    """
    return True


@defer.inlineCallbacks
def get_servers(service):
    # TODO servers for upstreams should be stored in cassandra
    # so that we could register/unregister servers during servers deployment
    upstream = CACHE.get(service)
    if upstream:
        defer.returnValue(upstream)

    try:
        r = yield client.execute_cql3_query(
            safe_format(
                "select upstream from services where name = '{}'", service))
        upstream = r.rows[0].columns[0].value
        defer.returnValue(upstream)
    except TimeoutError, e:
        log.err("All Cassandra nodes are down")
        defer.returnValue(config.get(service, e))
    except Exception, e:
        defer.returnValue(config.get(service, e))
