package command

import (
	"encoding/json"
	. "launchpad.net/gocheck"
)

type UpstreamSuite struct{}

var _ = Suite(&UpstreamSuite{})

func (s *UpstreamSuite) TestNewUpstream(c *C) {
	u, err := NewUpstream(
		"http",
		"localhost",
		5000)
	c.Assert(err, IsNil)
	expected := Upstream{
		Id:     "http://localhost:5000",
		Scheme: "http",
		Host:   "localhost",
		Port:   5000,
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
				Id:     "http://google.com:5000",
				Scheme: "http",
				Host:   "google.com",
				Port:   5000,
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
			Parse: `{"scheme": "https", "host": "localhost", "port": 4000}`,
			Expected: Upstream{
				Id:     "https://localhost:4000",
				Scheme: "https",
				Host:   "localhost",
				Port:   4000,
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
