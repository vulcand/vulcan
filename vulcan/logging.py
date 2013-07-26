from twisted.python import log
from twisted.python.log import FileLogObserver, _safeFormat, textFromEventDict
from twisted.python import util


ERROR = "ERROR"
INFO = "INFO"


class CustomizableFileLogObserver(FileLogObserver):
    timeFormat = None

    def __init__(self, f, fmt="[%(system)s] [%(level)s] %(text)s\n"):
        self.write = f.write
        self.flush = f.flush
        self.fmt = fmt

    def emit(self, eventDict):
        text = textFromEventDict(eventDict)
        if text is None:
            return

        timeStr = self.formatTime(eventDict['time'])
        level = eventDict.get("logLevel",
                              ERROR if eventDict["isError"] else INFO)
        fmtDict = {'system': eventDict['system'],
                   'text': text.replace("\n", "\n\t"),
                   'level': level}
        msgStr = _safeFormat(self.fmt, fmtDict)

        util.untilConcludes(self.write, timeStr + " " + msgStr)
        util.untilConcludes(self.flush)  # Hoorj!
