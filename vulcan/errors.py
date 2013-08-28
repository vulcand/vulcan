from twisted.web.error import Error
from twisted.web import http

from vulcan.utils import safe_format


class RateLimitReached(Exception):
    def __init__(self, retry_seconds):
        self.retry_seconds = retry_seconds
        description = safe_format(
            "Rate limit reached. Retry in {} second{}",
            str(retry_seconds),
            's' if retry_seconds > 1 else '')

        Exception.__init__(self, description)

    def __eq__(self, other):
        if isinstance(other, self.__class__):
            return self.__dict__ == other.__dict__


class AuthorizationFailed(Error):
    pass

class TimeoutError(Exception):
    """Raised when time expires in timeout decorator"""


# http://tools.ietf.org/html/rfc6585
TOO_MANY_REQUESTS = 429

RESPONSES = {TOO_MANY_REQUESTS: "Too Many Requests"}
RESPONSES.update(http.RESPONSES)
