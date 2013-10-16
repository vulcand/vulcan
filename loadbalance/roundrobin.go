/* Implements round robin load balancing algorithm.

* As long as vulcan does not have static endpoints configurations most of the time,
it keeps track of the endpoints that were used recently and keeps cursor for these endpoints for a while.

* Unused cursors are being expired and removed from the map if they have not been used
for 60 seconds.

* If the load balancer can find matching cursor for the given endpoints, algo simply advances to the next one,
taking into consideration endpoint availability.
*/
package loadbalance

import (
	"bytes"
	"container/heap"
	"fmt"
	"github.com/golang/glog"
	"github.com/mailgun/vulcan/datastruct"
	"github.com/mailgun/vulcan/timeutils"
	"sync"
)

type RoundRobin struct {
	// collection of cursors identified by the ids of endpoints
	cursors map[string]*cursor
	// time provider (mostly for testing as we need to override time
	timeProvider timeutils.TimeProvider
	// keep expiration times in the priority queue (min heap) so we can TTL effectively
	expiryTimes *datastruct.PriorityQueue
	// keep in mind that load balancer used by different endpoints
	mutex *sync.Mutex
}

// Cursor represents the current position in the given endpoints sequence
type cursor struct {
	index int
	id    string
	item  *datastruct.Item
}

func NewRoundRobin(timeProvider timeutils.TimeProvider) *RoundRobin {
	pq := &datastruct.PriorityQueue{}
	heap.Init(pq)
	return &RoundRobin{
		timeProvider: timeProvider,
		cursors:      make(map[string]*cursor),
		expiryTimes:  pq,
		mutex:        &sync.Mutex{},
	}
}

func (r *RoundRobin) NextEndpoint(endpoints []Endpoint) (Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("Need some endpoints")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	// Cleanup unused cursors
	r.cleanupGarbage()
	// Get existing or create new cursor
	cursor := r.getCursor(endpoints)
	// Return the next endpoint referred by this cursor
	return cursor.next(endpoints)
}

const ExpirySeconds = 60

// Returns cursor - existing one if it has been referred before, or the new one
// it also manages the expiry times of the cursors, so active cursors won't be deleted
func (r *RoundRobin) getCursor(endpoints []Endpoint) *cursor {
	cursor := newCursor(endpoints)
	// Find if the endpoints combination we are referring to already exists
	existingCursor, exists := r.cursors[cursor.id]
	expirySeconds := int(r.timeProvider.UtcNow().Unix()) + ExpirySeconds

	if exists {
		// In case if the set is present, use it and update expiry seconds
		r.expiryTimes.Update(existingCursor.item, expirySeconds)
		return existingCursor
	} else {
		// In case if we have not seen this set of endpoints before,
		// add it to the expiryTimes priority queue and the map of our endpoint set
		r.cursors[cursor.id] = cursor
		item := &datastruct.Item{
			Value:    cursor.id,
			Priority: expirySeconds,
		}
		cursor.item = item
		heap.Push(r.expiryTimes, item)
		return cursor
	}
}

func (r *RoundRobin) cleanupGarbage() {
	glog.Infof("RoundRobin gc start: %d cursors, expiry times: %d", len(r.cursors), len(*r.expiryTimes))
	for {
		if r.expiryTimes.Len() == 0 {
			break
		}
		item := r.expiryTimes.Peek()
		now := int(r.timeProvider.UtcNow().Unix())
		if item.Priority > now {
			glog.Infof("Nothing to expire, earliest expiry is: Cursor(id=%s, lastAccess=%d), now is %d", item.Value, item.Priority, now)
			break
		} else {
			glog.Infof("Cursor(id=%s, lastAccess=%d) has expired (now=%d), deleting", item.Value, item.Priority, now)
			pitem := heap.Pop(r.expiryTimes)
			item := pitem.(*datastruct.Item)
			delete(r.cursors, item.Value)
		}
	}
	glog.Infof("RoundRobin gc end: %d cursors, expiry times: %d", len(r.cursors), len(*r.expiryTimes))
}

func newCursor(endpoints []Endpoint) *cursor {
	return &cursor{
		index: 0,
		id:    makeId(endpoints),
	}
}

func (s *cursor) next(endpoints []Endpoint) (Endpoint, error) {
	for i := 0; i < len(endpoints); i++ {
		endpoint := endpoints[s.index]
		s.index = (s.index + 1) % len(endpoints)
		if endpoint.IsActive() {
			return endpoint, nil
		} else {
			glog.Infof("Skipping inactive endpoint: %s", endpoint.Id())
		}
	}
	// That means that we did full circle and found nothing
	return nil, fmt.Errorf("No available endpoints!")
}

func makeId(endpoints []Endpoint) string {
	buf := &bytes.Buffer{}
	totalLen := 0
	for _, endpoint := range endpoints {
		totalLen += len(endpoint.Id())
	}
	buf.Grow(totalLen)
	for _, endpoint := range endpoints {
		buf.WriteString(endpoint.Id())
	}
	return buf.String()
}
