# -*- test-case-name: vulcan.test.test_timeout -*-

import functools

from twisted.internet import defer, reactor

from vulcan.errors import TimeoutError


def timeout(secs):
    """
    Decorator to add timeout to Deferred calls

    Credit to theduderog https://gist.github.com/735556
    """

    def wrap(func):
        @functools.wraps(func)
        def func_with_timeout(*args, **kwargs):
            d = defer.maybeDeferred(func, *args, **kwargs)

            timedOut = [False]

            def onTimeout():
                # We set this to true so we can distinguish between
                # someone calling cancel on the deferred we return
                # and our timeout.
                timedOut[0] = True
                d.cancel()

            timesUp = reactor.callLater(secs, onTimeout)

            def onResult(result):
                if timesUp.active():
                    timesUp.cancel()

                return result

            d.addBoth(onResult)

            def onCancelled(failure):
                if failure.check(defer.CancelledError) and timedOut[0]:
                    raise TimeoutError("%s secs have expired" % secs)

                return failure

            d.addErrback(onCancelled)
            return d

        return func_with_timeout
    return wrap
