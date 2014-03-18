package tokenbucket

import (
	"github.com/mailgun/gotools-time"
	"github.com/mailgun/ttlmap"
	. "github.com/mailgun/vulcan/request"
	"sync"
	"time"
)

// Mapper function takes the request and returns token that corresponds to this request
// and the amount of tokens this request is going to consume, e.g.
// * Client ip rate limiter - token is a client ip, amount is 1 request
// * Client ip memory limiter - token is a client ip, amount is number of bytes to consume
// In case of error returns non nil error, in this case rate limiter will reject the request.
type MapperFn func(r Request) (token string, amount int64, err error)

type TokenLimiter struct {
	tokens    *ttlmap.TtlMap
	mutex     *sync.Mutex
	mapper    MapperFn
	rate      Rate
	maxTokens int64
	capacity  int64
}

type TokenLimiterSettings struct {
	Rate         Rate  // Average allowed rate
	MaxTokens    int64 // Maximum burst size in tokens
	Capacity     int64 // Overall capacity
	Mapper       MapperFn
	TimeProvider timetools.TimeProvider
}

func NewTokenLimiter(s TokenLimiterSettings) (*TokenLimiter, error) {
	return nil, nil
}

func (tl *TokenLimiter) Limit(r Request) (time.Duration, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()
	return 0, nil
}
