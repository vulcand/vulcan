# -*- coding: utf-8 -*-

from twisted.trial.unittest import TestCase
from vulcan.utils import safe_format


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
