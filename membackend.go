/*
Memory backend, used mostly for testing, but may be extended to become
more useful in the future. In this case it'll need garbage collection.
*/
package vulcan

import "time"

type MemoryBackend struct {
	hits         map[string]int64
	timeProvider TimeProvider
}

func NewMemoryBackend(timeProvider TimeProvider) (*MemoryBackend, error) {
	return &MemoryBackend{
		hits:         map[string]int64{},
		timeProvider: timeProvider,
	}, nil
}

func (b *MemoryBackend) getStats(key string, rate *Rate) (int64, error) {
	return b.hits[getHit(b.timeProvider.utcNow(), key, rate)], nil
}

func (b *MemoryBackend) updateStats(key string, rate *Rate) error {
	b.hits[getHit(b.timeProvider.utcNow(), key, rate)] += rate.Increment
	return nil
}

func (b *MemoryBackend) utcNow() time.Time {
	return b.timeProvider.utcNow()
}
