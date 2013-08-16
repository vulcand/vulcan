from os import path

from mock import patch, Mock
from twisted.internet import epollreactor
import twisted

import vulcan


twisted.internet.base.DelayedCall.debug = True
epollreactor.install()

from telephus.pool import CassandraClusterPool

with patch.object(CassandraClusterPool, 'make_conn', Mock()):
    vulcan.initialize(
        path.join(path.dirname(__file__), "..", "..", "test.ini"))
