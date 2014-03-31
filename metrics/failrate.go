package metrics

import (
	"fmt"
	timetools "github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/callback"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"time"
)

type FailRateGetter interface {
	GetRate() float64
	Before
	After
}

// Predicate that helps to see if the attempt resulted in error
type FailPredicate func(Attempt) bool

func IsNetworkError(attempt Attempt) bool {
	return attempt != nil && attempt.GetError() != nil
}

// Calculates in memory failure rate of an endpoint
type FailRateMeter struct {
	lastUpdated    time.Time
	success        []int
	failure        []int
	endpoint       Endpoint
	buckets        int
	resolution     time.Duration
	isError        FailPredicate
	timeProvider   timetools.TimeProvider
	countedBuckets int // how many samples in different buckets have we collected so far
	lastBucket     int // last recorded bucket
}

func NewFailRateMeter(endpoint Endpoint, buckets int, resolution time.Duration, timeProvider timetools.TimeProvider, isError FailPredicate) (*FailRateMeter, error) {
	if buckets <= 0 {
		return nil, fmt.Errorf("Buckets should be >= 0")
	}
	if resolution < time.Second {
		return nil, fmt.Errorf("Resolution should be larger than a second")
	}
	if endpoint == nil {
		return nil, fmt.Errorf("Select an endpoint")
	}
	if isError == nil {
		isError = IsNetworkError
	}

	return &FailRateMeter{
		endpoint:     endpoint,
		buckets:      buckets,
		resolution:   resolution,
		isError:      isError,
		timeProvider: timeProvider,
		success:      make([]int, buckets),
		failure:      make([]int, buckets),
		lastBucket:   -1,
	}, nil
}

func (em *FailRateMeter) Reset() {
	em.lastBucket = -1
	em.countedBuckets = 0
	em.lastUpdated = time.Time{}
	for i, _ := range em.success {
		em.success[i] = 0
		em.failure[i] = 0
	}
}

func (em *FailRateMeter) IsReady() bool {
	return em.countedBuckets >= em.buckets
}

func (em *FailRateMeter) GetRate() float64 {
	// Cleanup the data that was here in case if endpoint has been inactive for some time
	em.cleanup(em.failure)
	em.cleanup(em.success)

	success := em.sum(em.success)
	failure := em.sum(em.failure)
	// No data, return ok
	if success+failure == 0 {
		return 0
	}
	return float64(failure) / float64(success+failure)
}

func (em *FailRateMeter) Before(r Request) (*http.Response, error) {
	return nil, nil
}

func (em *FailRateMeter) After(r Request) error {
	lastAttempt := r.GetLastAttempt()
	if lastAttempt == nil || lastAttempt.GetEndpoint() != em.endpoint {
		return nil
	}
	// Cleanup the data that was here in case if endpoint has been inactive for some time
	em.cleanup(em.failure)
	em.cleanup(em.success)

	if em.isError(lastAttempt) {
		em.incBucket(em.failure)
	} else {
		em.incBucket(em.success)
	}
	return nil
}

// Returns the number in the moving window bucket that this slot occupies
func (em *FailRateMeter) getBucket(t time.Time) int {
	return int(t.Truncate(em.resolution).Unix() % int64(em.buckets))
}

func (em *FailRateMeter) incBucket(buckets []int) {
	now := em.timeProvider.UtcNow()
	bucket := em.getBucket(now)
	buckets[bucket] += 1
	em.lastUpdated = now
	// update usage stats if we haven't collected enough
	if !em.IsReady() {
		if em.lastBucket != bucket {
			em.lastBucket = bucket
			em.countedBuckets += 1
		}
	}
}

// Reset buckets that were not updated
func (em *FailRateMeter) cleanup(buckets []int) {
	now := em.timeProvider.UtcNow()
	for i := 0; i < em.buckets; i++ {
		now = now.Add(time.Duration(-1*i) * em.resolution)
		if now.Truncate(em.resolution).After(em.lastUpdated.Truncate(em.resolution)) {
			buckets[em.getBucket(now)] = 0
		} else {
			break
		}
	}
}

func (em *FailRateMeter) sum(buckets []int) int64 {
	out := int64(0)
	for _, v := range buckets {
		out += int64(v)
	}
	return out
}
