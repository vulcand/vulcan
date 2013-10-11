package vulcan

// Throttling backend interface, used by throttler
// to request stats about upstreams and tokens
type Backend interface {
	// Used to retreive time for current stats.
	// Creates mostly for test reasons so we can override
	// time in tests
	TimeProvider
	// Get hitcount of the given key in the time period defined
	// by rate.
	getStats(key string, rate *Rate) (int64, error)
	// Updates hitcount of the given key, notice that rate increment
	// property will can be used to bump counter,
	// e.g. to count request size
	updateStats(key string, rate *Rate) error
}
