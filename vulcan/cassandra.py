from telephus.pool import CassandraClusterPool
from telephus.client import CassandraClient

from vulcan.timeout import timeout

CONN_TIMEOUT = 1

class ResponsiveCassandraClient(CassandraClient):
    @timeout(CONN_TIMEOUT)
    def execute_cql3_query(self, *args, **kwargs):
        return CassandraClient.execute_cql3_query(self, *args, **kwargs)


def initialize(servers, keyspace):
    global client, pool

    pool = CassandraClusterPool(
        [(h, p) for h, p in servers],
        keyspace,
        conn_timeout=0.5)
    client = ResponsiveCassandraClient(pool)

