# -*- test-case-name: vulcan.test.test_utils -*-

import random
from copy import copy

from os.path import exists
from ConfigParser import ConfigParser

import regex as re


RE_IP_ADDRESS = re.compile("^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$")


def is_valid_ip(str_ip):
    """Determines if a given IP address is good"""
    return RE_IP_ADDRESS.match(str_ip or '') is not None


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
    """
    Helper: formats string with any combination of bytestrings/unicode
    strings without raising exceptions
    """
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
    except:
        return u''


def shuffled(values):
    """Returns shuffled version of the passed list without
    actually touching the original one.
    """
    v = copy(values)
    random.shuffle(v)
    return v
