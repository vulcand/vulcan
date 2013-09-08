from os import path

from mock import patch, Mock, call
import yaml
import twisted

import vulcan


# twisted.internet.base.DelayedCall.debug = True
from telephus.pool import CassandraClusterPool

test_yml = path.abspath(path.join(path.dirname(__file__), "..", "..", "test.yml"))

with patch.object(CassandraClusterPool, 'make_conn', Mock()):
    with open(test_yml) as f:
        params = yaml.load(f)
    vulcan.initialize(params)
