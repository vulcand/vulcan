/*
Memory backend, used mostly for testing.
*/
package vulcan

import "time"

type MemoryBackend struct {
	hits         map[string]int
	timeProvider TimeProvider
}

func NewMemoryBackend(timeProvider TimeProvider) (*MemoryBackend, error) {
	return &MemoryBackend{
		hits:         map[string]int{},
		timeProvider: timeProvider,
	}, nil
}

func (b *MemoryBackend) getStats(key string, rate *Rate) (int, error) {
	return b.hits[getHit(b.timeProvider.utcNow(), key, rate)], nil
}

func (b *MemoryBackend) updateStats(key string, rate *Rate, increment int) error {
	b.hits[getHit(b.timeProvider.utcNow(), key, rate)] += 1
	return nil
}

func (b *MemoryBackend) utcNow() time.Time {
	return b.timeProvider.utcNow()
}
