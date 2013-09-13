from mock import patch

from treq.test.util import TestCase

from twisted.internet import defer, task

from vulcan.timeout import timeout
from vulcan import timeout as t
from vulcan.errors import TimeoutError


class TimeoutTest(TestCase):
    def setUp(self):
        self.clock = task.Clock()

    def test_call_timeouts(self):
        @timeout(1)
        def f():
            d = defer.Deferred()
            t.reactor.callLater(2, d.callback, None)
            return d

        with patch.object(t, 'reactor', self.clock):
            d = f()
            self.assertFailure(d, TimeoutError)
            self.clock.advance(1)
            return d

    def test_call_not_deferred(self):
        @timeout(1)
        def f():
            pass

        with patch.object(t, 'reactor', self.clock):
            self.successResultOf(f(), None)

    def test_call_not_timeouts(self):
        @timeout(3)
        def f():
            d = defer.Deferred()
            t.reactor.callLater(2, d.callback, None)
            return d

        with patch.object(t, 'reactor', self.clock):
            d = f()

            def _check(r):
                self.assertEquals(r, None)

            d.addBoth(_check)
            self.clock.advance(2)
            return d

    def test_call_raises_exception(self):
        @timeout(1)
        def f():
            return defer.fail(MyException("Bam!"))

        with patch.object(t, 'reactor', self.clock):
            d = f()
            self.assertFailure(d, MyException)
            self.clock.advance(1)
            return d


class MyException(Exception):
    pass
