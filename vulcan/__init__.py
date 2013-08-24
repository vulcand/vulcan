
def initialize(params):
    global config
    config = params

    from vulcan import cassandra
    cassandra.initialize(
        config['cassandra']['servers'], config['cassandra']['keyspace'])
