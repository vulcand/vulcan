from os import path

from mock import patch, Mock
from twisted.internet import epollreactor
import twisted

import vulcan

twisted.internet.base.DelayedCall.debug = True

vulcan.initialize(path.join(path.dirname(__file__), "..", "..", "test.ini"))

epollreactor.install()

from vulcan import throttling
from telephus.pool import CassandraClusterPool

with patch.object(CassandraClusterPool, 'make_conn', Mock()):
    throttling.initialize()
