from telephus.pool import CassandraClusterPool
from telephus.client import CassandraClient

from vulcan.timeout import timeout
from vulcan import config


CONN_TIMEOUT = 1


class ResponsiveCassandraClient(CassandraClient):
    @timeout(CONN_TIMEOUT)
    def execute_cql3_query(self, *args, **kwargs):
        return CassandraClient.execute_cql3_query(self, *args, **kwargs)


def initialize():
    global client, pool
    seed_nodes = []
    servers = config['cassandra'].split(",")
    for s in servers:
        host, port = s.split(":")
        seed_nodes.append((host, int(port)))
    pool = CassandraClusterPool(seed_nodes, keyspace=config['keyspace'],
                                conn_timeout=0.5)
    client = ResponsiveCassandraClient(pool)
