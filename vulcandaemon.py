from os import getpid

import sys
import argparse
import yaml

import setproctitle

from twisted.internet import epollreactor
from twisted.python import log

import vulcan
from vulcan.logging import CustomizableFileLogObserver


def parse_args():
    p = argparse.ArgumentParser(
        description="Proxies HTTP(S) and SMTP requests")

    p.add_argument("--http-port", "-p", default=8080, type=int,
                   metavar='<PORT>', help="HTTP port number to listen on.")
    p.add_argument("--config", "-c", metavar='<FILENAME>', required=True,
                   help="config file name")
    p.add_argument('--pid-file', help="pid file path")

    return p.parse_args()

def initialize(args, process_name="vulcan"):

    with open(args.config) as f:
        params = yaml.load(f)
        print params

    vulcan.initialize(params)

    # Create the pidfile:
    if args.pid_file:
        with open(args.pid_file, 'w') as pidfile:
            pidfile.write(str(getpid()))

    # Change the name of the process to "vulcan"
    setproctitle.setproctitle(process_name)

    log.addObserver(CustomizableFileLogObserver(sys.stdout).emit)


def main():
    # pick epoll()-based twisted reactor. this needs to appear before
    # any other Twisted imports
    epollreactor.install()
    args = parse_args()
    initialize(args)

    from twisted.internet import reactor

    from vulcan.httpserver import HTTPFactory
    from vulcan import cassandra

    cassandra.pool.startService()
    reactor.listenTCP(args.http_port, HTTPFactory())
    reactor.suggestThreadPoolSize(vulcan.config.get("numthreads", 10))
    reactor.run()


if __name__ == '__main__':
    main()
