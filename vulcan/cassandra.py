from telephus.pool import CassandraClusterPool
from telephus.client import CassandraClient

from vulcan.timeout import timeout

CONN_TIMEOUT = 1


class ResponsiveCassandraClient(CassandraClient):
    """Tiny wrapper around Telephus's client that adds timeout
    to the queries.
    """
    @timeout(CONN_TIMEOUT)
    def execute_cql3_query(self, *args, **kwargs):
        return CassandraClient.execute_cql3_query(self, *args, **kwargs)


def initialize(servers, keyspace, pool_size, max_connections_per_node):
    global client, pool

    pool = CassandraClusterPool(
        [(h, p) for h, p in servers],
        keyspace,
        conn_timeout=0.5,
        pool_size=pool_size)
    pool.max_connections_per_node = max_connections_per_node
    client = ResponsiveCassandraClient(pool)
