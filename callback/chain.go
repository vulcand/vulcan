package callback

import (
	"fmt"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
)

type BeforeChain struct {
	chain *chain
}

func NewBeforeChain() *BeforeChain {
	return &BeforeChain{
		chain: newChain(),
	}
}

func (c *BeforeChain) Add(id string, b Before) error {
	return c.chain.add(id, b)
}

func (c *BeforeChain) Remove(id string) error {
	return c.chain.remove(id)
}

func (c *BeforeChain) Update(id string, b Before) error {
	return c.chain.update(id, b)
}

func (c *BeforeChain) Before(r Request) (*http.Response, error) {
	it := c.chain.getIter()
	for v := it.next(); v != nil; v = it.next() {
		response, err := v.(Before).Before(r)
		if err != nil || response != nil {
			return response, err
		}
	}
	return nil, nil
}

type AfterChain struct {
	chain *chain
}

func NewAfterChain() *AfterChain {
	return &AfterChain{
		chain: newChain(),
	}
}

func (c *AfterChain) Add(id string, a After) error {
	return c.chain.add(id, a)
}

func (c *AfterChain) Remove(id string) error {
	return c.chain.remove(id)
}

func (c *AfterChain) Update(id string, a After) error {
	return c.chain.update(id, a)
}

func (c *AfterChain) After(r Request) error {
	it := c.chain.getIter()
	for v := it.next(); v != nil; v = it.next() {
		err := v.(After).After(r)
		if err != nil {
			return err
		}
	}
	return nil
}

// Map with guaranteed iteration order, in place updates that do not change the order
// and iterator that does not hold locks
type chain struct {
	mutex     *sync.RWMutex
	callbacks []*callback
	indexes   map[string]int // Indexes for in place updates
	iter      *iter          //current version of iterator
}

type callback struct {
	id string
	cb interface{}
}

func newChain() *chain {
	return &chain{
		mutex:     &sync.RWMutex{},
		callbacks: []*callback{},
		indexes:   make(map[string]int),
		iter:      newIter([]*callback{}),
	}
}

func (c *chain) add(id string, cb interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, found := c.indexes[id]; found {
		return fmt.Errorf("Callback with id: %s already exists", id)
	}
	c.callbacks = append(c.callbacks, &callback{id, cb})
	c.indexes[id] = len(c.callbacks) - 1
	c.iter = newIter(c.callbacks)
	return nil
}

func (c *chain) update(id string, cb interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	i, found := c.indexes[id]
	if !found {
		return fmt.Errorf("Callback with id: %s not found", id)
	}
	c.callbacks[i].cb = cb
	c.iter = newIter(c.callbacks)
	return nil
}

func (c *chain) remove(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	i, found := c.indexes[id]
	if !found {
		return fmt.Errorf("Callback with id: %s not found", id)
	}
	c.callbacks = append(c.callbacks[:i], c.callbacks[i+1:]...)
	for i, cb := range c.callbacks {
		c.indexes[cb.id] = i
	}
	c.iter = newIter(c.callbacks)
	return nil
}

// Note that we hold read lock to get access to the current iterator
func (c *chain) getIter() *iter {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.iter
}

type iter struct {
	index     int
	callbacks []*callback
}

func newIter(callbacks []*callback) *iter {
	out := make([]*callback, len(callbacks))
	for i, cb := range callbacks {
		out[i] = cb
	}
	return &iter{
		callbacks: out,
	}
}

func (it *iter) next() interface{} {
	if it.index >= len(it.callbacks) {
		return nil
	}
	val := it.callbacks[it.index].cb
	it.index += 1
	return val
}
