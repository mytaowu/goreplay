//go:build linux
// +build linux

package capture

import (
	"net"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"goreplay/config"
)

// TestUnitLinuxCapture unit test execute
func TestUnitLinuxCapture(t *testing.T) {
	suite.Run(t, new(CaptureLinuxSuite))
}

// CaptureLinuxSuite capture unit test suite
type CaptureLinuxSuite struct {
	suite.Suite
}

// SetupTest init before test run
func (s *CaptureLinuxSuite) SetupTest() {
}

func (s *CaptureLinuxSuite) TestLinuxSocketHandler() {
	tests := []struct {
		name    string
		opts    config.PcapOptions
		mock    func() *gomonkey.Patches
		want    int
		wantErr bool
	}{
		{
			name: "success",
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(unix.SetsockoptPacketMreq, func(fd, level, opt int, mreq *unix.PacketMreq) error {
					return nil
				})
			},
			want:    5,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.mock()
			if patches != nil {
				defer patches.Reset()
			}

			l, err := NewListener(loopBack.Name, 8000, "", config.EngineRawSocket, true)
			if err != nil {
				s.T().Errorf("newListener error %v", err)
				return
			}

			ifi := NetInterface{}
			ifi.Interface = loopBack
			addrs, _ := loopBack.Addrs()
			ifi.IPs = make([]string, len(addrs))
			for j, addr := range addrs {
				ifi.IPs[j] = cutMask(addr)
			}

			// call activateRawSocket
			err = l.Activate()
			if err != nil {
				s.T().Errorf("expected error to be nil, got %v", err)
				return
			}

			defer func() {
				_ = l.Handles[loopBack.Name].(*SockRaw).Close()
			}()

			for i := 0; i < tt.want; i++ {
				_, _ = net.Dial("tcp", "127.0.0.1:8000")
			}
			sts, _ := l.Handles[loopBack.Name].(*SockRaw).Stats()
			if sts.Packets < uint32(tt.want) {
				s.T().Errorf("expected >=%d packets got %d", tt.want, sts.Packets)
			}
		})
	}
}
