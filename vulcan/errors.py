from twisted.web.error import Error
from twisted.web import http
from twisted.web.http import UNAUTHORIZED

# http://tools.ietf.org/html/rfc6585
TOO_MANY_REQUESTS = 429

RESPONSES = {TOO_MANY_REQUESTS: "Too Many Requests"}
RESPONSES.update(http.RESPONSES)


class RateLimitReached(Error):
    def __init__(self, retry_seconds):
        self.retry_seconds = retry_seconds
        suffix = 's' if self.retry_seconds > 1 else ''
        message = "Rate limit reached. Retry in %s second%s" % (
            self.retry_seconds, suffix)

        Error.__init__(
            self,
            TOO_MANY_REQUESTS,
            message)

    def __eq__(self, other):
        if isinstance(other, self.__class__):
            return self.__dict__ == other.__dict__


class AuthorizationFailed(Error):
    def __init__(self,
                 code=http.UNAUTHORIZED,
                 message=RESPONSES[UNAUTHORIZED],
                 response=None):
        Error.__init__(
            self,
            code,
            message,
            response)


class TimeoutError(Exception):
    """Raised when time expires in timeout decorator"""
