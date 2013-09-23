package vulcan

import (
	"fmt"
	. "launchpad.net/gocheck"
	"net/http"
	"net/url"
	"time"
)

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func (s *MainSuite) TestParseBadHeaders(c *C) {
	headers := []string{
		//just empty string
		"",
		//missing auth type
		"justplainstring",
		//unknown auth type
		"Whut justplainstring",
		//invalid base64
		"Basic Shmasic",
		//random encoded string
		"Basic YW55IGNhcm5hbCBwbGVhcw==",
	}
	for _, h := range headers {
		_, err := parseAuthHeader(h)
		c.Assert(err, NotNil)
	}
}

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func (s *MainSuite) TestParseSuccess(c *C) {
	headers := []struct {
		Header   string
		Expected BasicAuth
	}{
		{
			"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			BasicAuth{Username: "Aladdin", Password: "open sesame"},
		},
		//empty pass
		{
			"Basic QWxhZGRpbjo=",
			BasicAuth{Username: "Aladdin", Password: ""},
		},
	}
	for _, h := range headers {
		request, err := parseAuthHeader(h.Header)
		c.Assert(err, IsNil)
		c.Assert(request.Username, Equals, h.Expected.Username)
		c.Assert(request.Password, Equals, h.Expected.Password)

	}
}

// We should panic with wrong args
func (s *MainSuite) TestRandomRangeFail(c *C) {
	c.Assert(func() { randomRange(0, 0) }, PanicMatches, `Invalid range .*`)
}

// Just make sure we don't panic on good args
func (s *MainSuite) TestRandomSuccess(c *C) {
	randomRange(0, 1)
	randomRange(2, 4)
}

// Make sure copy does it right, so the copied url
// is safe to alter without modifying the other
func (s *MainSuite) TestCopyUrl(c *C) {
	urlA := &url.URL{
		Scheme:   "http",
		Host:     "localhost:5000",
		Path:     "/upstream",
		Opaque:   "opaque",
		RawQuery: "a=1&b=2",
		Fragment: "#hello",
		User:     &url.Userinfo{},
	}
	urlB := copyUrl(urlA)
	c.Assert(urlB, DeepEquals, urlB)
	urlB.Scheme = "https"
	c.Assert(urlB, Not(DeepEquals), urlA)
}

// Make sure parseUrl is strict enough not to accept total garbage
func (s *MainSuite) TestParseBadUrl(c *C) {
	badUrls := []string{
		"",
		" some random text ",
		"http---{}{\\bad bad url",
	}
	for _, badUrl := range badUrls {
		_, err := parseUrl(badUrl)
		c.Assert(err, NotNil)
	}
}

func (s *MainSuite) TestGetHit(c *C) {
	hits := []struct {
		Key      string
		Rate     *Rate
		Expected string
	}{
		{
			Key:      "key1",
			Rate:     &Rate{Value: 1, Period: time.Second},
			Expected: "key1_1s_%d",
		},
		{
			Key:      "key2",
			Rate:     &Rate{Value: 10, Period: time.Minute},
			Expected: "key2_1m0s_%d",
		},
		{
			Key:      "key1",
			Rate:     &Rate{Value: 2, Period: time.Hour},
			Expected: "key1_1h0m0s_%d",
		},
	}
	for _, u := range hits {
		expected := fmt.Sprintf(u.Expected, u.Rate.currentBucket(s.timeProvider.utcNow()).Unix())
		hit := getHit(s.timeProvider.utcNow(), u.Key, u.Rate)
		c.Assert(expected, Equals, hit)
	}
}

func (s *MainSuite) TestTimes(c *C) {
	tm := &RealTime{}
	c.Assert(tm.utcNow(), NotNil)
}

// Make sure copy headers is not shallow and copies all headers
func (s *MainSuite) TestCopyHeaders(c *C) {
	source, destination := make(http.Header), make(http.Header)
	source.Add("a", "b")
	source.Add("c", "d")

	copyHeaders(destination, source)

	c.Assert(destination.Get("a"), Equals, "b")
	c.Assert(destination.Get("c"), Equals, "d")

	// make sure that altering source does not affect the destination
	source.Del("a")
	c.Assert(source.Get("a"), Equals, "")
	c.Assert(destination.Get("a"), Equals, "b")
}

func (s *MainSuite) TestHasHeaders(c *C) {
	source := make(http.Header)
	source.Add("a", "b")
	source.Add("c", "d")
	c.Assert(hasHeaders([]string{"a", "f"}, source), Equals, true)
	c.Assert(hasHeaders([]string{"i", "j"}, source), Equals, false)
}

func (s *MainSuite) TestRemoveHeaders(c *C) {
	source := make(http.Header)
	source.Add("a", "b")
	source.Add("a", "m")
	source.Add("c", "d")
	removeHeaders([]string{"a"}, source)
	c.Assert(source.Get("a"), Equals, "")
	c.Assert(source.Get("c"), Equals, "d")
}
