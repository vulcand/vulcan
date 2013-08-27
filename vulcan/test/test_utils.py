# -*- coding: utf-8 -*-

from . import *

from twisted.trial.unittest import TestCase
from vulcan.utils import safe_format, is_valid_ip


class UtilsTest(TestCase):
    def test_safe_format(self):
        self.assertEquals("one один", safe_format("one {}", u"один"))
        self.assertEquals("один один", safe_format(u"один {}", "один"))
        self.assertEquals("hey один", safe_format("hey {}", "один"))
        self.assertEquals("один один", safe_format(u"один {}", "один"))
        self.assertEquals("один один", safe_format(u"один {one}", one="один"))
        self.assertEquals("hey один", safe_format("hey один"))
        # KeyError: 'one'
        self.assertEquals(u"", safe_format("one {one}", two="два"))

    def test_ip_validation(self):
        self.assertTrue(is_valid_ip("127.0.0.1"))
        self.assertFalse(is_valid_ip("z27.0.0.1"))
        self.assertFalse(is_valid_ip(None))
        self.assertFalse(is_valid_ip("hello"))
