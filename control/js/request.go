package js

import (
	"github.com/mailgun/vulcan/netutils"
	"net/http"
)

func requestToJs(r *http.Request) (map[string]interface{}, error) {
	auth, err := netutils.ParseAuthHeader(r.Header.Get("Authorization"))
	if err != nil {
		auth = &netutils.BasicAuth{}
	}

	return map[string]interface{}{
		"auth": map[string]interface{}{
			"username": auth.Username,
			"password": auth.Password,
		},
		"query":    r.URL.Query(),
		"protocol": r.Proto,
		"method":   r.Method,
		"url":      r.RequestURI,
		"length":   r.ContentLength,
		"headers":  r.Header,
	}, nil
}
