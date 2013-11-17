package command

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"net/http"
	"time"
)

type CommandsSuite struct{}

var _ = Suite(&CommandsSuite{})

func (s *CommandsSuite) TestCommandsFromObj(c *C) {
	commands := []struct {
		Expected interface{}
		Parse    string
	}{
		{
			Parse: `{"code": 500, "message": "access denied"}`,
			Expected: &Reply{
				Code:    500,
				Message: "access denied",
			},
		},
		{
			Parse: `{"code": 405, "message": {"error": "some error"}}`,
			Expected: &Reply{
				Code:    405,
				Message: map[string]interface{}{"error": "some error"},
			},
		},
		{
			Parse: `{"upstreams": ["http://localhost:5000", "http://localhost:5001"]}`,
			Expected: &Forward{
				Upstreams: []*Upstream{
					&Upstream{
						Id:     "http://localhost:5000",
						Scheme: "http",
						Port:   5000,
						Host:   "localhost",
					},
					&Upstream{
						Id:     "http://localhost:5001",
						Scheme: "http",
						Port:   5001,
						Host:   "localhost",
					},
				},
			},
		},
		{
			Parse: `{"rates": {"$request.ip": "1 req/second"}, "upstreams": ["http://localhost:5000", "http://localhost:5001"]}`,
			Expected: &Forward{
				Rates: map[string][]*Rate{
					"$request.ip": []*Rate{&Rate{Units: 1, Period: time.Second}},
				},
				Upstreams: []*Upstream{
					&Upstream{
						Id:     "http://localhost:5000",
						Scheme: "http",
						Port:   5000,
						Host:   "localhost",
					},
					&Upstream{
						Id:     "http://localhost:5001",
						Scheme: "http",
						Port:   5001,
						Host:   "localhost",
					},
				},
			},
		},
		{
			Parse: `{
                  "failover": {"active": true, "codes": [301, 302]},
                  "rates": {
                     "$request.ip": [
                         "1 req/second",
                         {"MB": 8, "period": "hour"}
                  ]},
                  "upstreams": [
                       "http://localhost:5000/rewrite-path",
                        {
                           "scheme": "http",
                           "host": "localhost",
                           "port": 5001,
                           "rewrite-path": "/p2",
                           "add-headers": {"A": "b"},
                           "remove-headers": {"B": "c"}
                        }
                  ],
                "add-headers": {"N": "v1"},
                "remove-headers": {"M": "v2"}
            }`,
			Expected: &Forward{
				Failover: &Failover{Active: true, Codes: []int{301, 302}},
				Rates: map[string][]*Rate{
					"$request.ip": []*Rate{
						&Rate{Units: 1, Period: time.Second},
						&Rate{Units: 8, UnitType: UnitTypeMegabytes, Period: time.Hour},
					},
				},
				AddHeaders:    http.Header{"N": []string{"v1"}},
				RemoveHeaders: http.Header{"M": []string{"v2"}},
				Upstreams: []*Upstream{
					&Upstream{
						Id:          "http://localhost:5000",
						Scheme:      "http",
						Port:        5000,
						Host:        "localhost",
						RewritePath: "/rewrite-path",
					},
					&Upstream{
						Id:            "http://localhost:5001",
						Scheme:        "http",
						Port:          5001,
						Host:          "localhost",
						RewritePath:   "/p2",
						AddHeaders:    http.Header{"A": []string{"b"}},
						RemoveHeaders: http.Header{"B": []string{"c"}},
					},
				},
			},
		},
	}

	for _, cmd := range commands {
		var value interface{}
		err := json.Unmarshal([]byte(cmd.Parse), &value)
		c.Assert(err, IsNil)
		parsed, err := NewCommandFromObj(value)
		c.Assert(err, IsNil)
		c.Assert(parsed, DeepEquals, cmd.Expected)
	}
}
