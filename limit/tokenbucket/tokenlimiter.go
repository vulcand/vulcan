package tokenbucket

import (
	"fmt"
	"github.com/mailgun/gotools-time"
	"github.com/mailgun/ttlmap"
	. "github.com/mailgun/vulcan/request"
	"strings"
	"sync"
	"time"
)

// Mapper function takes the request and returns token that corresponds to this request
// and the amount of tokens this request is going to consume, e.g.
// * Client ip rate limiter - token is a client ip, amount is 1 request
// * Client ip memory limiter - token is a client ip, amount is number of bytes to consume
// In case of error returns non nil error, in this case rate limiter will reject the request.
type MapperFn func(r Request) (token string, amount int, err error)

type TokenLimiter struct {
	buckets  *ttlmap.TtlMap
	mutex    *sync.Mutex
	settings Settings
}

type Settings struct {
	Rate         Rate // Average allowed rate
	MaxTokens    int  // Maximum tokens (controls burst size)
	Capacity     int  // Overall capacity (maximum sumultaneuously active tokens)
	Mapper       MapperFn
	TimeProvider timetools.TimeProvider
	Delay        bool // Whether to delay requests
}

// Rate limits requests based on client ip
func NewClientIpLimiter(s Settings) (*TokenLimiter, error) {
	s.Mapper = MapClientIp
	return NewTokenLimiter(s)
}

func NewTokenLimiter(s Settings) (*TokenLimiter, error) {
	settings, err := parseSettings(s)
	if err != nil {
		return nil, err
	}
	buckets, err := ttlmap.NewMapWithProvider(settings.Capacity, settings.TimeProvider)
	if err != nil {
		return nil, err
	}

	return &TokenLimiter{
		settings: settings,
		mutex:    &sync.Mutex{},
		buckets:  buckets,
	}, nil
}

// Existing implementation does not support delays and either rejects request right away or allows it to proceed
func (tl *TokenLimiter) Limit(r Request) (time.Duration, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	token, amount, err := tl.settings.Mapper(r)
	if err != nil {
		return -1, err
	}

	bucketI, exists := tl.buckets.Get(token)
	if !exists {
		bucketI, err = NewTokenBucket(tl.settings.Rate, tl.settings.MaxTokens, tl.settings.TimeProvider)
		if err != nil {
			return -1, err
		}
		// We set ttl as 10 times rate period. E.g. if rate is 100 requests/second per client ip
		// the counters for this ip will expire after 10 seconds of inactivity
		tl.buckets.Set(token, bucketI, int(tl.settings.Rate.Period/time.Second)*10+1)
	}
	bucket := bucketI.(*TokenBucket)
	delay, err := bucket.Consume(amount)
	if err != nil {
		return delay, err
	}
	if delay > 0 {
		return -1, fmt.Errorf("Rate limit reached")
	}
	return 0, nil
}

// Check arguments and initialize defaults
func parseSettings(s Settings) (Settings, error) {
	if s.MaxTokens <= 0 {
		s.MaxTokens = 1
	}
	if s.Capacity <= 0 {
		s.Capacity = DefaultCapacity
	}
	if s.TimeProvider == nil {
		s.TimeProvider = &timetools.RealTime{}
	}
	if s.Mapper == nil {
		return s, fmt.Errorf("Provide mapper function")
	}
	return s, nil
}

const DefaultCapacity = 32768

// This function maps the request to it's client ip. Rate limiter using this mapper
// function will do rate limiting based on the client ip.
func MapClientIp(req Request) (string, int, error) {
	vals := strings.SplitN(req.GetHttpRequest().RemoteAddr, ":", 2)
	if len(vals[0]) == 0 {
		return "", -1, fmt.Errorf("Failed to parse client ip")
	}
	return vals[0], 1, nil
}
