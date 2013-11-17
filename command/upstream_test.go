package command

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"net/http"
)

type UpstreamSuite struct{}

var _ = Suite(&UpstreamSuite{})

func (s *UpstreamSuite) TestNewUpstream(c *C) {
	u, err := NewUpstream(
		"http",
		"localhost",
		5000, "/new/path",
		http.Header{"A": []string{"b"}},
		http.Header{"B": []string{"c"}})
	c.Assert(err, IsNil)
	expected := Upstream{
		Id:            "http://localhost:5000",
		Scheme:        "http",
		Host:          "localhost",
		Port:          5000,
		RewritePath:   "/new/path",
		AddHeaders:    http.Header{"A": []string{"b"}},
		RemoveHeaders: http.Header{"B": []string{"c"}},
	}
	c.Assert(*u, DeepEquals, expected)
}

func (s *UpstreamSuite) TestUpstreamFromObj(c *C) {
	upstreams := []struct {
		Expected Upstream
		Parse    string
	}{
		{
			Parse: `"http://google.com:5000"`,
			Expected: Upstream{
				Id:     "http://google.com:5000",
				Scheme: "http",
				Host:   "google.com",
				Port:   5000,
			},
		},
		{
			Parse: `"http://google.com:5000/"`,
			Expected: Upstream{
				Id:          "http://google.com:5000",
				Scheme:      "http",
				Host:        "google.com",
				Port:        5000,
				RewritePath: "/",
			},
		},
		{
			Parse: `"https://google.com:5000/a/b"`,
			Expected: Upstream{
				Id:          "https://google.com:5000",
				Scheme:      "https",
				Host:        "google.com",
				Port:        5000,
				RewritePath: "/a/b",
			},
		},
		{
			Parse: `{"scheme": "http", "host": "localhost", "port": 3000}`,
			Expected: Upstream{
				Id:     "http://localhost:3000",
				Scheme: "http",
				Host:   "localhost",
				Port:   3000,
			},
		},
		{
			Parse: `{"scheme": "https", "host": "localhost", "port": 4000, "rewrite-path": "/new/path", "add-headers": {"a": "b"}, "remove-headers": {"c": ["d", "f"]}}`,
			Expected: Upstream{
				Id:            "https://localhost:4000",
				Scheme:        "https",
				Host:          "localhost",
				Port:          4000,
				RewritePath:   "/new/path",
				AddHeaders:    http.Header{"A": []string{"b"}},
				RemoveHeaders: http.Header{"C": []string{"d", "f"}},
			},
		},
	}

	for _, u := range upstreams {
		var value interface{}
		err := json.Unmarshal([]byte(u.Parse), &value)
		c.Assert(err, IsNil)
		parsed, err := NewUpstreamFromObj(value)
		c.Assert(err, IsNil)
		c.Assert(u.Expected, DeepEquals, *parsed)
	}
}
