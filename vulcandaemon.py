from os import path, getpid
import sys
import argparse

import setproctitle

from twisted.internet import epollreactor
from twisted.python import log

import vulcan
from vulcan.logging import CustomizableFileLogObserver


def parse_args():
    p = argparse.ArgumentParser(
        description="Proxies HTTP(S) and SMTP requests")
    p.add_argument("--smtp-port", "-m", default=5050, type=int,
                   metavar='<PORT>', help="SMTP port number to listen on.")
    p.add_argument("--http-port", "-p", default=8080, type=int,
                   metavar='<PORT>', help="HTTP port number to listen on.")
    p.add_argument("--admin-port", "-a", default=8096, type=int,
                   metavar='<PORT>', help="HTTP port for admin interface.")
    p.add_argument("--ini-file", "-f", metavar='<FILENAME>', required=True,
                   help="config file name")
    p.add_argument('--pid-file', help="pid file path")
    return p.parse_args()

def initialize(args, process_name="vulcan"):

    vulcan.initialize(path.join(path.dirname(__file__), args.ini_file))
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
    from twisted.web.server import Site

    from vulcan.httpserver import HTTPFactory
    from vulcan import cassandra
    from vulcan.routing import AdminResource

    cassandra.pool.startService()
    reactor.listenTCP(args.http_port, HTTPFactory())
    reactor.listenTCP(args.admin_port, Site(AdminResource()))
    reactor.suggestThreadPoolSize(vulcan.config.get("numthreads", 10))
    reactor.run()


if __name__ == '__main__':
    main()
