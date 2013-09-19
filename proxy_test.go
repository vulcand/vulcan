package vulcan

import (
	. "launchpad.net/gocheck"

//	"net/http"
//	"net/http/httptest"
//	"net/url"
)

func (s *MainSuite) TestProxySuccess(c *C) {
	/*
		backend := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hi"))
			}))
		defer backend.Close()
		backendURL, err := url.Parse(backend.URL)
		if err != nil {
			c.Fatal(err)
		}

		control := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hi"))
			}))
		defer control.Close()
		controlURL, err := url.Parse(backend.URL)
		if err != nil {
			c.Fatal(err)
		}
	*/
}
