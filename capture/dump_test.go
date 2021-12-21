package capture

import (
	"bytes"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/suite"
)

type dumpSuite struct {
	suite.Suite
}

// SetupTest init before test run
func (s *dumpSuite) SetupTest() {
}

func TestUnitDump(t *testing.T) {
	suite.Run(t, new(dumpSuite))
}

func (s *dumpSuite) TestNewWriter() {
	data := []byte("123")

	tests := []struct {
		name string
		pack gopacket.CaptureInfo
		mock func()
	}{
		{
			name: "success",
			pack: gopacket.CaptureInfo{
				Length:        len(data),
				CaptureLength: len(data),
			},
			mock: func() {
			},
		},
		{
			name: "capture length not equal data length",
			pack: gopacket.CaptureInfo{
				Length:        len(data) - 1,
				CaptureLength: len(data),
			},
			mock: func() {
			},
		},
		{
			name: "capture length greater than data length",
			pack: gopacket.CaptureInfo{
				Length:        len(data) - 1,
				CaptureLength: len(data),
			},
			mock: func() {
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {

			w := NewWriter(bytes.NewBuffer([]byte("abc")))
			s.Equal(true, w != nil)

			_ = w.WriteFileHeader(uint32(10), layers.LinkTypeEthernet)

			_ = w.WritePacket(tt.pack, data)
		})
	}
}
