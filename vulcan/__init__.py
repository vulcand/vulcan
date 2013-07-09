import logging
import logging.config
import ConfigParser

from vulcan.utils import load_config


def initialize(ini_file):
    try:
        logging.config.fileConfig(ini_file, disable_existing_loggers=False)
    except ConfigParser.NoSectionError:
        logging.basicConfig()
    global log, config
    log = logging.getLogger(__name__)
    config = load_config(ini_file)
