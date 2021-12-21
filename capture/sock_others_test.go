// +build !linux

package capture

import (
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
)

type otherSockSuite struct {
	suite.Suite
}

func TestOtherSock(t *testing.T) {
	suite.Run(t, new(otherSockSuite))
}

func (s *otherSockSuite) TestNewSocket() {
	s.Run("new", func() {
		sock, err := NewSocket(net.Interface{})

		s.Equal(nil, sock)
		s.Equal(true, err != nil)
	})
}
