/*
Cassandra backend.
Based on cassandra counters.
*/
package vulcan

import (
	"time"
	"tux21b.org/v1/gocql"
)

type CassandraConfig struct {
	Keyspace    string
	Consistency gocql.Consistency
	Servers     []string
}

type CassandraBackend struct {
	session      *gocql.Session // session
	timeProvider TimeProvider
}

func NewCassandraBackend(config CassandraConfig, timeProvider TimeProvider) (*CassandraBackend, error) {
	cluster := gocql.NewCluster(config.Servers...)
	cluster.Consistency = config.Consistency
	cluster.Keyspace = config.Keyspace
	cluster.ProtoVersion = 1
	return &CassandraBackend{
		session:      cluster.CreateSession(),
		timeProvider: timeProvider,
	}, nil
}

func (b *CassandraBackend) getStats(key string, rate *Rate) (int, error) {
	var counter int

	query := b.session.Query(
		"SELECT counter from hits WHERE hit = ? LIMIT 1",
		getHit(b.timeProvider.utcNow(), key, rate))

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

func (b *CassandraBackend) updateStats(key string, rate *Rate, increment int) error {
	query := b.session.Query(
		"UPDATE hits SET counter = counter + ? WHERE hit = ?",
		increment,
		getHit(b.timeProvider.utcNow(), key, rate))

	if err := query.Exec(); err != nil {
		LogError("Error when executing update query, err: %v", err)
		return err
	}
	return nil
}

func (b *CassandraBackend) utcNow() time.Time {
	return b.timeProvider.utcNow()
}
