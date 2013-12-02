package js

import (
	"fmt"
	"github.com/mailgun/vulcan/client"
	"github.com/mailgun/vulcan/command"
	. "launchpad.net/gocheck"
	"net/http"
)

type JsSuite struct {
	Client *client.RecordingClient
}

var _ = Suite(&JsSuite{})

func (s *JsSuite) executeCode(request *http.Request, code string) (interface{}, error) {
	s.Client = &client.RecordingClient{}
	controller := &JsController{
		CodeGetter: &StringGetter{
			Code: code,
		},
		Client: s.Client,
	}
	return controller.GetInstructions(request)
}

func (s *JsSuite) expectReply(r *http.Request, code string) *command.Reply {
	replyI, err := s.executeCode(r, code)
	if err != nil {
		panic(err)
	}
	reply, ok := replyI.(*command.Reply)
	if !ok {
		panic(fmt.Errorf("Expected Reply, got: %T", replyI))
	}
	return reply
}

func (s *JsSuite) expectForward(r *http.Request, code string) *command.Forward {
	forwardI, err := s.executeCode(r, code)
	if err != nil {
		panic(err)
	}
	forward, ok := forwardI.(*command.Forward)
	if !ok {
		panic(fmt.Errorf("Expected Forward, got: %T", forwardI))
	}
	return forward
}

func (s *JsSuite) TestReturnReply(c *C) {
	reply := s.expectReply(
		NewTestRequest("GET", "http://localhost", nil),
		`function handle(request){return {code: 200, body: "OK"}}`,
	)
	c.Assert(reply.Code, Equals, 200)
	c.Assert(reply.Body, DeepEquals, "OK")
}

func (s *JsSuite) TestReturnForward(c *C) {
	forward := s.expectForward(
		NewTestRequest("GET", "http://localhost", nil),
		`function handle(request){return {upstreams: ["http://localhost:5000"]}}`,
	)
	c.Assert(
		forward.Upstreams,
		DeepEquals,
		[]*command.Upstream{NewTestUpstream("http://localhost:5000")})
}
