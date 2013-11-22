package ratelimit

import (
	"github.com/mailgun/vulcan/backend"
	"github.com/mailgun/vulcan/command"
	"net/http"
	"text/template"
)

type RateLimiter struct {
	Backend backend.Backend
}

func (rl *RateLimiter) Limit(request *http.Request, rates map[string][]*command.Rate) (retrySeconds int, err error) {
	retrySeconds = 0
	for key, rates := range rates {
		key := replaceKey(key, request)
	}
	return 0, nil
}

func replaceKey(string key, request *httpRequest) {
	tpl := template.New("a")

}
