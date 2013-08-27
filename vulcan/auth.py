# -*- test-case-name: vulcan.test.test_auth -*-

"""
Authorization module for reverse proxy.
"""

import json

from twisted.internet import defer

import random
import treq

from vulcan import config
from vulcan.routing import AuthResponse
from vulcan.errors import AuthorizationFailed


@defer.inlineCallbacks
def authorize(request):
    """Authorize request based on the parameters extracted from the request.
    """

    url = random.choice(config["auth"]["urls"])
    r = yield treq.get(url, params=request.to_json())
    content = yield treq.content(r)

    if r.code >= 300 or r.code < 200:
        raise AuthorizationFailed(r.code, r.phrase, content)

    defer.returnValue(AuthResponse.from_json(json.loads(content)))

