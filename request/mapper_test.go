package request

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestMapper(t *testing.T) { TestingT(t) }

type MapperSuite struct {
}

var _ = Suite(&MapperSuite{})

func (s *MapperSuite) TestVariableToMapper(c *C) {
	m, err := VariableToMapper("client.ip")
	c.Assert(err, IsNil)
	c.Assert(m, NotNil)

	m, err = VariableToMapper("request.host")
	c.Assert(err, IsNil)
	c.Assert(m, NotNil)

	m, err = VariableToMapper("request.header.X-Header-Name")
	c.Assert(err, IsNil)
	c.Assert(m, NotNil)

	m, err = VariableToMapper("rsom")
	c.Assert(err, NotNil)
	c.Assert(m, IsNil)
}
