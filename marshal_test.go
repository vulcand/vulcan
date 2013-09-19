package vulcan

import (
	. "launchpad.net/gocheck"
	"net/url"
	"time"
)

func (s *MainSuite) TestUnmarshalSuccess(c *C) {
	objects := []struct {
		Bytes    []byte
		Expected ProxyInstructions
	}{
		{
			Expected: ProxyInstructions{
				Tokens: []*Token{
					&Token{
						Id: "Hello",
						Rates: []*Rate{
							&Rate{
								Period: time.Hour,
								Value:  10000,
							},
						},
					},
				},
				Upstreams: []*Upstream{
					&Upstream{
						Url: &url.URL{
							Scheme: "http",
							Host:   "localhost:5000",
							Path:   "/upstream",
						},
						Headers: map[string][]string{
							"X-Sasha":  []string{"b"},
							"X-Serega": []string{"a"},
						},
						Rates: []*Rate{
							&Rate{Value: 10, Period: time.Minute},
						},
					},
					&Upstream{
						Url: &url.URL{
							Scheme: "http",
							Host:   "localhost:5000",
							Path:   "/upstream2",
						},
						Headers: map[string][]string{
							"X-Sasha":  []string{"b2"},
							"X-Serega": []string{"a2"},
						},
						Rates: []*Rate{
							&Rate{Value: 4, Period: time.Second},
							&Rate{Value: 40000, Period: time.Minute},
						},
					},
				},
			},
			Bytes: []byte(`{
    "tokens": [
        {
            "id": "Hello",
            "rates": [
                {
                    "period": "hour",
                    "value": 10000
                }
            ]
        }
    ],
    "upstreams": [
        {
            "headers": {
                "X-Sasha": [
                    "b"
                ],
                "X-Serega": [
                    "a"
                ]
            },
            "rates": [
                {
                    "period": "minute",
                    "value": 10
                }
            ],
            "url": "http://localhost:5000/upstream"
        },
        {
            "headers": {
                "X-Sasha": [
                    "b2"
                ],
                "X-Serega": [
                    "a2"
                ]
            },
            "rates": [
                {
                    "period": "second",
                    "value": 4
                },
                {
                    "period": "minute",
                    "value": 40000
                }
            ],
            "url": "http://localhost:5000/upstream2"
        }
    ]}`),
		},
	}
	for _, u := range objects {
		authResponse, err := proxyInstructionsFromJson(u.Bytes)
		c.Assert(err, IsNil)
		//we will be checking individual elements here
		//as if something fails would be impossible to debug

		//Check tokens
		c.Assert(len(authResponse.Tokens), Equals, len(u.Expected.Tokens))
		for i, token := range authResponse.Tokens {
			expectedToken := u.Expected.Tokens[i]
			c.Assert(token, DeepEquals, expectedToken)
		}

		//Check upstreams
		c.Assert(len(authResponse.Upstreams), Equals, len(u.Expected.Upstreams))
		for i, upstream := range authResponse.Upstreams {
			expectedUpstream := u.Expected.Upstreams[i]
			c.Assert(*upstream.Url, DeepEquals, *expectedUpstream.Url)
			c.Assert(upstream.Headers, DeepEquals, expectedUpstream.Headers)
			c.Assert(upstream.Rates, DeepEquals, expectedUpstream.Rates)
		}
	}
}

func (s *MainSuite) TestUnmarshalFail(c *C) {
	objects := [][]byte{
		//Empty
		[]byte(""),
		//Good json, bad format
		[]byte(`{}`),
		//bad upstream
		[]byte(`{"upstreams": [{}]}`),
		//bad rates in tokens
		[]byte(`{
    "tokens": [
        {
            "rates": [
                {
                    "period": "year", 
                    "value": -1
                }
            ], 
            "id": "hola"
        }
    ]
}`),
		//bad rates in upstreams
		[]byte(`{
    "upstreams": [
        {
            "rates": [
                {
                    "period": "super-minute",
                    "value": 10
                }
            ],
            "url": "http://localhost:5000/upstream"
        }
    ]
}`),
	}
	for _, bytes := range objects {
		_, err := proxyInstructionsFromJson(bytes)
		c.Assert(err, NotNil)
	}
}
