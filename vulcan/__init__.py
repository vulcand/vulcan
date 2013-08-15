from vulcan.utils import load_config


def initialize(ini_file):
    global config

    config = load_config(ini_file)

    from vulcan import cassandra

    cassandra.initialize()
