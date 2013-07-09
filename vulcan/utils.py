from os.path import exists
import logging
from ConfigParser import ConfigParser

import regex as re


log = logging.getLogger(__name__)
RE_IP_ADDRESS = re.compile("^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$")


def load_config(ini_file):
    """Returns a config dictionary initialized from the given ini-file.

    Config section it looks for is DEFAULT.

    >>> conf = load_config('test.ini')
    >>> conf['built_type']
    "test"
    """
    parser = ConfigParser()
    if not parser.read([ini_file]):
        if not exists(ini_file):
            raise Exception("Can't load ini-file; {}".format(ini_file))

    # read name/value options from the ini-file and convert them into dictionary
    file_conf = {k: v for k, v in parser.items("DEFAULT")}

    # also append the full path of the ini-file itself to it, so we'll always
    # know which ini file was used:
    file_conf['ini-file'] = ini_file
    return file_conf


def is_valid_ip(str_ip):
    """Determines if a given IP address is good"""
    return RE_IP_ADDRESS.match(str_ip or '') != None


def to_utf8(str_or_unicode):
    """
    Safely returns a UTF-8 version of a given string
    >>> utils.to_utf8(u'hi')
        'hi'
    """
    if isinstance(str_or_unicode, unicode):
        return str_or_unicode.encode("utf-8", "ignore")
    return str(str_or_unicode)


def safe_format(format_string, *args, **kwargs):
    """Helper: logs any combination of bytestrings/unicode strings without
    raising exceptions"""
    try:
        if not args and not kwargs:
            return format_string
        else:
            return format_string.format(*args, **kwargs)

    # catch encoding errors and transform everything into utf-8 string
    # before logging:
    except (UnicodeEncodeError, UnicodeDecodeError):
        format_string = to_utf8(format_string)
        args = [to_utf8(p) for p in args]
        kwargs = {k: to_utf8(v) for k, v in kwargs.iteritems()}
        return format_string.format(*args, **kwargs)

    # ignore other errors
    except Exception, e:
        log.exception(e)
        return u''
