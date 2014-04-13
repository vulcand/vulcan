package callback

import (
	"fmt"
	. "github.com/mailgun/vulcan/request"
	"net/http"
)

// A handdy container to aggregate groups of containers
type Chain struct {
	beforeChain []iBefore
	afterChain  []iAfter
}

type iBefore struct {
	id     string
	before Before
}

type iAfter struct {
	id    string
	after After
}

func NewChain() *Chain {
	return &Chain{
		beforeChain: []iBefore{},
		afterChain:  []iAfter{},
	}
}

func (c *Chain) AddBefore(id string, b Before) error {
	if i := c.indexBefore(id); i != -1 {
		return fmt.Errorf("Before Callback with id: %s already exists", id)
	}
	c.beforeChain = append(c.beforeChain, iBefore{id, b})
	return nil
}

func (c *Chain) AddAfter(id string, a After) error {
	if i := c.indexBefore(id); i != -1 {
		return fmt.Errorf("After Callback with id: %s already exists", id)
	}
	c.afterChain = append(c.afterChain, iAfter{id, a})
	return nil
}

func (c *Chain) indexBefore(id string) int {
	for i, cb := range c.beforeChain {
		if cb.id == id {
			return i
		}
	}
	return -1
}

func (c *Chain) indexAfter(id string) int {
	for i, cb := range c.afterChain {
		if cb.id == id {
			return i
		}
	}
	return -1
}

func (c *Chain) Before(r Request) (*http.Response, error) {
	for _, cb := range c.beforeChain {
		re, err := cb.before.Before(r)
		if re != nil || err != nil {
			return re, err
		}
	}
	return nil, nil
}

func (c *Chain) After(r Request) error {
	for _, cb := range c.afterChain {
		err := cb.after.After(r)
		if err != nil {
			return err
		}
	}
	return nil
}
