/*
Cassandra backend.
Based on cassandra counters.
*/
package vulcan

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/mailgun/gocql"
	"time"
)

type CassandraConfig struct {
	Keyspace    string
	Consistency gocql.Consistency
	Servers     []string
}

type CassandraBackend struct {
	session      *gocql.Session // session
	timeProvider TimeProvider
	bucketSize   int
}

func NewCassandraBackend(config CassandraConfig, timeProvider TimeProvider) (*CassandraBackend, error) {
	if len(config.Servers) == 0 {
		return nil, fmt.Errorf("At least one node is required")
	}
	if len(config.Keyspace) == 0 {
		return nil, fmt.Errorf("Keyspace is missing")
	}

	cluster := gocql.NewCluster(config.Servers...)
	cluster.Consistency = config.Consistency
	cluster.Keyspace = config.Keyspace
	cluster.ProtoVersion = 1
	return &CassandraBackend{
		session:      cluster.CreateSession(),
		timeProvider: timeProvider,
	}, nil
}

func (b *CassandraBackend) getStats(key string, rate *Rate) (int64, error) {
	var counter int64

	glog.Infof("Get stats hit: %s", getHit(b.timeProvider.utcNow(), key, rate))

	query := b.session.Query(
		"SELECT value from hits WHERE hit = ? LIMIT 1",
		getHit(b.timeProvider.utcNow(), key, rate))

	if err := query.Scan(&counter); err != nil {
		if err == gocql.ErrNotFound {
			glog.Infof("Entry %s not found, it's ok", key)
			return 0, nil
		}
		glog.Error("Error when executing query, err:", err)
		return -1, err
	}
	return counter, nil
}

func (b *CassandraBackend) updateStats(key string, rate *Rate) error {
	query := b.session.Query(
		"UPDATE hits SET value = value + ? WHERE hit = ?",
		rate.Increment,
		getHit(b.timeProvider.utcNow(), key, rate))

	if err := query.Exec(); err != nil {
		glog.Error("Error when executing update query, err:", err)
		return err
	}
	return nil
}

func (b *CassandraBackend) utcNow() time.Time {
	return b.timeProvider.utcNow()
}
