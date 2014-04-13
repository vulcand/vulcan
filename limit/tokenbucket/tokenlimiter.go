package tokenbucket

import (
	"fmt"
	"github.com/mailgun/gotools-time"
	"github.com/mailgun/ttlmap"
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
	"time"
)

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

func (tl *TokenLimiter) Before(r Request) (*http.Response, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	token, amount, err := tl.settings.Mapper(r)
	if err != nil {
		return nil, err
	}

	bucketI, exists := tl.buckets.Get(token)
	if !exists {
		bucketI, err = NewTokenBucket(tl.settings.Rate, tl.settings.MaxTokens, tl.settings.TimeProvider)
		if err != nil {
			return nil, err
		}
		// We set ttl as 10 times rate period. E.g. if rate is 100 requests/second per client ip
		// the counters for this ip will expire after 10 seconds of inactivity
		tl.buckets.Set(token, bucketI, int(tl.settings.Rate.Period/time.Second)*10+1)
	}
	bucket := bucketI.(*TokenBucket)
	delay, err := bucket.Consume(amount)
	if err != nil {
		return nil, err
	}
	if delay > 0 {
		return nil, fmt.Errorf("Rate limit reached")
	}
	return nil, nil
}

func (tl *TokenLimiter) After(r Request) error {
	return nil
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
