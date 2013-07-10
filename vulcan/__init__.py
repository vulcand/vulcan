from twisted.python import log

from vulcan.utils import load_config


def initialize(ini_file):
    global config
    config = load_config(ini_file)
