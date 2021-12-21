// +build linux

package capture

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type linuxSockSuite struct {
	suite.Suite
}

func TestUnitLinuxSock(t *testing.T) {
	suite.Run(t, new(linuxSockSuite))
}

func (s *linuxSockSuite) TestReadPacketData() {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "success",
			data: []byte("data"),
		},
	}

	for _, t := range tests {
		s.Run(t.name, func() {
			sock, _ := NewSocket(loopBack)
			_ = sock.SetSnapLen(1 << 2)
			sock.SetLoopbackIndex(int32(1))
			_ = sock.SetPromiscuous(true)
			_ = sock.SetTimeout(time.Second)

			_ = sock.WritePacketData(t.data)

			d, _, _ := sock.ReadPacketData()

			s.T().Logf("read data: %v", string(d))
		})
	}
}

func (s *linuxSockSuite) TestSetSnapLen() {
	tests := []struct {
		name string
		snap int
		want int
	}{
		{
			name: "success",
			snap: 1 << 2,
			want: 1 << 2,
		},
		{
			name: "large",
			snap: BLOCKSIZE + 1,
			want: BLOCKSIZE,
		},
		{
			name: "negative",
			snap: -1,
			want: BLOCKSIZE,
		},
	}

	for _, t := range tests {
		s.Run(t.name, func() {
			sock, _ := NewSocket(loopBack)
			_ = sock.SetSnapLen(t.snap)

			snap := sock.GetSnapLen()
			s.Equal(t.want, snap)
		})
	}
}
