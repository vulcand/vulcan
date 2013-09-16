package vulcan

import (
	"fmt"
	"time"
	"tux21b.org/v1/gocql"
)

type ThrottlerConfig struct {
	Keyspace    string
	Consistency gocql.Consistency
	Servers     []string
}

type Throttler struct {
	session *gocql.Session // session
}

func NewThrottler(config ThrottlerConfig) (*Throttler, error) {
	cluster := gocql.NewCluster(config.Servers...)
	cluster.Consistency = config.Consistency
	cluster.Keyspace = config.Keyspace
	cluster.ProtoVersion = 1
	return &Throttler{
		session: cluster.CreateSession(),
	}, nil
}

func (t *Throttler) getUpstream(request *AuthResponse) error {
	for _, token := range request.Tokens {
		for _, rate := range token.Rates {
			counter, err := t.getStats(token.Id, rate)
			if err != nil {
				return err
			}
			err = t.updateStats(token.Id, rate)
			if err != nil {
				return err
			}
			LogMessage(
				"Got stats: %d for token: %s, rate: %v",
				counter, token.Id, rate)
		}
	}
	return nil
}

func (t *Throttler) updateStats(key string, rate Rate) error {

	// select a single tweet
	query := t.session.Query(
		"UPDATE hits SET counter = counter + 1 WHERE hit = ?", getHit(key, rate))

	if err := query.Exec(); err != nil {
		LogError("Error when executing update query, err: %v", err)
		return err
	}
	return nil
}

func (t *Throttler) getStats(key string, rate Rate) (int, error) {
	var counter int

	// select a single tweet
	query := t.session.Query(
		"SELECT counter from hits WHERE hit = ? LIMIT 1", getHit(key, rate))

	if err := query.Scan(&counter); err != nil {
		if err == gocql.ErrNotFound {
			LogMessage("Entry not found, it's ok")
			return 0, nil
		}
		LogError("Error when executing query, err: %v", err)
		return -1, err
	}

	LogMessage("Got counter: %d", counter)
	return counter, nil
}

func getHit(key string, rate Rate) string {
	return fmt.Sprintf(
		"%s_%s_%d", key, rate.Period, timeBucket(time.Now().UTC(), rate))
}

//Returns epochSeconds rounded to the rate period
//e.g. minutes rate would return epoch seconds with seconds set to zero
//hourly rate would return epoch seconds with minutes and seconds set to zero
func timeBucket(t time.Time, rate Rate) int {
	return int(t.Round(rate.Period).Unix())
}
