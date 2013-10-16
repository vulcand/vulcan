package datastruct

import (
	"container/heap"
	. "launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type PQSuite struct{}

var _ = Suite(&PQSuite{})

func (s *PQSuite) TestPeek(c *C) {
	pq := &PriorityQueue{}
	heap.Init(pq)

	item := &Item{
		Value:    "a",
		Priority: 5,
	}
	heap.Push(pq, item)
	c.Assert(pq.Peek().Value, Equals, "a")
	c.Assert(pq.Len(), Equals, 1)

	item = &Item{
		Value:    "b",
		Priority: 1,
	}
	heap.Push(pq, item)
	c.Assert(pq.Len(), Equals, 2)
	c.Assert(pq.Peek().Value, Equals, "b")
	c.Assert(pq.Peek().Value, Equals, "b")
	c.Assert(pq.Len(), Equals, 2)

	pitem := heap.Pop(pq)
	item, ok := pitem.(*Item)
	if !ok {
		panic("Impossible")
	}
	c.Assert(item.Value, Equals, "b")
	c.Assert(pq.Len(), Equals, 1)
	c.Assert(pq.Peek().Value, Equals, "a")

	heap.Pop(pq)
	c.Assert(pq.Len(), Equals, 0)
}

func (s *PQSuite) TestUpdate(c *C) {
	pq := &PriorityQueue{}
	heap.Init(pq)
	x := &Item{
		Value:    "x",
		Priority: 4,
	}
	y := &Item{
		Value:    "y",
		Priority: 3,
	}
	z := &Item{
		Value:    "z",
		Priority: 8,
	}
	heap.Push(pq, x)
	heap.Push(pq, y)
	heap.Push(pq, z)
	c.Assert(pq.Peek().Value, Equals, "y")

	pq.Update(z, 1)
	c.Assert(pq.Peek().Value, Equals, "z")

	pq.Update(x, 0)
	c.Assert(pq.Peek().Value, Equals, "x")
}
