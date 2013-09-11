from os import getpid

import argparse
import yaml

import setproctitle

from twisted.internet import reactor

import vulcan


def parse_args():
    p = argparse.ArgumentParser(
        description="Proxies HTTP(S) and SMTP requests")

    p.add_argument("--http-port", "-p", default=8080, type=int,
                   metavar='<PORT>', help="HTTP port number to listen on.")
    p.add_argument("--config", "-c", metavar='<FILENAME>', required=True,
                   help="config file name")
    p.add_argument('--pid-file', help="pid file path")

    return p.parse_args()


def load_config(path):
    with open(path) as f:
        return yaml.load(f)


def initialize(args, process_name="vulcan"):

    params = load_config(args.config)

    vulcan.initialize(params)

    # Create the pidfile:
    if args.pid_file:
        with open(args.pid_file, 'w') as pidfile:
            pidfile.write(str(getpid()))

    # Change the name of the process to "vulcan"
    setproctitle.setproctitle(process_name)

    from vulcan.proxy import HTTPFactory
    from vulcan import cassandra

    cassandra.pool.startService()
    reactor.listenTCP(args.http_port, HTTPFactory())
    reactor.suggestThreadPoolSize(params['twisted']['thread_pool_size'])



def main():
    args = parse_args()
    initialize(args)
    reactor.run()


if __name__ == '__main__':
    main()
