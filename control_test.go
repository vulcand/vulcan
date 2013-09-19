package vulcan

import (
	. "launchpad.net/gocheck"
	"net/http"
)

func (s *MainSuite) TestFromHttpSuccess(c *C) {
	requests := []struct {
		In  http.Request
		Out ControlRequest
	}{
		{
			http.Request{
				Method: "GET",
				Header: map[string][]string{
					"Authorization": []string{"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="},
				}},
			ControlRequest{Method: "GET"},
		},
	}
	for _, r := range requests {
		_, err := controlRequestFromHttp(&r.In)
		c.Assert(err, IsNil)
	}
}

func (s *MainSuite) TestFromHttpFail(c *C) {
	requests := []http.Request{
		http.Request{
			Method: "GET",
			Header: map[string][]string{
				"Authorization": []string{"Broken auth"},
			}},
	}
	for _, r := range requests {
		_, err := controlRequestFromHttp(&r)
		c.Assert(err, NotNil)
	}
}
