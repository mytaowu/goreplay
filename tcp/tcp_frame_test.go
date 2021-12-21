package tcp

import (
	"net"
	"reflect"
	"testing"

	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/suite"
)

// TestUnitTCPFrameSuite go test 执行入口
func TestUnitTCPFrameSuite(t *testing.T) {
	suite.Run(t, new(tcpFrameSuite))
}

type tcpFrameSuite struct {
	suite.Suite
}

// SetupTest 执行用例前初始化
func (s *tcpFrameSuite) SetupTest() {
}

// httpFramerBuilder http framer builder for test
type httpFramerBuilder struct{}

func (fb *httpFramerBuilder) New(listenAddr string) Framer {
	return nil
}

func (s *tcpFrameSuite) TestReqRspKey() {
	commFramer := CommonFramer{"127.0.0.1:0"}
	tmpPacket := Packet{TCP: &layers.TCP{}, SrcIP: net.IP{127, 0, 0, 1}, DstIP: net.IP{127, 0, 0, 1}}
	tests := []struct {
		name string
		pckt *Packet
		want string
	}{
		{
			name: "success",
			pckt: &tmpPacket,
			want: "2130706433",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ret := commFramer.ReqRspKey(tt.pckt)
			s.Equal(tt.want, ret)
		})
	}
}

func (s *tcpFrameSuite) TestMessageKey() {
	commFramer := CommonFramer{"127.0.0.1:0"}
	tmpPacket := Packet{TCP: &layers.TCP{}, SrcIP: net.IP{127, 0, 0, 1}, DstIP: net.IP{127, 0, 0, 1}}
	tests := []struct {
		name  string
		pckt  *Packet
		compr bool
		want  string
	}{
		{
			name:  "success",
			pckt:  &tmpPacket,
			compr: true,
			want:  "2130706433",
		},
		{
			name:  "fail",
			pckt:  &tmpPacket,
			compr: false,
			want:  "2130706433",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			commFramer.ListenAddr = "127.0.0.1:0"
			ret := commFramer.MessageKey(tt.pckt, tt.compr)
			s.Equal(tt.want, ret.String())
		})
	}
}

func (s *tcpFrameSuite) TestMessageGroupBy() {
	commFramer := CommonFramer{"127.0.0.1:0"}
	tmpPacket := Packet{TCP: &layers.TCP{}, SrcIP: net.IP{127, 0, 0, 1}, DstIP: net.IP{127, 0, 0, 1}}
	tests := []struct {
		name   string
		pckt   *Packet
		result *Packet
		key    string
		want   bool
	}{
		{
			name:   "success",
			pckt:   &tmpPacket,
			result: &tmpPacket,
			key:    "2130706433",
			want:   true,
		},
		{
			name:   "fail",
			pckt:   &tmpPacket,
			result: nil,
			key:    "2130706433",
			want:   false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ret := commFramer.MessageGroupBy(tt.pckt)
			s.Equal(tt.want, tt.result == ret[tt.key])
		})
	}
}

func (s *tcpFrameSuite) TestIsStart() {
	commFramer := CommonFramer{"127.0.0.1:0"}
	tests := []struct {
		name string
		pckt *Packet
		want bool
	}{
		{
			name: "success_in",
			pckt: &Packet{TCP: &layers.TCP{}, SrcIP: net.IP{127, 0, 0, 1}, DstIP: net.IP{127, 0, 0, 1}},
			want: true,
		},
		{
			name: "success_out",
			pckt: &Packet{TCP: &layers.TCP{}, SrcIP: net.IP{127, 0, 0, 1}, DstIP: net.IP{127, 0, 0, 2}},
			want: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ret := commFramer.IsStart(tt.pckt)
			s.Equal(tt.want, ret)
		})
	}
}

func (s *tcpFrameSuite) TestCheckLength() {
	commFramer := CommonFramer{"127.0.0.1:0"}
	tests := []struct {
		name   string
		seq    []byte
		length int
		want   bool
	}{
		{
			name:   "upper",
			seq:    []byte{1, 2, 3, 4, 5, 6, 7, 8},
			length: 8,
			want:   true,
		},
		{
			name:   "lower",
			seq:    []byte{1, 2, 3, 4, 5, 6, 7, 8},
			length: 9,
			want:   false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			res := commFramer.CheckLength(tt.seq, tt.length)
			s.Equal(tt.want, res)
		})
	}
}

func TestDefaultMessageKey(t *testing.T) {
	tests := []struct {
		name    string
		srcPckt *Packet
		dstPckt *Packet
	}{
		{
			name: "req packet",
			srcPckt: &Packet{
				SrcIP: net.IPv4(8, 8, 8, 8),
				DstIP: net.IPv4(9, 9, 9, 9),
				TCP: &layers.TCP{
					SrcPort: 8888,
					DstPort: 9999,
				},
			},
			dstPckt: &Packet{
				SrcIP: net.IPv4(9, 9, 9, 9),
				DstIP: net.IPv4(8, 8, 8, 8),
				TCP: &layers.TCP{
					SrcPort: 9999,
					DstPort: 8888,
				},
			},
		},

		{
			name: "key",
			srcPckt: &Packet{
				SrcIP: net.IPv4(9, 218, 40, 153),
				DstIP: net.IPv4(100, 117, 161, 46),
				TCP: &layers.TCP{
					SrcPort: 55684,
					DstPort: 11109,
				},
			},
			dstPckt: &Packet{
				SrcIP: net.IPv4(100, 117, 161, 46),
				DstIP: net.IPv4(9, 218, 40, 153),
				TCP: &layers.TCP{
					SrcPort: 11109,
					DstPort: 55684,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqKey := DefaultMessageKey(tt.srcPckt, false)
			t.Logf("reqKey: %v", reqKey)

			rspKey := DefaultMessageKey(tt.dstPckt, true)
			t.Logf("rspKey: %v", rspKey)

			if !reflect.DeepEqual(rspKey, reqKey) {
				t.Errorf("rspKey %v not equals reqKey %v", rspKey, reqKey)
			}
		})
	}
}
