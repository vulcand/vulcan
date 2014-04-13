package callback

import (
	"fmt"
	"github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcan/request"
	. "launchpad.net/gocheck"
	"net/http"
	"testing"
)

func TestChain(t *testing.T) { TestingT(t) }

var _ = Suite(&ChainSuite{})

type ChainSuite struct {
	nilRe *http.Response
}

func (s *ChainSuite) TestBeforeEmptyChain(c *C) {
	chain := NewBeforeChain()
	re, err := chain.Before(nil)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
}

func (s *ChainSuite) TestAfterEmptyChain(c *C) {
	chain := NewAfterChain()
	err := chain.After(nil)
	c.Assert(err, IsNil)
}

func (s *ChainSuite) TestBeforeChainSingleElement(c *C) {
	chain := NewBeforeChain()

	cb := &TestBefore{}
	chain.Add("rate", cb)

	re, err := chain.Before(nil)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	c.Assert(len(cb.Requests), Equals, 1)
}

func (s *ChainSuite) TestUpdatePreservesOrder(c *C) {
	chain := NewBeforeChain()

	cb := &TestBefore{Header: http.Header{"X-Call": []string{"b"}}}
	cb2 := &TestBefore{Header: http.Header{"X-Call": []string{"a"}}}
	chain.Add("b", cb)
	chain.Add("a", cb2)

	req := makeRequest()
	re, err := chain.Before(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	c.Assert(len(cb.Requests), Equals, 1)
	c.Assert(len(cb2.Requests), Equals, 1)

	c.Assert(req.GetHttpRequest().Header["X-Call"], DeepEquals, []string{"b", "a"})

	// Now update the callback to something else
	cb3 := &TestBefore{Header: http.Header{"X-Call": []string{"new"}}}
	chain.Update("b", cb3)

	req = makeRequest()
	re, err = chain.Before(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	c.Assert(len(cb.Requests), Equals, 1)
	c.Assert(len(cb2.Requests), Equals, 2)
	c.Assert(len(cb3.Requests), Equals, 1)
	c.Assert(req.GetHttpRequest().Header["X-Call"], DeepEquals, []string{"new", "a"})
}

func (s *ChainSuite) TestRemove(c *C) {
	chain := NewBeforeChain()

	cb := &TestBefore{Header: http.Header{"X-Call": []string{"cb"}}}
	cb2 := &TestBefore{Header: http.Header{"X-Call": []string{"cb2"}}}
	chain.Add("cb", cb)
	chain.Add("cb2", cb2)
	c.Assert(chain.Remove("cb"), IsNil)

	req := makeRequest()
	re, err := chain.Before(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	c.Assert(len(cb.Requests), Equals, 0)
	c.Assert(len(cb2.Requests), Equals, 1)

	c.Assert(req.GetHttpRequest().Header["X-Call"], DeepEquals, []string{"cb2"})
}

func (s *ChainSuite) TestInterceptChainWithError(c *C) {
	chain := NewBeforeChain()

	cb := &TestBefore{Error: fmt.Errorf("cb")}
	cb2 := &TestBefore{}

	chain.Add("cb", cb)
	chain.Add("cb2", cb2)

	req := makeRequest()
	re, err := chain.Before(req)
	c.Assert(re, IsNil)
	c.Assert(err, Equals, cb.Error)

	c.Assert(len(cb.Requests), Equals, 1)
	c.Assert(len(cb2.Requests), Equals, 0)
}

func (s *ChainSuite) TestInterceptChainWithResponse(c *C) {
	chain := NewBeforeChain()

	cb := &TestBefore{Response: &http.Response{}}
	cb2 := &TestBefore{}

	chain.Add("cb", cb)
	chain.Add("cb2", cb2)

	req := makeRequest()
	re, err := chain.Before(req)
	c.Assert(re, Equals, cb.Response)
	c.Assert(err, IsNil)

	c.Assert(len(cb.Requests), Equals, 1)
	c.Assert(len(cb2.Requests), Equals, 0)
}

func (s *ChainSuite) TestAlreadyExists(c *C) {
	chain := NewBeforeChain()

	cb := &TestBefore{}
	c.Assert(chain.Add("rate", cb), IsNil)
	c.Assert(chain.Add("rate", cb), NotNil)
}

func (s *ChainSuite) TestUpdateNotFound(c *C) {
	chain := NewBeforeChain()
	c.Assert(chain.Update("rate", nil), NotNil)
}

func (s *ChainSuite) TestRemoveNotFound(c *C) {
	chain := NewBeforeChain()
	c.Assert(chain.Remove("rate"), NotNil)
}

func (s *ChainSuite) TestAfterChain(c *C) {
	chain := NewAfterChain()
	cb := &TestAfter{}
	chain.Add("cb", cb)

	err := chain.After(makeRequest())
	c.Assert(err, IsNil)
	c.Assert(len(cb.Requests), Equals, 1)
}

func (s *ChainSuite) TestAfterChainUpdate(c *C) {
	chain := NewAfterChain()

	cb := &TestAfter{}
	cb2 := &TestAfter{}
	cb3 := &TestAfter{}

	chain.Add("cb", cb)
	chain.Add("cb2", cb2)
	chain.Update("cb", cb3)

	err := chain.After(makeRequest())
	c.Assert(err, IsNil)
	c.Assert(len(cb.Requests), Equals, 0)
	c.Assert(len(cb2.Requests), Equals, 1)
	c.Assert(len(cb3.Requests), Equals, 1)
}

func (s *ChainSuite) TestAfterChainRemove(c *C) {
	chain := NewAfterChain()

	cb := &TestAfter{}
	cb2 := &TestAfter{}

	chain.Add("cb", cb)
	chain.Add("cb2", cb2)
	chain.Remove("cb")

	err := chain.After(makeRequest())
	c.Assert(err, IsNil)
	c.Assert(len(cb.Requests), Equals, 0)
	c.Assert(len(cb2.Requests), Equals, 1)
}

func (s *ChainSuite) TestAfterChainReturnError(c *C) {
	chain := NewAfterChain()

	cb := &TestAfter{Error: fmt.Errorf("cb")}
	cb2 := &TestAfter{}

	chain.Add("cb", cb)
	chain.Add("cb2", cb2)

	err := chain.After(makeRequest())
	c.Assert(err, Equals, cb.Error)

	c.Assert(len(cb.Requests), Equals, 1)
	c.Assert(len(cb2.Requests), Equals, 0)
}

func (tb *TestBefore) Before(req Request) (*http.Response, error) {
	tb.Requests = append(tb.Requests, req)
	if len(tb.Header) != 0 {
		netutils.CopyHeaders(req.GetHttpRequest().Header, tb.Header)
	}
	return tb.Response, tb.Error
}

type TestBefore struct {
	Requests []Request
	Response *http.Response
	Error    error
	Header   http.Header
}

type TestAfter struct {
	Requests []Request
	Error    error
}

func (tb *TestAfter) After(req Request) error {
	tb.Requests = append(tb.Requests, req)
	return tb.Error
}

func makeRequest() Request {
	return &BaseRequest{
		HttpRequest: &http.Request{
			Header: http.Header{},
		},
	}
}
