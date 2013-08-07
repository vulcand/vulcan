# -*- coding: utf-8 -*-

from . import *

import os
from ConfigParser import ConfigParser

from twisted.trial.unittest import TestCase
from twisted.python import log

from vulcan.utils import safe_format, load_config, is_valid_ip


class UtilsTest(TestCase):
    @patch.object(log, 'err')
    def test_safe_format(self, log_err):
        self.assertEquals("one один", safe_format("one {}", u"один"))
        self.assertEquals("один один", safe_format(u"один {}", "один"))
        self.assertEquals("hey один", safe_format("hey {}", "один"))
        self.assertEquals("один один", safe_format(u"один {}", "один"))
        self.assertEquals("один один", safe_format(u"один {one}", one="один"))
        self.assertEquals("hey один", safe_format("hey один"))
        # KeyError: 'one'
        self.assertEquals(u"", safe_format("one {one}", two="два"))
        self.assertTrue(log_err.called)

    @patch.object(ConfigParser, 'read', Mock(return_value=[]))
    @patch.object(os.path, 'exists', Mock(return_value=False))
    def test_config_missing(self):
        self.assertRaises(Exception, load_config, "non-existent.ini")

    def test_ip_validation(self):
        self.assertTrue(is_valid_ip("127.0.0.1"))
        self.assertFalse(is_valid_ip("z27.0.0.1"))
        self.assertFalse(is_valid_ip(None))
        self.assertFalse(is_valid_ip("hello"))
