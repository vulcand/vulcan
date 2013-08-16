from functools import partial
import json

import regex as re

from twisted.web.resource import Resource
from twisted.internet import defer
from twisted.python import log
from twisted.web.http import SERVICE_UNAVAILABLE, OK
from twisted.web.server import NOT_DONE_YET

from expiringdict import ExpiringDict

from vulcan.cassandra import client
from vulcan.utils import safe_format
from vulcan.errors import TimeoutError
from vulcan.errors import RESPONSES


CACHE = ExpiringDict(max_len=100, max_age_seconds=60)


class AdminResource(Resource):
    def getChild(self, path, request):
        return self

    def render_POST(self, request):
        data = json.loads(request.content.getvalue())
        query = safe_format("insert into services (name, path, upstream)"
                            "values('{}', '{}', '{}')",
                            data["name"], data["path"],
                            data["upstream"])
        _run_query(query, request)

        CACHE[data["name"]] = {"path": data["path"],
                               "upstream": data["upstream"]}

        return NOT_DONE_YET

    def render_PUT(self, request):
        service = request.uri.split("/")[-1]
        data = json.loads(request.content.getvalue())
        updates = ", ".join(
            [safe_format("{} = '{}'", k, v) for k, v in data.iteritems()])
        query = safe_format("update services set {} where name = '{}'",
                            updates, service)
        _run_query(query, request)

        cached = CACHE.get(service)
        if cached:
            cached.update(data)
            CACHE[service] = cached

        return NOT_DONE_YET

    def render_DELETE(self, request):
        service = request.uri.split("/")[-1]
        query = safe_format("delete from services where name = '{}'", service)
        _run_query(query, request)

        try:
            del CACHE[service]
        except KeyError:
            pass

        return NOT_DONE_YET


@defer.inlineCallbacks
def _run_query(query, request):
    try:
        yield client.execute_cql3_query(query)
        request.setResponseCode(OK, RESPONSES[OK])
        request.write("")
        request.finish()
    except Exception, e:
        log.err(e)
        request.setResponseCode(SERVICE_UNAVAILABLE,
                                RESPONSES[SERVICE_UNAVAILABLE])
        request.write("")
        request.finish()


@defer.inlineCallbacks
def pick_service(uri):
    service = _pick_service(uri, CACHE)
    if service:
        defer.returnValue(service)

    try:
        r = yield client.execute_cql3_query("select name, path from services")
        for row in r.rows:
            name = row.columns[0].value
            path = row.columns[1].value
            CACHE[name] = path
        s = _pick_service(uri, CACHE)
        defer.returnValue(s)
    except TimeoutError:
        log.err("All Cassandra nodes are down")
        raise


def _pick_service(uri, routes):
    # we use expiringdict for caching
    # iteration over dict won't remove expired values
    # so we access values directly
    for rule in routes:
        if re.match(routes[rule]["path"], uri):
            return rule
