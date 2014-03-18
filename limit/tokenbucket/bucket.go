package tokenbucket

import (
	"fmt"
	"github.com/mailgun/gotools-time"
	"time"
)

type Rate struct {
	Tokens int64
	Period time.Duration
}

// Implements token bucket rate limiting algorithm (http://en.wikipedia.org/wiki/Token_bucket)
// and is used by rate limiters to implement various rate limiting strategies
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
		refillPeriod: time.Duration(int64(rate.Period) / rate.Tokens),
		maxTokens:    maxBurst,
		timeProvider: timeProvider,
		lastRefill:   timeProvider.UtcNow(),
		tokens:       maxBurst,
	}, nil
}

// In case if there's enough tokens, consumes tokens and returns 0, nil
// In case if tokens to consume is larger than max burst returns -1, error
// In case if there's not enough tokens, returns time to wait till refill
func (tb *TokenBucket) Consume(tokens int64) (time.Duration, error) {
	tb.refill()
	if tokens > tb.maxTokens {
		return -1, fmt.Errorf("Requested tokens larger than max tokens")
	}
	if tb.tokens < tokens {
		return tb.timeToRefill(tokens), nil
	}
	tb.tokens -= tokens
	return 0, nil
}

// Returns the time after the capacity of tokens will reach the
func (tb *TokenBucket) timeToRefill(tokens int64) time.Duration {
	missingTokens := tokens - tb.tokens
	return time.Duration(missingTokens) * tb.refillPeriod
}

func (tb *TokenBucket) refill() {
	now := tb.timeProvider.UtcNow()
	timePassed := now.Sub(tb.lastRefill)
	tb.tokens = tb.tokens + int64(timePassed/tb.refillPeriod)
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}
