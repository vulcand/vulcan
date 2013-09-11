import yaml

from os import path
from mock import patch, Mock
from telephus.pool import CassandraClusterPool

import vulcan


test_yml = path.abspath(
    path.join(path.dirname(__file__), "..", "..", "test.yml"))

with patch.object(CassandraClusterPool, 'make_conn', Mock()):
    with open(test_yml) as f:
        params = yaml.load(f)
    vulcan.initialize(params)
