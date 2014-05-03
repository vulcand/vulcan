package middleware

import (
	//	"fmt"
	"github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcan/request"
	. "launchpad.net/gocheck"
	"net/http"
	"sync"
	"testing"
)

func TestChain(t *testing.T) { TestingT(t) }

var _ = Suite(&ChainSuite{})

type ChainSuite struct {
	nilRe *http.Response
}

func (s *ChainSuite) TestMiddlewareEmptyChain(c *C) {
	chain := NewMiddlewareChain()
	iter := chain.GetIter()

	c.Assert(iter.Next(), Equals, Middleware(nil))
	c.Assert(iter.Next(), Equals, Middleware(nil))

	c.Assert(iter.Prev(), Equals, Middleware(nil))
	c.Assert(iter.Prev(), Equals, Middleware(nil))
}

func (s *ChainSuite) TestMiddlewareChainSingleElement(c *C) {
	chain := NewMiddlewareChain()

	r := &Recorder{}
	chain.Append("r", r)

	iter := chain.GetIter()

	c.Assert(iter.Next(), Equals, r)
	c.Assert(iter.Next(), Equals, Middleware(nil))

	c.Assert(iter.Prev(), Equals, r)
	c.Assert(iter.Prev(), Equals, Middleware(nil))
}

func (s *ChainSuite) TestMiddlewareChainAddRemoveGet(c *C) {
	chain := NewMiddlewareChain()

	r := &Recorder{}
	chain.Append("r", r)
	c.Assert(chain.Get("r"), NotNil)
	chain.Remove("r")
	c.Assert(chain.Get("r"), IsNil)
}

func (s *ChainSuite) TestMiddlewareIteration(c *C) {
	chain := NewMiddlewareChain()

	m1 := &Recorder{}
	m2 := &Recorder{}
	chain.Append("m1", m1)
	chain.Append("m2", m2)

	iter := chain.GetIter()
	c.Assert(iter.Next(), Equals, m1)
	c.Assert(iter.Next(), Equals, m2)
	c.Assert(iter.Next(), Equals, nil)
	c.Assert(iter.Next(), Equals, nil)

	// And back
	c.Assert(iter.Prev(), Equals, m2)
	c.Assert(iter.Prev(), Equals, m1)
	c.Assert(iter.Prev(), Equals, nil)
	c.Assert(iter.Prev(), Equals, nil)
}

// Make sure updates to the chain do not affect the iterators created before updates
func (s *ChainSuite) TestMiddlewareVersionedIteration(c *C) {
	chain := NewMiddlewareChain()

	m1 := &Recorder{}
	m2 := &Recorder{}
	chain.Append("m1", m1)
	chain.Append("m2", m2)

	iter := chain.GetIter()
	c.Assert(iter.Next(), Equals, m1)
	c.Assert(iter.Next(), Equals, m2)

	m3 := &Recorder{}
	chain.Append("m3", m3)

	c.Assert(iter.Next(), Equals, nil)
	c.Assert(iter.Prev(), Equals, m2)
	c.Assert(iter.Prev(), Equals, m1)
	c.Assert(iter.Prev(), Equals, nil)

	// New iterator would see the changes
	iter2 := chain.GetIter()
	c.Assert(iter2.Next(), Equals, m1)
	c.Assert(iter2.Next(), Equals, m2)
	c.Assert(iter2.Next(), Equals, m3)
}

func (s *ChainSuite) TestConcurrentConsumption(c *C) {
	chain := NewMiddlewareChain()

	r := &Recorder{}
	chain.Append("r", r)

	wg := &sync.WaitGroup{}

	for i := 0; i < 10; i += 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			iter := chain.GetIter()

			re, err := iter.Next().ProcessRequest(nil)
			c.Assert(err, IsNil)
			c.Assert(re, IsNil)
			c.Assert(iter.Next(), IsNil)

			iter.Prev().ProcessResponse(nil, nil)
		}()
	}

	wg.Wait()
	c.Assert(len(r.ProcessedRequests), Equals, 10)
	c.Assert(len(r.ProcessedResponses), Equals, 10)
}

func (s *ChainSuite) TestObserverEmptyChain(c *C) {
	chain := NewObserverChain()
	chain.ObserveRequest(nil)
	chain.ObserveResponse(nil, nil)
}

func (s *ChainSuite) TestObserverChainOrder(c *C) {
	chain := NewObserverChain()

	r1 := &Recorder{Header: http.Header{"X-Call": []string{"r1"}}}
	r2 := &Recorder{Header: http.Header{"X-Call": []string{"r2"}}}
	chain.Append("r1", r1)
	chain.Append("r2", r2)

	req := makeRequest()

	chain.ObserveRequest(req)
	c.Assert(len(r1.ProcessedRequests), Equals, 1)
	c.Assert(len(r2.ProcessedRequests), Equals, 1)

	chain.ObserveResponse(req, nil)
	c.Assert(len(r1.ProcessedResponses), Equals, 1)
	c.Assert(len(r2.ProcessedResponses), Equals, 1)

	c.Assert(req.GetHttpRequest().Header["X-Call"], DeepEquals, []string{"r1", "r2", "r2", "r1"})
}

func (s *ChainSuite) TestUpdatePreservesOrder(c *C) {
	chain := NewMiddlewareChain()

	m1 := &Recorder{}
	m2 := &Recorder{}
	chain.Append("b", m1)
	chain.Append("a", m2)

	iter := chain.GetIter()
	c.Assert(iter.Next(), Equals, m1)
	c.Assert(iter.Next(), Equals, m2)
	c.Assert(iter.Next(), Equals, nil)

	// Now update the middleware to something else
	m3 := &Recorder{}
	chain.Update("b", m3)

	iter2 := chain.GetIter()
	c.Assert(iter2.Next(), Equals, m3)
	c.Assert(iter2.Next(), Equals, m2)
	c.Assert(iter2.Next(), Equals, nil)
}

func (s *ChainSuite) TestUpsertPreservesOrder(c *C) {
	chain := NewMiddlewareChain()

	m1 := &Recorder{}
	m2 := &Recorder{}
	chain.Append("b", m1)
	chain.Append("a", m2)

	iter := chain.GetIter()
	c.Assert(iter.Next(), Equals, m1)
	c.Assert(iter.Next(), Equals, m2)

	// Now update the middleware to something else
	m3 := &Recorder{}
	chain.Upsert("b", m3)

	iter = chain.GetIter()
	c.Assert(iter.Next(), Equals, m3)
	c.Assert(iter.Next(), Equals, m2)
}

func (s *ChainSuite) TestRemove(c *C) {
	chain := NewMiddlewareChain()

	m1 := &Recorder{}
	m2 := &Recorder{}
	chain.Append("m1", m1)
	chain.Append("m2", m2)
	c.Assert(chain.Remove("m1"), IsNil)

	iter := chain.GetIter()
	c.Assert(iter.Next(), Equals, m2)
	c.Assert(iter.Next(), Equals, nil)
}

func (s *ChainSuite) TestUpsertNew(c *C) {
	chain := NewMiddlewareChain()

	m1 := &Recorder{}
	chain.Upsert("m1", m1)

	iter := chain.GetIter()
	c.Assert(iter.Next(), Equals, m1)
}

func (s *ChainSuite) TestMiddlewareChainGet(c *C) {
	chain := NewMiddlewareChain()

	c.Assert(chain.Get("val"), Equals, Middleware(nil))

	m1 := &Recorder{}
	m2 := &Recorder{}

	chain.Append("m1", m1)
	chain.Append("m2", m2)

	c.Assert(chain.Get("m1"), Equals, m1)
	c.Assert(chain.Get("m2"), Equals, m2)
}

func (s *ChainSuite) TestObserverChainGet(c *C) {
	chain := NewObserverChain()

	c.Assert(chain.Get("val"), Equals, Observer(nil))

	m1 := &Recorder{}
	m2 := &Recorder{}

	chain.Append("m1", m1)
	chain.Append("m2", m2)

	c.Assert(chain.Get("m1"), Equals, m1)
	c.Assert(chain.Get("m2"), Equals, m2)
}

func (s *ChainSuite) TestAlreadyExists(c *C) {
	chain := NewMiddlewareChain()

	m := &Recorder{}
	c.Assert(chain.Append("r", m), IsNil)
	c.Assert(chain.Append("r", m), NotNil)
}

func (s *ChainSuite) TestUpdateNotFound(c *C) {
	chain := NewMiddlewareChain()
	c.Assert(chain.Update("m", nil), NotNil)
}

func (s *ChainSuite) TestRemoveNotFound(c *C) {
	chain := NewMiddlewareChain()
	c.Assert(chain.Remove("m"), NotNil)
}

type Recorder struct {
	ProcessedRequests  []Request
	ProcessedResponses []struct {
		R Request
		A Attempt
	}
	Response *http.Response
	Error    error
	Header   http.Header
	mutex    sync.Mutex
}

func (tb *Recorder) ProcessRequest(req Request) (*http.Response, error) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	if len(tb.Header) != 0 {
		netutils.CopyHeaders(req.GetHttpRequest().Header, tb.Header)
	}
	tb.ProcessedRequests = append(tb.ProcessedRequests, req)
	return tb.Response, tb.Error
}

func (tb *Recorder) ProcessResponse(req Request, a Attempt) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	if len(tb.Header) != 0 {
		netutils.CopyHeaders(req.GetHttpRequest().Header, tb.Header)
	}
	tb.ProcessedResponses = append(tb.ProcessedResponses, struct {
		R Request
		A Attempt
	}{R: req, A: a})
}

func (tb *Recorder) ObserveRequest(req Request) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	if len(tb.Header) != 0 {
		netutils.CopyHeaders(req.GetHttpRequest().Header, tb.Header)
	}
	tb.ProcessedRequests = append(tb.ProcessedRequests, req)
}

func (tb *Recorder) ObserveResponse(req Request, a Attempt) {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	if len(tb.Header) != 0 {
		netutils.CopyHeaders(req.GetHttpRequest().Header, tb.Header)
	}
	tb.ProcessedResponses = append(tb.ProcessedResponses, struct {
		R Request
		A Attempt
	}{R: req, A: a})
}

func makeRequest() Request {
	return &BaseRequest{
		HttpRequest: &http.Request{
			Header: http.Header{},
		},
	}
}
