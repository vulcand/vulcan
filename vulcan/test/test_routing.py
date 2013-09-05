# coding: utf-8

from . import *
from twisted.trial.unittest import TestCase

from vulcan.routing import (AuthRequest, AuthResponse,\
    Upstream, Token, Rate, ProxyHeaders)
from vulcan.utils import to_utf8


class RoutingTest(TestCase):
    def test_token(self):
        self.assertEquals({"id": "abc", "rates": []},
                          Token("abc").to_json())
        self.assertEquals(
            {"id": "abc", "rates": [{"value": 1, "period": "minute"},
                                    {"value": 2, "period": "hour"}]},
            Token("abc", [Rate(1, "minute"),
                          Rate(2, "hour")]).to_json())

    def test_rate(self):
        self.assertEquals({"value": 1, "period": "second"},
                          Rate(1, "second").to_json())

    def test_upstream(self):
        self.assertEquals({
                "url": "http://1.2.3.4:80",
                "rates": [],
                "headers": {}
                },
         Upstream("http://1.2.3.4:80").to_json())

        self.assertEquals(
            {"url": "http://1.2.3.4:80",
             "rates": [{"value": 1, "period": "minute"},
                       {"value": 2, "period": "hour"}],
             "headers": {'X-Mailgun': 'yes'}
             },
            Upstream("http://1.2.3.4:80",
                     [Rate(1, "minute"),
                      Rate(2, "hour")],
                     headers=ProxyHeaders({'X-Mailgun': 'yes'})
                     ).to_json())

    def test_upstream_unicode_headers_from_json(self):
        u = Upstream(
            "http://1.2.3.4:80",
            [Rate(1, "minute"), Rate(2, "hour")],
            headers=ProxyHeaders({'X-Mailgun': u'Юникод'}))

        self.assertEquals({
                'X-Mailgun': to_utf8(u'Юникод')}, u.headers.encoded)


    def test_authresponse(self):
        self.assertEquals(
            {"tokens": [{"id": "abc", "rates": []},
                        {"id": "def", "rates": []}],
             "upstreams": [{"url": "http://1.2.3.4:80", "rates": [], "headers": {}},
                           {"url": "http://1.2.3.4:90", "rates": [], "headers": {}}],
             "headers": {}},
            AuthResponse(
                tokens=[Token("abc"), Token("def")],
                upstreams=[Upstream("http://1.2.3.4:80"),
                           Upstream("http://1.2.3.4:90")]).to_json())

    def test_authrequest(self):
        self.assertEquals(
            AuthRequest(username="user",
                        password="secret",
                        protocol="http",
                        method="get",
                        url="url",
                        length=20,
                        ip="1.2.3.4"),
            AuthRequest.from_json(
                dict(username="user",
                     password="secret",
                     protocol="http",
                     method="get",
                     url="url",
                     length=20,
                     ip="1.2.3.4")))
