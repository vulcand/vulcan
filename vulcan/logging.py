# -*- test-case-name: vulcan.test.test_logging -*-

from twisted.python import log
from twisted.python.log import FileLogObserver, _safeFormat, textFromEventDict
from twisted.python import util


ERROR = "ERROR"
INFO = "INFO"


class CustomizableFileLogObserver(FileLogObserver):
    timeFormat = None

    def __init__(self, f, fmt="[%(system)s] [%(logLevel)s] %(text)s\n"):
        self.write = f.write
        self.flush = f.flush
        self.fmt = fmt

    def emit(self, eventDict):
        text = textFromEventDict(eventDict)
        if text is None:
            return

        timeStr = self.formatTime(eventDict['time'])
        fmtDict = eventDict.copy()
        logLevel = eventDict.get("logLevel",
                              ERROR if eventDict["isError"] else INFO)
        fmtDict["logLevel"] = logLevel
        fmtDict["text"] = text.replace("\n", "\n\t")
        msgStr = _safeFormat(self.fmt, fmtDict)

        util.untilConcludes(self.write, timeStr + " " + msgStr)
        util.untilConcludes(self.flush)  # Hoorj!
