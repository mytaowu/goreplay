package capture

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/stretchr/testify/suite"

	"goreplay/config"
)

var ifts, _ = net.Interfaces()
var loopBack = func() net.Interface {
	for _, v := range ifts {
		if v.Flags&net.FlagLoopback != 0 {
			return v
		}
	}
	return ifts[0]
}()

const (
	testPort = 18899
	pbfFiler = "dst port 18899 and host 127.0.0.1"
)

// used to benchmark sock engine
var buf [1024]byte

func init() {
	for i := 0; i < len(buf); i++ {
		buf[i] = 0xff
	}
}

// TestUnitCapture unit test execute
func TestUnitCapture(t *testing.T) {
	suite.Run(t, new(CaptureSuite))
}

// CaptureSuite capture unit test suite
type CaptureSuite struct {
	suite.Suite
}

func (s *CaptureSuite) SetupTest() {
}

func (s *CaptureSuite) TestNewListener() {
	tests := []struct {
		name      string
		engine    config.EngineType
		host      string
		transport string
		mock      func() *gomonkey.Patches
		wantErr   bool
	}{
		{
			name:      "libpcap",
			engine:    config.EnginePcap,
			host:      "127.0.0.1",
			transport: "",
			wantErr:   false,
		},
		{
			name:      "pcap_file",
			engine:    config.EnginePcapFile,
			host:      "",
			transport: "",
			wantErr:   false,
		},
		{
			name:      "raw_socket",
			engine:    config.EngineRawSocket,
			host:      "",
			transport: "udp",
			wantErr:   false,
		},
		{
			name:      "fail",
			engine:    config.EnginePcap,
			host:      "",
			transport: "",
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(net.Interfaces, func() ([]net.Interface, error) {
					return nil, fmt.Errorf("op error")
				})
			},
			wantErr: true,
		},
	}

	n := new(int32)
	counter := new(int32)

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.mock != nil {
				patches := tt.mock()
				if patches != nil {
					defer patches.Reset()
				}
			}

			l, err := NewListener(tt.host, uint16(8000), tt.transport, tt.engine, true)
			s.Equal(tt.wantErr, err != nil)

			s.T().Logf("listener: %v", l)

			if l != nil {
				l.SetPcapOptions(config.PcapOptions{})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				l.ListenBackground(ctx, handler(n, counter))
			}
		})
	}
}

func (s *CaptureSuite) TestSetInterfaces() {
	tests := []struct {
		name    string
		host    string
		mock    func() *gomonkey.Patches
		wantErr bool
	}{
		{
			name:    "loopback",
			host:    "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "hardware addr",
			host:    loopBack.HardwareAddr.String(),
			wantErr: false,
		},
		{
			name:    "empty host",
			host:    "",
			wantErr: false,
		},
		{
			name:    "not found error",
			host:    "127.0.0.99",
			wantErr: true,
		},
		{
			name: "get net interfaces error",
			host: "",
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(net.Interfaces, func() ([]net.Interface, error) {
					return nil, fmt.Errorf("op error")
				})
			},
			wantErr: true,
		},
		{
			name:    "loopBack device",
			host:    loopBack.Name,
			wantErr: false,
		},
		{
			name: "interface addrs error",
			host: "",
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&net.Interface{}), "Addrs",
					func(*net.Interface) ([]net.Addr, error) {
						return nil, fmt.Errorf("addrs error")
					})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.mock != nil {
				patches := tt.mock()
				if patches != nil {
					defer patches.Reset()
				}
			}

			l := &Listener{}
			l.host = tt.host
			err := l.setInterfaces()

			s.T().Logf("interfaces len: %d, loopBack: %v", len(l.Interfaces), loopBack.Name)
			s.Equal(tt.wantErr, err != nil)
		})
	}
}

func (s *CaptureSuite) TestBPFFilter() {
	s.Run("filter", func() {
		l := &Listener{}
		l.host = "127.0.0.1"
		l.Transport = "tcp"
		_ = l.setInterfaces()
		filter := l.Filter(l.Interfaces[0])
		if filter != "(tcp dst portrange 0-65535 and host 127.0.0.1)" {
			s.T().Error("wrong filter", filter)
		}
		l.port = 8000
		l.trackResponse = true
		filter = l.Filter(l.Interfaces[0])
		if filter != "(tcp port 8000 and host 127.0.0.1)" {
			s.T().Error("wrong filter")
		}
	})
}

var decodeOpts = gopacket.DecodeOptions{Lazy: true, NoCopy: true}

func generateHeaders(seq uint32, length uint16) (headers [44]byte) {
	// set ethernet headers
	binary.BigEndian.PutUint32(headers[0:4], uint32(layers.ProtocolFamilyIPv4))

	// set ip header
	ip := headers[4:]
	copy(ip[0:2], []byte{4<<4 | 5, 0x28<<2 | 0x00})
	binary.BigEndian.PutUint16(ip[2:4], length+54)
	ip[9] = uint8(layers.IPProtocolTCP)
	copy(ip[12:16], []byte{127, 0, 0, 1})
	copy(ip[16:], []byte{127, 0, 0, 1})

	// set tcp header
	tcp := ip[20:]
	binary.BigEndian.PutUint16(tcp[0:2], 45678)
	binary.BigEndian.PutUint16(tcp[2:4], 8000)
	tcp[12] = 5 << 4
	return
}

func randomPackets(start uint32, _len int, length uint16) []gopacket.Packet {
	var packets = make([]gopacket.Packet, _len)
	for i := start; i < start+uint32(_len); i++ {
		h := generateHeaders(i, length)
		d := make([]byte, int(length)+len(h))
		copy(d, h[0:])
		packet := gopacket.NewPacket(d, layers.LinkTypeLoop, decodeOpts)
		packets[i-start] = packet
		inf := packets[i-start].Metadata()
		_len := len(d)
		inf.CaptureInfo = gopacket.CaptureInfo{CaptureLength: _len, Length: _len, Timestamp: time.Now()}
	}
	return packets
}

func (s *CaptureSuite) TestPcapDumpHandler() {
	tests := []struct {
		name     string
		linkType layers.LinkType
		mock     func() *gomonkey.Patches
	}{
		{
			name:     "success",
			linkType: layers.LinkTypeLoop,
		},
		{
			name:     "error",
			linkType: layers.LinkTypeEthernet,
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&Writer{}), "WriteFileHeader",
					func(*Writer, uint32, layers.LinkType) error {
						return fmt.Errorf("writer file handle error")
					})
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.mock != nil {
				patches := tt.mock()
				if patches != nil {
					defer patches.Reset()
				}
			}
			f, err := ioutil.TempFile("", "pcap_file")
			if err != nil {
				s.T().Error(err)
			}

			h, err := PcapDumpHandler(f, tt.linkType)
			if err != nil {
				return
			}
			packets := randomPackets(1, 5, 5)
			for i := 0; i < len(packets); i++ {
				if i == 1 {
					tcp := packets[i].Data()[4:][20:]
					// change dst port
					binary.BigEndian.PutUint16(tcp[2:], 8001)
				}
				if i == 4 {
					inf := packets[i].Metadata()
					inf.CaptureLength = 40
				}
				h(packets[i])
			}

			f.Close()

			name := f.Name()
			testPcapDumpEngine(name, s.T())
		})
	}
}

func testPcapDumpEngine(f string, t *testing.T) {
	defer os.Remove(f)
	l, err := NewListener(f, 8000, "", config.EnginePcapFile, true)
	if err != nil {
		t.Errorf("newListener error %v", err)
		return
	}

	err = l.Activate()
	if err != nil {
		t.Errorf("expected error to be nil, got %q", err)
		return
	}
	pckts := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = l.Listen(ctx, func(packet gopacket.Packet) {
		if packet.Metadata().CaptureLength != 49 {
			t.Errorf("expected packet length to be %d, got %d", 49, packet.Metadata().CaptureLength)
		}
		pckts++
	})

	if err != nil {
		t.Errorf("expected error to be nil, got %q", err)
	}
	if pckts != 3 {
		t.Errorf("expected %d packets, got %d packets", 3, pckts)
	}
}

type pcapHandleTest struct {
	name    string
	opts    config.PcapOptions
	mock    func() *gomonkey.Patches
	want    int
	wantErr bool
}

func prepareTestCasesForPacpHandle() []pcapHandleTest {
	return []pcapHandleTest{
		{
			name: "success",
			opts: config.PcapOptions{
				TimestampType: "PCAP_TSTAMP_HOST",
				Promiscuous:   true,
				Monitor:       true,
				Snaplen:       true,
				// https://stackoverflow.com/questions/11397367/issue-in-pcap-set-buffer-size
				BufferSize: 2097152, // default 2097152
				BPFFilter:  pbfFiler,
			},
			mock: func() *gomonkey.Patches {
				go func() {
					_ = http.ListenAndServe(":"+strconv.Itoa(testPort), nil)
				}()
				patches := gomonkey.ApplyFunc(pcap.TimestampSourceFromString, func(string) (pcap.TimestampSource, error) {
					return pcap.TimestampSource(1), nil
				})
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetTimestampSource",
					func(*pcap.InactiveHandle, pcap.TimestampSource) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetPromisc",
					func(*pcap.InactiveHandle, bool) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetRFMon",
					func(*pcap.InactiveHandle, bool) error { return nil })
				return patches
			},
			want:    5,
			wantErr: false,
		},
		{
			name: "timestamptype error",
			opts: config.PcapOptions{
				TimestampType: "PCAP_TSTAMP_HOST",
				Promiscuous:   true,
				Monitor:       true,
			},
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(pcap.TimestampSourceFromString, func(string) (pcap.TimestampSource, error) {
					return pcap.TimestampSource(1), fmt.Errorf("timestamptype error")
				})
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "timestamps error",
			opts: config.PcapOptions{
				TimestampType: "PCAP_TSTAMP_HOST",
				Promiscuous:   true,
				Monitor:       true,
			},
			mock: func() *gomonkey.Patches {
				patches := gomonkey.ApplyFunc(pcap.TimestampSourceFromString, func(string) (pcap.TimestampSource, error) {
					return pcap.TimestampSource(1), nil
				})
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetTimestampSource",
					func(*pcap.InactiveHandle, pcap.TimestampSource) error { return fmt.Errorf("timestamps error") })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetPromisc",
					func(*pcap.InactiveHandle, bool) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetRFMon",
					func(*pcap.InactiveHandle, bool) error { return nil })
				return patches
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "promiscuous error",
			opts: config.PcapOptions{
				TimestampType: "PCAP_TSTAMP_HOST",
				Promiscuous:   true,
				Monitor:       true,
			},
			mock: func() *gomonkey.Patches {
				patches := gomonkey.ApplyFunc(pcap.TimestampSourceFromString, func(string) (pcap.TimestampSource, error) {
					return pcap.TimestampSource(1), nil
				})
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetTimestampSource",
					func(*pcap.InactiveHandle, pcap.TimestampSource) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetPromisc",
					func(*pcap.InactiveHandle, bool) error { return fmt.Errorf("promiscuous error") })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetRFMon",
					func(*pcap.InactiveHandle, bool) error { return nil })
				return patches
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "monitor error",
			opts: config.PcapOptions{
				TimestampType: "PCAP_TSTAMP_HOST",
				Promiscuous:   true,
				Monitor:       true,
			},
			mock: func() *gomonkey.Patches {
				patches := gomonkey.ApplyFunc(pcap.TimestampSourceFromString, func(string) (pcap.TimestampSource, error) {
					return pcap.TimestampSource(1), nil
				})
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetTimestampSource",
					func(*pcap.InactiveHandle, pcap.TimestampSource) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetPromisc",
					func(*pcap.InactiveHandle, bool) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&pcap.InactiveHandle{}), "SetRFMon",
					func(*pcap.InactiveHandle, bool) error { return fmt.Errorf("monitor error") })
				return patches
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "inactive error",
			opts: config.PcapOptions{},
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(pcap.NewInactiveHandle, func(string) (*pcap.InactiveHandle, error) {
					return nil, fmt.Errorf("inactive fake error")
				})
			},
			want:    0,
			wantErr: true,
		},
	}
}
func (s *CaptureSuite) TestPcapHandle() {
	tests := prepareTestCasesForPacpHandle()

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.mock != nil {
				patches := tt.mock()
				if patches != nil {
					defer patches.Reset()
				}
			}

			l, err := NewListener(loopBack.Name, testPort, "", config.EnginePcap, true)
			if err != nil {
				s.T().Errorf("expected error to be nil, got %v", err)
				return
			}
			l.SetPcapOptions(tt.opts)

			// call activatePcap
			err = l.Activate()
			s.T().Logf("activatePcap error: %v", err)

			s.Equal(tt.wantErr, err != nil)

			if err != nil {
				return
			}

			defer l.Handles[loopBack.Name].(*pcap.Handle).Close()

			for i := 0; i < tt.want; i++ {
				_, _ = net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(testPort))
			}
			sts, _ := l.Handles[loopBack.Name].(*pcap.Handle).Stats()
			if sts.PacketsReceived < tt.want {
				s.T().Errorf("expected >=%d packets got %d", tt.want, sts.PacketsReceived)
			}
		})
	}
}

func (s *CaptureSuite) TestSocketHandler() {
	s.Run("other", func() {

	})
}

func BenchmarkPcapDump(b *testing.B) {
	f, err := ioutil.TempFile("", "pcap_file")
	if err != nil {
		b.Error(err)
		return
	}
	now := time.Now()
	defer os.Remove(f.Name())
	h, _ := PcapDumpHandler(f, layers.LinkTypeLoop)
	packets := randomPackets(1, b.N, 5)
	for i := 0; i < len(packets); i++ {
		h(packets[i])
	}
	f.Close()
	b.Logf("%d packets in %s", b.N, time.Since(now))
}

func BenchmarkPcapFile(b *testing.B) {
	f, err := ioutil.TempFile("", "pcap_file")
	if err != nil {
		b.Error(err)
		return
	}
	defer os.Remove(f.Name())
	h, _ := PcapDumpHandler(f, layers.LinkTypeLoop)
	packets := randomPackets(1, b.N, 5)
	for i := 0; i < len(packets); i++ {
		h(packets[i])
	}
	name := f.Name()
	f.Close()
	b.ResetTimer()
	var l *Listener
	l, err = NewListener(name, 8000, "", config.EnginePcapFile, true)
	if err != nil {
		b.Error(err)
		return
	}
	err = l.Activate()
	if err != nil {
		b.Error(err)
		return
	}
	now := time.Now()
	pckts := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = l.Listen(ctx, func(packet gopacket.Packet) {
		if packet.Metadata().CaptureLength != 49 {
			b.Errorf("expected packet length to be %d, got %d", 49, packet.Metadata().CaptureLength)
		}
		pckts++
	}); err != nil {
		b.Error(err)
	}
	b.Logf("%d/%d packets in %s", pckts, b.N, time.Since(now))
}

func handler(n, counter *int32) Handler {
	return func(p gopacket.Packet) {
		nn := int32(len(p.Data()))
		atomic.AddInt32(n, nn)
		atomic.AddInt32(counter, 1)
	}
}

func BenchmarkPcap(b *testing.B) {
	var err error
	n := new(int32)
	counter := new(int32)
	l, err := NewListener(loopBack.Name, 8000, "", config.EnginePcap, false)
	if err != nil {
		b.Error(err)
		return
	}
	l.PcapOptions.BPFFilter = pbfFiler
	err = l.Activate()
	if err != nil {
		b.Error(err)
		return
	}
	errCh := l.ListenBackground(context.Background(), handler(n, counter))
	select {
	case <-l.Reading:
	case err = <-errCh:
		b.Error(err)
		return
	}
	var conn net.Conn
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		conn, err = net.Dial("udp", "127.0.0.1:8000")
		if err != nil {
			b.Error(err)
			return
		}
		b.StartTimer()
		_, err = conn.Write(buf[:])
		if err != nil {
			b.Error(err)
			return
		}
	}
	b.ReportMetric(float64(atomic.LoadInt32(n)), "buf")
	b.ReportMetric(float64(atomic.LoadInt32(counter)), "packets")
}

func BenchmarkRawSocket(b *testing.B) {
	var err error
	n := new(int32)
	counter := new(int32)
	l, err := NewListener(loopBack.Name, 8000, "", config.EngineRawSocket, false)
	if err != nil {
		b.Error(err)
		return
	}
	l.PcapOptions.BPFFilter = pbfFiler
	err = l.Activate()
	if err != nil {
		b.Error(err)
		return
	}
	errCh := l.ListenBackground(context.Background(), handler(n, counter))
	select {
	case <-l.Reading:
	case err = <-errCh:
		b.Error(err)
		return
	}
	var conn net.Conn
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		conn, err = net.Dial("udp", "127.0.0.1:8000")
		if err != nil {
			b.Error(err)
			return
		}
		b.StartTimer()
		_, err = conn.Write(buf[:])
		if err != nil {
			b.Error(err)
			return
		}
	}
	b.ReportMetric(float64(atomic.LoadInt32(n)), "buf")
	b.ReportMetric(float64(atomic.LoadInt32(counter)), "packets")
}
