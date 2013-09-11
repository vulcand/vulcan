#!/usr/bin/env python
import argparse

from twisted.internet import reactor
from twisted.web.client import HTTPConnectionPool
from twisted.internet import defer
from twisted.internet.error import ConnectionLost
from twisted.web._newclient import ResponseNeverReceived

import treq

from datetime import datetime

class Params(object):
    # settings
    url = None
    max_running_requests = None
    request_size_bytes = None
    request = None
    auth = ('api', 'bla')
    method = {}

    # counters
    req_generated = 0
    req_current = 0
    start = datetime.utcnow()

    # connectors and semaphores
    pool = HTTPConnectionPool(reactor)
    semaphore = None

    @classmethod
    def from_args(cls, args):
        cls.url = args.url
        cls.request = {'a': 'b' * args.bytes}
        cls.max_running_requests = args.requests

        cls.semaphore = defer.DeferredSemaphore(cls.max_running_requests)
        cls.method = {'method': getattr(treq, args.method) }


def requests_per_second(date_point, requests):
    delta_seconds = (datetime.utcnow() - date_point).total_seconds()
    if delta_seconds:
        return requests/(delta_seconds * 1.0)
    else:
        return 0


def counter():
    print datetime.utcnow(), "Requests: {} generated; {} current, Requests/second: {}".format(
        Params.req_generated,
        Params.req_current,
        requests_per_second(Params.start, Params.req_generated))

    last_flushed = (datetime.utcnow() - Params.start).total_seconds()
    if last_flushed > 10:
        print "Flushing metrics"
        Params.start = datetime.utcnow()
        Params.req_generated = 0

    # schedule ourselves
    reactor.callLater(1, counter)


@defer.inlineCallbacks
def request():
    response = None
    content = None

    try:
        Params.req_generated += 1
        Params.req_current += 1

        response = yield Params.method['method'](
            url=Params.url,
            auth=Params.auth,
            data=Params.request,
            pool=Params.pool,
            persistent=True)

        content = yield treq.content(response)
        Params.req_current -= 1
        if response.code != 200:
            print "Got error response:", response.code if response else None, content
    except Exception, f:
        while isinstance(f, list):
            f = f[0]
        if not isinstance(f, (ConnectionLost, ResponseNeverReceived)):
            print "Got response:", response.code if response else None, content, f
    finally:
        Params.semaphore.release()

@defer.inlineCallbacks
def generate():
    while True:
        yield Params.semaphore.acquire()
        request()

def parse_args():
    p = argparse.ArgumentParser(
        description="Bombard URL with requests")

    p.add_argument(
        "url", metavar='<URL>', help="URL to bombard.")

    p.add_argument(
        "--requests", "-r",
        type=int,
        metavar='<REQUESTS>',
        help="The amount of concurrent requests",
        default=10)

    p.add_argument(
        "--bytes", "-b",
        metavar='<BYTES>',
        type=int,
        default=10,
        help="Request size in bytes")

    p.add_argument('--method', "-m",
                   choices=['post', 'get'],
                   help="request method",
                   default="get")

    return p.parse_args()


def main():
    args = parse_args()
    print args
    Params.from_args(args)
    generate()
    reactor.callLater(1, counter)
    reactor.run()


if __name__ == '__main__':
    main()

