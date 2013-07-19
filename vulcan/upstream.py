from random import randint

from vulcan import config


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


def get_servers(upstream):
    # TODO servers for upstreams should be stored in cassandra
    # so that we could register/unregister servers during servers deployment
    return config[upstream]
