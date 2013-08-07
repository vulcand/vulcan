# -*- coding: utf-8 -*-

from . import *

import gc
from StringIO import StringIO

from twisted.trial.unittest import SynchronousTestCase
from twisted.python import log
from twisted.python.failure import Failure

from vulcan.logging import CustomizableFileLogObserver


class LoggingTest(SynchronousTestCase):
    def setUp(self):
        self.output = StringIO()
        self.observer = CustomizableFileLogObserver(self.output)
        self.observer.start()

    def tearDown(self):
        self.observer.stop()

    def test_empty_msg(self):
        log.msg()
        self.assertEquals("", self.output.getvalue())

    def test_msg(self):
        log.msg("message")
        self.assertIn("[-] [INFO] message", self.output.getvalue())

    def test_msg_logLevel(self):
        log.msg("message", system="vulcan", logLevel="DEBUG")
        self.assertIn("[vulcan] [DEBUG] message", self.output.getvalue())

    def test_msg_unknown_param(self):
        log.msg("message", unknown_param="value")
        self.assertIn("[-] [INFO] message", self.output.getvalue())

    def test_msg_custom_format(self):
        self.observer.stop()
        self.observer = CustomizableFileLogObserver(
            self.output,
            fmt="[%(system)s][%(threadname)s] [%(logLevel)s] %(text)s\n")
        self.observer.start()

        log.msg("message", threadname="vulcan-0", system="vulcan")
        self.assertIn(
            "[vulcan][vulcan-0] [INFO] message", self.output.getvalue())

    def test_err(self):
        log.err(Failure(MyException("Bam!")))
        self.assertIn("[-] [ERROR]", self.output.getvalue())
        self.assertIn("MyException: Bam!", self.output.getvalue())
        gc.collect()
        self.flushLoggedErrors(MyException)

    def test_err_logLevel(self):
        log.err(Failure(MyException("Bam!")), logLevel="CRITICAL")
        self.assertIn("[-] [CRITICAL]", self.output.getvalue())
        self.assertIn("MyException: Bam!", self.output.getvalue())
        gc.collect()
        self.flushLoggedErrors(MyException)

    def test_err_unknown_param(self):
        log.err(Failure(MyException("Bam!")), unknown_param="value")
        self.assertIn("[-] [ERROR]", self.output.getvalue())
        self.assertIn("MyException: Bam!", self.output.getvalue())
        gc.collect()
        self.flushLoggedErrors(MyException)

    def test_err_custom_format(self):
        self.observer.stop()
        self.observer = CustomizableFileLogObserver(
            self.output,
            fmt="[%(system)s][%(threadname)s] [%(logLevel)s] %(text)s\n")
        self.observer.start()

        log.err(Failure(MyException("Bam!")),
                threadname="vulcan-0", system="vulcan")
        self.assertIn("[vulcan][vulcan-0] [ERROR]", self.output.getvalue())
        self.assertIn("MyException: Bam!", self.output.getvalue())
        gc.collect()
        self.flushLoggedErrors(MyException)


class MyException(Exception):
    pass
