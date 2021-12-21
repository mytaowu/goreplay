package plugins

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"

	"goreplay/capture"
	"goreplay/config"
	"goreplay/protocol"
	"goreplay/tcp"
)

// TestUnitStat go test 执行入口
func TestUnitInputRaw(t *testing.T) {
	suite.Run(t, new(inputRawSuite))
}

type inputRawSuite struct {
	suite.Suite
	fakeErr error
}

// SetupTest 执行用例前初始化 client
func (s *inputRawSuite) SetupTest() {
	s.fakeErr = fmt.Errorf("fake error")
	gomonkey.
		ApplyFunc(log.Fatalf, func(string, ...interface{}) {}).
		ApplyFunc(log.Fatal, func(...interface{}) {})
}

func (s *inputRawSuite) TestNewRAWInput() {
	inputConfig := config.RAWInputConfig{Logreplay: true, Protocol: "grpc", SelectHost: "127.0.0.1"}

	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantErr bool
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&capture.Listener{}), "ListenBackground",
					func(l *capture.Listener, ctx context.Context, handler capture.Handler) chan error {
						err := make(chan error, 2)
						err <- s.fakeErr
						return err
					})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			patches := tt.prepare()
			defer patches.Reset()

			input := NewRAWInput("127.0.0.1:1", inputConfig)

			s.Equal(tt.wantErr, input == nil)
		})
	}
}

func (s *inputRawSuite) TestPluginRead() {
	var input *RAWInput

	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantErr bool
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				message := &tcp.Message{
					Stats: tcp.Stats{LostData: 1, Truncated: true,
						TimedOut: true, IsIncoming: true, SrcAddr: "127.0.0.1:1234"},
				}
				input.handler(message)
				return gomonkey.
					ApplyFunc(protocol.PayloadHeader, func(byte, []byte, int64, int64) []byte { return []byte("Meta") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "Data", func(*tcp.Message) []byte { return []byte("123") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "UUID", func(*tcp.Message) []byte { return []byte("UUID") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "ConnectionID", func(*tcp.Message) string { return "ID" })
			},
		},
		{
			name: "filter ip error",
			prepare: func() *gomonkey.Patches {
				message := &tcp.Message{
					Stats: tcp.Stats{LostData: 1, Truncated: true,
						TimedOut: true, IsIncoming: true, SrcAddr: "127.0.0.2:1234"},
				}
				input.handler(message)
				return gomonkey.
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "Data", func(*tcp.Message) []byte { return []byte("123") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "UUID", func(*tcp.Message) []byte { return []byte("UUID") })
			},
			wantErr: true,
		},
		{
			name: "record src ip",
			prepare: func() *gomonkey.Patches {
				message := &tcp.Message{
					Stats: tcp.Stats{LostData: 1, Truncated: true,
						TimedOut: true, IsIncoming: false, SrcAddr: "127.0.0.2:1234", DstAddr: "127.0.0.3:1234"},
				}
				input.handler(message)
				return gomonkey.
					ApplyFunc(protocol.PayloadHeader, func(byte, []byte, int64, int64) []byte { return []byte("Meta") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "Data", func(*tcp.Message) []byte { return []byte("123") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "UUID", func(*tcp.Message) []byte { return []byte("UUID") }).
					ApplyMethod(reflect.TypeOf(&tcp.Message{}), "ConnectionID", func(*tcp.Message) string { return "ID" })
			},
		},
		{
			name: "Quit",
			prepare: func() *gomonkey.Patches {
				_ = input.Close()
				return nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			input = &RAWInput{
				RAWInputConfig: config.RAWInputConfig{Stats: true},
				selectHostMap: map[string]bool{
					"127.0.0.1": true,
				},
			}
			input.message = make(chan *tcp.Message, 10)
			input.Quit = make(chan bool)
			input.cancelListener = func() {}
			input.messageStats = make([]tcp.Stats, 10010)

			patches := tt.prepare()
			if patches != nil {
				defer patches.Reset()
			}

			_, err := input.PluginRead()
			_ = input.String()
			_ = input.GetStats()

			s.Equal(tt.wantErr, err != nil)
		})
	}
}
