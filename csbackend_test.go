package vulcan

import (
	"fmt"
	"github.com/mailgun/gocql"
	. "launchpad.net/gocheck"
	"os"
	"time"
)

type CassandraBackendSuite struct {
	timeProvider *FreezedTime
	backend      *CassandraBackend
	shouldSkip   bool
	keySpace     string
}

var _ = Suite(&CassandraBackendSuite{})

func (s *CassandraBackendSuite) SetUpSuite(c *C) {
	if os.Getenv("CASSANDRA") != "yes" {
		s.shouldSkip = true
		return
	}
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.keySpace = "vulcan_csbackend_test"
	s.timeProvider = &FreezedTime{CurrentTime: start}
	cassandraConfig := CassandraConfig{
		Servers:     []string{"localhost"},
		Keyspace:    s.keySpace,
		Consistency: gocql.One,
	}
	backend, err := NewCassandraBackend(cassandraConfig, s.timeProvider)
	c.Assert(err, IsNil)
	s.backend = backend
	backend.session.Query(
		fmt.Sprintf("DROP KEYSPACE %s", s.keySpace)).Exec()

	err = backend.session.Query(fmt.Sprintf(`CREATE KEYSPACE %s
				WITH replication = {
					'class' : 'SimpleStrategy',
					'replication_factor' : 1
				}`, s.keySpace)).Exec()

	if err != nil {
		c.Fatalf("Failed to create keyspace: %s", err)
	}

	// create table hitscompact () WITH COMPACT STORAGE;
	err = backend.session.Query(`CREATE TABLE hits (
                    hit text PRIMARY KEY,
                    value counter
				) WITH COMPACT STORAGE`).Exec()
	if err != nil {
		c.Fatal("Failed to create table", err)
	}
}

func (s *CassandraBackendSuite) TestUtcNow(c *C) {
	if s.shouldSkip {
		c.Skip("Cassandra backend is not activated")
	}
	c.Assert(s.backend.utcNow(), Equals, s.timeProvider.CurrentTime)
}

func (s *CassandraBackendSuite) TestBackendGetSet(c *C) {
	if s.shouldSkip {
		c.Skip("Cassandra backend is not activated")
	}

	counter, err := s.backend.getStats("key1", &Rate{Increment: 1, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)
	c.Assert(counter, Equals, int64(0))

	err = s.backend.updateStats("key1", &Rate{Increment: 2, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)

	counter, err = s.backend.getStats("key1", &Rate{Increment: 2, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)
	c.Assert(counter, Equals, int64(2))
}
