from random import randint

from vulcan import config


def pick_server(servers):
    servers = [s for s in servers.split(",") if is_active(s)]
    return servers[randint(0, len(servers) - 1)]


def is_active(server):
    return True


def get_servers(upstream):
    # TODO servers for upstreams should be stored in cassandra
    # so that we could register/unregister servers during servers deployment
    return config[upstream]
