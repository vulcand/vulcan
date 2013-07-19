# -*- coding: utf-8 -*-

from . import *

from twisted.trial.unittest import TestCase

from vulcan import upstream
from vulcan.upstream import pick_server


class UpstreamTest(TestCase):
    def test_pick_server(self):
        self.assertIn(pick_server('10.241.0.25:3001,10.241.0.26:3002'),
                      ['10.241.0.25:3001', '10.241.0.26:3002'])
        with patch.object(upstream, 'is_active', Mock(return_value=False)):
            self.assertIsNone(pick_server('10.241.0.25:3001'))
