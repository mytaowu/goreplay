package protocol

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/http2"

	"goreplay/framer"
	"goreplay/tcp"
)

func TestUnitGrpcSuite(t *testing.T) {
	suite.Run(t, new(GrpcSuite))
}

type GrpcSuite struct {
	suite.Suite
	fakeErr error
	packet  *tcp.Packet
}

func (s *GrpcSuite) SetupTest() {
	s.fakeErr = fmt.Errorf("fake error")
	s.packet = &tcp.Packet{SrcIP: []byte("1234"), DstIP: []byte("5678"), TCP: &layers.TCP{SrcPort: 80, DstPort: 90}}
}

func (s *GrpcSuite) TestMessageGroupBy() {
	pckt := s.packet
	fr := &http2.MetaHeadersFrame{HeadersFrame: &http2.HeadersFrame{FrameHeader: http2.FrameHeader{StreamID: 1}}}
	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantLen int
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				pckt.Payload = []byte("123")
				calledNum := 0
				return gomonkey.
					ApplyFunc(framer.ReEncodeMetaHeadersFrame,
						func(*http2.MetaHeadersFrame) ([]byte, error) { return []byte{}, nil }).
					ApplyMethod(reflect.TypeOf(&framer.HTTP2Framer{}), "ReadFrameAndBytes",
						func(*framer.HTTP2Framer) ([]byte, http2.Frame, error) {
							var err error
							calledNum++
							if calledNum >= 3 {
								err = s.fakeErr
							}
							return []byte{}, fr, err
						})
			},
			wantLen: 1,
		},
		{
			name: "pckt is nil",
			prepare: func() *gomonkey.Patches {
				pckt.Payload = []byte{}
				return nil
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.prepare()
			if patches != nil {
				defer patches.Reset()
			}

			builder := grpcFramerBuilder{}
			gFramer := builder.New("")
			m := gFramer.MessageGroupBy(pckt)

			s.Equal(tt.wantLen, len(m))
		})
	}
}

func (s *GrpcSuite) TestMessageKey() {
	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantKey *big.Int
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				return gomonkey.
					ApplyFunc(tcp.DefaultMessageKey,
						func(*tcp.Packet, bool) *big.Int { return big.NewInt(0) }).
					ApplyMethod(reflect.TypeOf(&http2.Framer{}), "ReadFrame",
						func(*http2.Framer) (http2.Frame, error) { return &http2.FrameHeader{StreamID: 1}, nil })
			},
			wantKey: big.NewInt(1),
		},
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				return gomonkey.
					ApplyFunc(tcp.DefaultMessageKey,
						func(*tcp.Packet, bool) *big.Int { return big.NewInt(0) }).
					ApplyMethod(reflect.TypeOf(&http2.Framer{}), "ReadFrame",
						func(*http2.Framer) (http2.Frame, error) { return nil, s.fakeErr })
			},
			wantKey: big.NewInt(0),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.prepare()
			defer patches.Reset()

			builder := grpcFramerBuilder{}
			gFramer := builder.New("")

			key := gFramer.MessageKey(s.packet, false)
			s.Equal(tt.wantKey, key)
		})
	}
}

func (s *GrpcSuite) TestStart() {
	pckt := &tcp.Packet{TCP: &layers.TCP{}}
	builder := grpcFramerBuilder{}
	gFramer := builder.New("")

	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wamtIn  bool
	}{
		{
			name: "success in",
			prepare: func() *gomonkey.Patches {
				pckt.Payload = []byte("123")
				return gomonkey.
					ApplyMethod(reflect.TypeOf(&tcp.CommonFramer{}), "InOut",
						func(*tcp.CommonFramer, *tcp.Packet) (bool, bool) { return true, false }).
					ApplyMethod(reflect.TypeOf(&http2.Framer{}), "ReadFrame",
						func(*http2.Framer) (http2.Frame, error) { return &http2.FrameHeader{StreamID: 1}, nil })
			},
			wamtIn: true,
		},
		{
			name: "success out",
			prepare: func() *gomonkey.Patches {
				pckt.Payload = []byte("123")
				return gomonkey.
					ApplyMethod(reflect.TypeOf(&tcp.CommonFramer{}), "InOut",
						func(*tcp.CommonFramer, *tcp.Packet) (bool, bool) { return false, true }).
					ApplyMethod(reflect.TypeOf(&http2.Framer{}), "ReadFrame",
						func(*http2.Framer) (http2.Frame, error) { return &http2.FrameHeader{StreamID: 1}, nil })
			},
			wamtIn: false,
		},
		{
			name: "empty payload",
			prepare: func() *gomonkey.Patches {
				pckt.Payload = []byte{}
				return nil
			},
			wamtIn: false,
		},
		{
			name: "readFrame error",
			prepare: func() *gomonkey.Patches {
				pckt.Payload = []byte("123")
				return gomonkey.
					ApplyMethod(reflect.TypeOf(&tcp.CommonFramer{}), "InOut",
						func(*tcp.CommonFramer, *tcp.Packet) (bool, bool) { return false, true }).
					ApplyMethod(reflect.TypeOf(&http2.Framer{}), "ReadFrame",
						func(*http2.Framer) (http2.Frame, error) { return nil, s.fakeErr })
			},
			wamtIn: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.prepare()
			if patches != nil {
				defer patches.Reset()
			}

			in, _ := gFramer.Start(pckt)
			s.Equal(tt.wamtIn, in)
		})
	}

}

func (s *GrpcSuite) TestEnd() {
	builder := grpcFramerBuilder{}
	gFramer := builder.New("")
	msg := &tcp.Message{}

	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantEnd bool
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				msg.Length = 20
				return gomonkey.ApplyMethod(reflect.TypeOf(&tcp.Message{}), "Packets",
					func(*tcp.Message) []*tcp.Packet {
						return []*tcp.Packet{{TCP: &layers.TCP{BaseLayer: layers.BaseLayer{Payload: []byte("123")}}}}
					})
			},
			wantEnd: false,
		},
		{
			name: "payload is empty",
			prepare: func() *gomonkey.Patches {
				msg.Length = 20
				return gomonkey.ApplyMethod(reflect.TypeOf(&tcp.Message{}), "Packets",
					func(*tcp.Message) []*tcp.Packet { return []*tcp.Packet{{TCP: &layers.TCP{}}} })
			},
			wantEnd: false,
		},
		{
			name: "msg.Length is zero",
			prepare: func() *gomonkey.Patches {
				msg.Length = 0
				return nil
			},
			wantEnd: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.prepare()
			if patches != nil {
				defer patches.Reset()
			}

			end := gFramer.End(msg)

			s.Equal(tt.wantEnd, end)
		})
	}

}
