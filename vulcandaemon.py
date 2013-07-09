from os import path, getpid
import sys
import argparse

import setproctitle

from twisted.internet import epollreactor
from twisted.python import usage

import vulcan


def parse_args():
    global args
    p = argparse.ArgumentParser(description="Proxies HTTP(S) and SMTP requests")
    p.add_argument("--smtp-port", "-m", default=5050, type=int,
                   metavar='<PORT>', help="SMTP port number to listen on.")
    p.add_argument("--http-port", "-p", default=8080, type=int,
                   metavar='<PORT>', help="HTTP port number to listen on.")
    p.add_argument("--ini-file", "-f", metavar='<FILENAME>', required=True,
                   help="config file name")
    p.add_argument('--pid-file', help="pid file path")
    args = p.parse_args()


def initialize():
    parse_args()

    vulcan.initialize(path.join(path.dirname(__file__), args.ini_file))
    # Create the pidfile:
    if args.pid_file:
        with open(pid_file, 'w') as pidfile:
            pidfile.write(str(getpid()))

    # Change the name of the process to "vulcan"
    setproctitle.setproctitle("vulcan")


def main():
    # pick epoll()-based twisted reactor. this needs to appear before
    # any other Twisted imports
    epollreactor.install()

    from twisted.internet import reactor
    from twisted.cred.portal import Portal

    from vulcan import config
    from vulcan.httpserver import HTTPFactory
    from vulcan.smtpserver import SMTPFactory, SimpleRealm, CredentialsChecker
    from vulcan import throttling

    throttling.initialize()
    reactor.listenTCP(args.http_port, HTTPFactory())
    reactor.listenTCP(args.smtp_port, SMTPFactory(
            Portal(SimpleRealm(), [CredentialsChecker()])))
    reactor.suggestThreadPoolSize(vulcan.config.get("numthreads", 10))
    reactor.run()


if __name__ == '__main__':
    initialize()
    main()
