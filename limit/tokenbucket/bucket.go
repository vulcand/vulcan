package tokenbucket

import (
	"fmt"
	"github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"time"
)

type Rate struct {
	Requests int64
	Period   time.Duration
}

type TokenBucket struct {
	// Maximum amount of tokens available at given time (controls burst rate)
	maxTokens int64
	// Specifies the period of the rate
	refillPeriod time.Duration
	// Current value of tokens
	tokens int64
	// Interface that gives current time (so tests can override)
	timeProvider timetools.TimeProvider
	lastRefill   time.Time
}

func NewTokenBucket(rate Rate, maxBurst int64, timeProvider timetools.TimeProvider) (*TokenBucket, error) {
	return &TokenBucket{
		refillPeriod: time.Duration(int64(rate.Period) / rate.Requests),
		maxTokens:    maxBurst,
		timeProvider: timeProvider,
		lastRefill:   timeProvider.UtcNow(),
		tokens:       maxBurst,
	}, nil
}

func (tb *TokenBucket) Limit(r Request) (time.Duration, error) {
	tb.refill()
	if tb.tokens < 1 {
		return 0, fmt.Errorf("Rate limit reached")
	}
	tb.tokens -= 1
	return 0, nil
}

func (tb *TokenBucket) refill() {
	now := tb.timeProvider.UtcNow()
	timePassed := now.Sub(tb.lastRefill)
	newTokens := tb.tokens + int64(timePassed/tb.refillPeriod)
	if newTokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	} else {
		tb.tokens = newTokens
	}
	tb.lastRefill = now
}

func (tb *TokenBucket) Before(r Request) (*http.Response, error) {
	return nil, nil
}

func (tb *TokenBucket) After(r Request, response *http.Response, err error) error {
	return nil
}
