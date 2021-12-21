package tcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/suite"
)

const (
	poolSize = 1 << 20
	pcktSize = 63 << 10
)

// TestUnitTCPSuite go test 执行入口
func TestUnitTCPSuite(t *testing.T) {
	suite.Run(t, new(tcpSuite))
}

type tcpSuite struct {
	suite.Suite
}

// SetupTest 执行用例前初始化
func (s *tcpSuite) SetupTest() {
}

func (s *tcpSuite) TestIsRequest() {
	tests := []struct {
		name    string
		pckt    *Packet
		address string
		want    bool
	}{
		{
			name:    "success",
			pckt:    &Packet{TCP: &layers.TCP{}, DstIP: net.IP{127, 0, 0, 1}},
			address: "127.0.0.1:0",
			want:    true,
		},
		{
			name:    "fail",
			pckt:    &Packet{TCP: &layers.TCP{}, DstIP: net.IP{0, 0, 0, 0}},
			address: "127.0.0.1:0",
			want:    false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			res := IsRequest(tt.pckt, tt.address)
			s.Equal(tt.want, res)
		})
	}
}

func (s *tcpSuite) TestIsResponse() {
	tests := []struct {
		name    string
		pckt    *Packet
		address string
		want    bool
	}{
		{
			name:    "success",
			pckt:    &Packet{TCP: &layers.TCP{}, SrcIP: net.IP{127, 0, 0, 1}},
			address: "127.0.0.1:0",
			want:    true,
		},
		{
			name:    "fail",
			pckt:    &Packet{TCP: &layers.TCP{}, SrcIP: net.IP{0, 0, 0, 0}},
			address: "127.0.0.1:0",
			want:    false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			res := IsResponse(tt.pckt, tt.address)
			s.Equal(tt.want, res)
		})
	}
}

var decodeOpts = gopacket.DecodeOptions{Lazy: true, NoCopy: true}

func headersIP4(seq uint32, length uint16) (headers [54]byte) {
	// set ethernet headers
	binary.BigEndian.PutUint16(headers[12:14], uint16(layers.EthernetTypeIPv4))

	// set ip header
	ip := headers[14:]
	copy(ip[0:2], []byte{4<<4 | 5, 0x28<<2 | 0x00})
	binary.BigEndian.PutUint16(ip[2:4], length+40)
	ip[9] = uint8(layers.IPProtocolTCP)
	copy(ip[12:16], []byte{192, 168, 1, 2})
	copy(ip[16:], []byte{192, 168, 1, 3})

	// set tcp header
	tcp := ip[20:]
	binary.BigEndian.PutUint16(tcp[0:2], 45678)
	binary.BigEndian.PutUint16(tcp[2:4], 8001)
	tcp[12] = 5 << 4
	return
}

func GetPackets(start uint32, _len int, payload []byte) []gopacket.Packet {
	var packets = make([]gopacket.Packet, _len)
	for i := start; i < start+uint32(_len); i++ {
		data := make([]byte, 54+len(payload))
		h := headersIP4(i, uint16(len(payload)))
		copy(data, h[:])
		copy(data[len(h):], payload)
		packets[i-start] = gopacket.NewPacket(data, layers.LinkTypeEthernet, decodeOpts)
	}
	return packets
}

func exchangeIP(packets []gopacket.Packet) []gopacket.Packet {
	for _, packet := range packets {
		ip := packet.Data()[14:]
		var ip1 = make([]byte, 4)
		copy(ip1, ip[12:16])
		copy(ip[12:16], ip[16:20])
		copy(ip[16:20], ip1[0:4])

		var port = make([]byte, 2)
		tcp := ip[20:]
		copy(port, tcp[0:2])
		copy(tcp[0:2], tcp[2:4])
		copy(tcp[2:4], port)
	}

	return packets
}

func (s *tcpSuite) TestMessageParserWithHint() {
	s.Run("success", func() {
		var mssg = make(chan *Message, 3)
		pool := NewMessagePool(poolSize, time.Second, func(m *Message) { mssg <- m })
		packets := GetPackets(1, 30, nil)
		packets[0].Data()[14:][20:][13] = 2  // SYN flag
		packets[10].Data()[14:][20:][13] = 2 // SYN flag
		packets[29].Data()[14:][20:][13] = 1 // FIN flag

		str := "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n7"
		packets[4] = GetPackets(5, 1, []byte(str))[0]
		packets[5] = GetPackets(6, 1, []byte("\r\nMozilla\r\n9\r\nDeveloper\r"))[0]
		packets[6] = GetPackets(7, 1, []byte("\n7\r\nNetwork\r\n0\r\n\r\n"))[0]

		for i := 0; i < 30; i++ {
			pool.Handler(packets[i])
		}
		var m *Message
		select {
		case <-time.After(time.Second):
			s.Errorf(nil, "can't parse packets fast enough")
			return
		case m = <-mssg:
		}
		if !bytes.HasSuffix(m.Data(), []byte("\n7\r\nNetwork\r\n0\r\n\r\n")) {
			s.Errorf(nil, "expected to %q to have suffix %q", m.Data(), []byte("\n7\r\nNetwork\r\n0\r\n\r\n"))
		}
	})
}

func (s *tcpSuite) TestMessageParserWithoutHint() {
	s.Run("success", func() {
		var mssg = make(chan *Message, 1)
		var data [pcktSize]byte
		packets := GetPackets(1, 10, data[:])
		packets[0].Data()[14:][20:][13] = 2 // SYN flag
		packets[9].Data()[14:][20:][13] = 1 // FIN flag
		p := NewMessagePool(pcktSize*10, time.Second, func(m *Message) { mssg <- m })
		for _, v := range packets {
			p.Handler(v)
		}
		var m *Message
		select {
		case <-time.After(time.Second):
			s.Errorf(nil, "can't parse packets fast enough")
			return
		case m = <-mssg:
		}
		if m.Length != pcktSize*10 {
			s.Errorf(nil, "expected %d to equal %d", m.Length, pcktSize*10)
		}
	})

}

func (s *tcpSuite) TestMessageMaxSizeReached() {
	s.Run("success", func() {
		var mssg = make(chan *Message, 2)
		var data [pcktSize]byte
		packets := GetPackets(1, 2, data[:])
		packets = append(packets, GetPackets(3, 1, make([]byte, pcktSize+10))...)
		packets[0].Data()[14:][20:][13] = 2 // SYN flag
		packets[2].Data()[14:][20:][13] = 2 // SYN flag
		packets[2].Data()[14:][15] = 3      // changing address
		p := NewMessagePool(pcktSize+10, time.Second, func(m *Message) { mssg <- m })
		for _, v := range packets {
			p.Handler(v)
		}
		var m *Message
		select {
		case <-time.After(time.Second):
			s.Errorf(nil, "can't parse packets fast enough")
			return
		case m = <-mssg:
		}
		if m.Length != pcktSize+10 {
			s.Errorf(nil, "expected %d to equal %d", m.Length, pcktSize+10)
		}
		if !m.Truncated {
			s.Error(nil, "expected message to be truncated")
		}

		select {
		case <-time.After(time.Second):
			s.Errorf(nil, "can't parse packets fast enough")
			return
		case m = <-mssg:
		}
		if m.Length != pcktSize+10 {
			s.Errorf(nil, "expected %d to equal %d", m.Length, pcktSize+10)
		}
		if m.Truncated {
			s.Error(nil, "expected message to not be truncated")
		}
	})

}

func (s *tcpSuite) TestMessageTimeoutReached() {
	s.Run("success", func() {
		var mssg = make(chan *Message, 2)
		var data [pcktSize]byte
		packets := GetPackets(1, 2, data[:])
		packets[0].Data()[14:][20:][13] = 2 // SYN flag
		p := NewMessagePool(poolSize, 0, func(m *Message) { mssg <- m })
		p.Handler(packets[0])
		time.Sleep(time.Millisecond * 200)
		p.Handler(packets[1])
		m := <-mssg
		if m.Length != pcktSize<<1 {
			s.Errorf(nil, "expected %d to equal %d", m.Length, pcktSize<<1)
		}
		if !m.TimedOut {
			s.Error(nil, "expected message to be timeout")
		}
	})

}

func (s *tcpSuite) TestMessageUUID() {
	s.Run("success", func() {
		packets := GetPackets(1, 10, nil)
		packets[0].Data()[14:][20:][13] = 2 // SYN flag
		packets[4].Data()[14:][20:][13] = 1 // FIN flag
		packets[5].Data()[14:][20:][13] = 2 // SYN flag
		packets[9].Data()[14:][20:][13] = 1 // FIN flag
		var uuid, uuid1 []byte
		pool := NewMessagePool(0, 0, func(msg *Message) {
			if len(uuid) == 0 {
				uuid = msg.UUID()
				return
			}
			uuid1 = msg.UUID()
		})
		pool.MatchUUID(true)
		for _, p := range packets {
			pool.Handler(p)
		}

		if string(uuid) != string(uuid1) {
			s.Errorf(nil, "expected %s, to equal %s", uuid, uuid1)
		}
	})

}
