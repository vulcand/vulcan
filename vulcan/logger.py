import sys
import syslog
from twisted.python import log, syslog as twisted_syslog


def initialize(handlers):
    """Initializes twisted loggers from the list of handlers
    """
    for handler in handlers:
        if handler['type'] not in HANDLERS:
            raise ValueError(
                "Unknown logger: %s, supported handlers are %s" % (
                    handler, HANDLERS.keys()))

        HANDLERS[handler['type']](handler)


def init_syslog(handler):
    facility_name = handler.get('facility', 'LOG_USER')
    facility = getattr(syslog, facility_name, None)
    if not facility:
        raise ValueError(
            "Unrecognized syslog facility: %s" % (facility_name, ))

    prefix = handler.get('prefix', 'vulcan')

    redirect_stdout = handler.get('redirect_stdout', True)
    twisted_syslog.startLogging(
        prefix=prefix, facility=facility, setStdout=redirect_stdout)


def init_stdout(handler):
    log.startLogging(sys.stdout)

HANDLERS = {
    'syslog': init_syslog,
    'stdout': init_stdout
}
