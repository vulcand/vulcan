package vulcan

import (
	. "launchpad.net/gocheck"
)

func (s *MainSuite) TestLoggerSuccess(c *C) {
	LogMessage("Hello testing logger %d", 1)
	LogError("Hello %d", 1)
}
