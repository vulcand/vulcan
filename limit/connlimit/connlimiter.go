package connlimit

import (
	"fmt"
	. "github.com/mailgun/vulcan/limit"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
)

// This limiter tracks concurrent connection per
type ConnectionLimiter struct {
	mutex            *sync.Mutex
	mapper           MapperFn
	connections      map[string]int
	maxConnections   int
	totalConnections int64
}

func NewClientIpLimiter(maxConnections int) (*ConnectionLimiter, error) {
	return NewConnectionLimiter(MapClientIp, maxConnections)
}

func NewConnectionLimiter(mapper MapperFn, maxConnections int) (*ConnectionLimiter, error) {
	if mapper == nil {
		return nil, fmt.Errorf("Mapper function can not be nil")
	}
	if maxConnections <= 0 {
		return nil, fmt.Errorf("Max connections should be >= 0")
	}
	return &ConnectionLimiter{
		mutex:          &sync.Mutex{},
		mapper:         mapper,
		maxConnections: maxConnections,
		connections:    make(map[string]int),
	}, nil
}

func (cl *ConnectionLimiter) Before(r Request) (*http.Response, error) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	token, amount, err := cl.mapper(r)
	if err != nil {
		return nil, err
	}

	connections := cl.connections[token]
	if connections >= cl.maxConnections {
		return nil, fmt.Errorf("Connection limit reached. Max is: %d, yours: %d", cl.maxConnections, connections)
	}

	cl.connections[token] += amount
	cl.totalConnections += int64(amount)

	return nil, nil
}

func (cl *ConnectionLimiter) After(r Request) error {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	token, amount, err := cl.mapper(r)
	if err != nil {
		return err
	}
	cl.connections[token] -= amount
	cl.totalConnections -= int64(amount)

	// Otherwise it would grow forever
	if cl.connections[token] == 0 {
		delete(cl.connections, token)
	}

	return nil
}

func (cl *ConnectionLimiter) GetConnectionCount() int64 {
	return cl.totalConnections
}
