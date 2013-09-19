package vulcan

import "math"

type Backend interface {
	TimeProvider
	getStats(key string, rate *Rate) (int, error)
	updateStats(key string, rate *Rate, increment int) error
}

type Throttler struct {
	backend Backend
}

type TokenStats struct {
	token *Token
	stats []*RateStats
}

type UpstreamStats struct {
	upstream *Upstream
	stats    []*RateStats
}

type RateStats struct {
	requests int   // how many requests have been served for token or upstream
	rate     *Rate // rate for which the stats have been calculated
}

func NewThrottler(b Backend) *Throttler {
	return &Throttler{backend: b}
}

// Determines if the current usage stats allow the request.
// The request is allowed if:
//
// * Amount of requests identified by all tokens does not exceed any rate
// * There's at least on upstream which usage rate is not exceeded
//
// If the request is not allowed, minimum retry time is calculcated
// taking minimim of
//
// * The retry time of the slowest token
// (because if at least one token is not allowing the request, the request wil be denied)
// * The upstream that would become available the earliest
//
func (t *Throttler) throttle(instructions *ProxyInstructions) (upstreamStats []*UpstreamStats, retrySeconds int, err error) {
	retrySeconds, err = t.throttleTokens(instructions.Tokens)
	if err != nil || retrySeconds > 0 {
		return nil, retrySeconds, err
	}
	return t.throttleUpstreams(instructions.Upstreams)
}

func (t *Throttler) throttleTokens(tokens []*Token) (retrySeconds int, err error) {
	retrySeconds = 0
	for _, token := range tokens {
		tokenStats, err := t.getTokenStats(token)
		if err != nil {
			return -1, err
		}
		tokenRetry := t.statsRetrySeconds(tokenStats.stats)
		if tokenRetry > 0 {
			LogMessage("Token %s is out of capacity %s", token, tokenStats.stats)
			// we are interested in max retry seconds
			// because no request will succeed if there's at least
			// one token in tokens not allowing the request
			if tokenRetry > retrySeconds {
				retrySeconds = tokenRetry
			}
		}
	}
	return retrySeconds, nil
}

func (t *Throttler) throttleUpstreams(upstreams []*Upstream) (outUpstreams []*UpstreamStats, retrySeconds int, err error) {
	retrySeconds = math.MaxInt32
	outUpstreams = []*UpstreamStats{}
	for _, upstream := range upstreams {
		upstreamStats, err := t.getUpstreamStats(upstream)
		if err != nil {
			return nil, -1, err
		}

		upstreamRetry := t.statsRetrySeconds(upstreamStats.stats)
		if upstreamRetry > 0 {
			LogMessage("Upstream %s is out of capacity.", upstream)
			if upstreamRetry < retrySeconds {
				retrySeconds = upstreamRetry
			}
		} else {
			outUpstreams = append(outUpstreams, upstreamStats)
			retrySeconds = 0
		}
	}
	return outUpstreams, retrySeconds, nil
}

// Updates usage stats after the request is being made to the upstream
func (t *Throttler) updateStats(tokens []*Token, upstream *Upstream) error {
	for _, token := range tokens {
		err := t.updateTokenStats(token)
		if err != nil {
			return err
		}
	}
	return t.updateUpstreamStats(upstream)
}

func (t *Throttler) getTokenStats(token *Token) (*TokenStats, error) {
	stats, err := t.getRatesStats(token.Id, token.Rates)
	if err != nil {
		return nil, err
	}

	return &TokenStats{
		token: token,
		stats: stats,
	}, nil
}

func (t *Throttler) getUpstreamStats(upstream *Upstream) (*UpstreamStats, error) {
	stats, err := t.getRatesStats(upstream.Id(), upstream.Rates)
	if err != nil {
		return nil, err
	}

	return &UpstreamStats{
		upstream: upstream,
		stats:    stats,
	}, nil
}

func (t *Throttler) getRatesStats(id string, rates []*Rate) ([]*RateStats, error) {
	stats := make([]*RateStats, len(rates))

	for i, rate := range rates {
		requests, err := t.backend.getStats(id, rate)
		if err != nil {
			return nil, err
		}
		stats[i] = &RateStats{requests: requests, rate: rate}
	}
	return stats, nil
}

func (t *Throttler) updateTokenStats(token *Token) error {
	for _, rate := range token.Rates {
		err := t.backend.updateStats(token.Id, rate, 1)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Throttler) updateUpstreamStats(upstream *Upstream) error {
	for _, rate := range upstream.Rates {
		err := t.backend.updateStats(upstream.Id(), rate, 1)
		if err != nil {
			return err
		}
	}
	return nil
}

// Determines if the rate limit for any rate of the token has been hit
// if that's the case returns next time when the token can be available
// this is actually the biggest retry seconds of all the rates
// (if any rate is no request is allowed)
func (t *Throttler) statsRetrySeconds(stats []*RateStats) int {
	retry := 0
	for _, stat := range stats {
		//requests in a given period exceeded rate value
		if stat.requests >= stat.rate.Value {
			retrySeconds := stat.rate.retrySeconds(t.backend.utcNow())
			if retrySeconds > retry {
				retry = retrySeconds
			}
		}
	}
	return retry
}
