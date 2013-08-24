import urlparse
import json


class Token(object):
    def __init__(self, id, rates):
        self.id = id
        self.rates = rates

    @classmethod
    def from_json(cls, obj):
        return cls(
            id=obj['id'],
            rates=[Rate.from_json(r) for r in obj.get('rates') or []])


class Rate(object):
    def __init__(self, value, period):
        self.value = value
        self.period = period


    @property
    def period_as_seconds(self):
        return {
            'second': 1,
            'minute': 60,
            'hour': 3600,
            'day': 24 * 3600
        }[self.period]

    @classmethod
    def from_json(cls, obj):
        return cls(obj['value'], obj['period'])


class Upstream(object):
    def __init__(self, url, rates):
        self.url = url
        self.rates = rates

    @property
    def host(self):
        return urlparse(self.url).host

    @property
    def port(self):
        return urlparse(self.url).port

    @classmethod
    def from_json(cls, obj):
        return cls(
            url=obj['url'],
            rates=[Rate.from_json(r) for r in obj.get('rates') or []])


class AuthResponse(object):
    def __init__(self, tokens, upstreams, headers):
        self.tokens = tokens
        self.upstreams = upstreams
        self.headers = headers

    @classmethod
    def from_json(cls, obj):
        tokens = [Token.from_json(t) for t in obj['tokens']]
        upstreams = [Upstream.from_json(u) for u in obj['upstreams']]
        headers = obj.get('headers') or []
        return cls(tokens, upstreams, headers)


class AuthRequest(object):
    def __init__(self, username, password, protocol, method, url, length, ip):
        self.username = username
        self.password = password
        self.protocol = protocol
        self.method = method
        self.url = url
        self.length = length
        self.ip = ip

    def to_json(self):
        return {
            'username': self.username,
            'password': self.password,
            'protocol': self.protocol,
            'method': self.method,
            'url': self.url,
            'length': self.length,
            'ip': self.ip
            }

    def __str__(self):
        return json.dumps(self.to_json())

    @classmethod
    def from_http_request(cls, request):
        return cls(
            username=request.getUser(),
            password=request.getPassword(),
            protocol=request.clientproto,
            method=request.method,
            uri=request.uri,
            length=request.getHeader("Content-Length") or 0,
            ip=request.getHeader(IP_HEADER))

IP_HEADER = "X-Real-IP"

