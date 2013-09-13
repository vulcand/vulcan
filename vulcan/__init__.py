

def initialize(params):
    global config
    config = params

    from vulcan import logger
    logger.initialize(params.get('log') or [])

    from vulcan import cassandra
    cassandra.initialize(
        config['cassandra']['servers'],
        config['cassandra']['keyspace'],
        config['cassandra']['pool_size'],
        config['cassandra']['max_connections_per_node'])
