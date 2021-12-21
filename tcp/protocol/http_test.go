package protocol

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/suite"

	"goreplay/proto"
	"goreplay/tcp"
)

func TestUnitHttpSuite(t *testing.T) {
	suite.Run(t, new(HTTPSuite))
}

type HTTPSuite struct {
	suite.Suite
}

func (s *HTTPSuite) SetupTest() {
}

func (s *HTTPSuite) TestStart() {
	pckt := &tcp.Packet{TCP: &layers.TCP{BaseLayer: layers.BaseLayer{Payload: []byte("123")}}}
	builder := httpFramerBuilder{}
	framer := builder.New("")

	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantIn  bool
	}{
		{
			name: "start",
			prepare: func() *gomonkey.Patches {
				return gomonkey.
					ApplyFunc(proto.HasRequestTitle, func(payload []byte) bool { return false }).
					ApplyFunc(proto.HasResponseTitle, func(payload []byte) bool { return false })
			},
			wantIn: false,
		},
		{
			name: "isIncoming",
			prepare: func() *gomonkey.Patches {
				return gomonkey.
					ApplyFunc(proto.HasRequestTitle, func(payload []byte) bool { return true })
			},
			wantIn: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.prepare()
			defer patches.Reset()

			in, _ := framer.Start(pckt)

			s.Equal(tt.wantIn, in)
		})
	}

}

func (s *HTTPSuite) TestEnd() {
	builder := httpFramerBuilder{}
	framer := builder.New("")
	msg := tcp.NewMessage("", "", 0, 0)

	s.Run("end", func() {
		patches := gomonkey.ApplyFunc(proto.HasFullPayload, func([]byte, proto.Feedback) bool { return true })
		defer patches.Reset()

		end := framer.End(msg)

		s.Equal(true, end)
	})
}
