package client

import (
	"errors"
	"io"
	"net"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"
)

// TestUnitTCPClientSuite go test 执行入口
func TestUnitTCPClientSuite(t *testing.T) {
	suite.Run(t, new(clientSuite))
}

type clientSuite struct {
	suite.Suite
	fakeErr error
}

// SetupTest 执行用例前初始化
func (s *clientSuite) SetupTest() {
	s.fakeErr = errors.New("fake error")
}

func (s *clientSuite) TestNewTCPClient() {
	s.Run("success", func() {
		client := NewTCPClient("addr", &TCPClientConfig{})

		s.Equal(false, client == nil)
	})
}

func (s *clientSuite) TestConnect() {
	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantLen int
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(net.DialTimeout, func(string, string, time.Duration) (net.Conn, error) {
					return &net.TCPConn{}, nil
				})
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.prepare != nil {
				patches := tt.prepare()
				if patches != nil {
					defer patches.Reset()
				}
			}
			client := &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{Secure: true}}
			err := client.Connect()

			s.Equal(false, err == nil)
		})
	}
}

func (s *clientSuite) TestIsAlive() {
	client := TCPClient{conn: &net.TCPConn{}}

	tests := []struct {
		name      string
		prepare   func() *gomonkey.Patches
		wantAlive bool
	}{
		{
			name:      "success",
			wantAlive: true,
		},
		{
			name: "alive",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Read",
					func(*net.TCPConn, []byte) (int, error) { return 0, nil })
			},
			wantAlive: true,
		},
		{
			name: "io.EOF",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Read",
					func(*net.TCPConn, []byte) (int, error) { return 0, io.EOF })
			},
			wantAlive: false,
		},
		{
			name: "syscall.EPIPE",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Read",
					func(*net.TCPConn, []byte) (int, error) { return 0, syscall.EPIPE })
			},
			wantAlive: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.prepare != nil {
				patches := tt.prepare()
				if patches != nil {
					defer patches.Reset()
				}
			}

			alive := client.isAlive()

			s.Equal(tt.wantAlive, alive)
		})
	}
}

func (s *clientSuite) TestDoConnect() {
	client := TCPClient{}

	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		wantErr bool
	}{
		{
			name: "error",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyMethod(reflect.TypeOf(&TCPClient{}), "Connect",
					func(*TCPClient) error { return s.fakeErr })
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.prepare != nil {
				patches := tt.prepare()
				if patches != nil {
					defer patches.Reset()
				}
			}

			err := client.doConnect()

			s.Equal(tt.wantErr, err != nil)
		})
	}
}

func (s *clientSuite) TestSend() {
	tests := []struct {
		name    string
		prepare func() *gomonkey.Patches
		client  *TCPClient
		wantErr bool
	}{
		{
			name: "success",
			prepare: func() *gomonkey.Patches {
				patches := gomonkey.ApplyPrivateMethod(reflect.TypeOf(&TCPClient{}), "doConnect",
					func(*TCPClient) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Write",
					func(*net.TCPConn, []byte) (int, error) { return 1, nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "CloseWrite",
					func(*net.TCPConn) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Read",
					func(*net.TCPConn, []byte) (int, error) { return 10, io.EOF })
				return patches
			},
			client:  &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{}, respBuf: []byte("12345")},
			wantErr: false,
		},
		{
			name: "error",
			prepare: func() *gomonkey.Patches {
				patches := gomonkey.ApplyPrivateMethod(reflect.TypeOf(&TCPClient{}), "doConnect",
					func(*TCPClient) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Write",
					func(*net.TCPConn, []byte) (int, error) { return 1, nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "CloseWrite",
					func(*net.TCPConn) error { return nil })
				return patches
			},
			client:  &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{}, respBuf: []byte("")},
			wantErr: true,
		},
		{
			name: "maxResponseSize",
			prepare: func() *gomonkey.Patches {
				patches := gomonkey.ApplyPrivateMethod(reflect.TypeOf(&TCPClient{}), "doConnect",
					func(*TCPClient) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Write",
					func(*net.TCPConn, []byte) (int, error) { return 1, nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "CloseWrite",
					func(*net.TCPConn) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Read",
					func(*net.TCPConn, []byte) (int, error) { return maxResponseSize + 10, nil })
				return patches
			},
			client:  &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{}, respBuf: []byte("")},
			wantErr: false,
		},
		{
			name: "doConnect error",
			prepare: func() *gomonkey.Patches {
				return gomonkey.ApplyPrivateMethod(reflect.TypeOf(&TCPClient{}), "doConnect",
					func(*TCPClient) error { return s.fakeErr })
			},
			client:  &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{}, respBuf: []byte("")},
			wantErr: true,
		},
		{
			name: "write error",
			prepare: func() *gomonkey.Patches {
				patches := gomonkey.ApplyPrivateMethod(reflect.TypeOf(&TCPClient{}), "doConnect",
					func(*TCPClient) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Write",
					func(*net.TCPConn, []byte) (int, error) { return 1, s.fakeErr })
				return patches
			},
			client:  &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{}, respBuf: []byte("")},
			wantErr: true,
		},
		{
			name: "close error",
			prepare: func() *gomonkey.Patches {
				patches := gomonkey.ApplyPrivateMethod(reflect.TypeOf(&TCPClient{}), "doConnect",
					func(*TCPClient) error { return nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "Write",
					func(*net.TCPConn, []byte) (int, error) { return 1, nil })
				patches.ApplyMethod(reflect.TypeOf(&net.TCPConn{}), "CloseWrite",
					func(*net.TCPConn) error { return s.fakeErr })
				return patches
			},
			client:  &TCPClient{conn: &net.TCPConn{}, config: &TCPClientConfig{}, respBuf: []byte("")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.prepare != nil {
				patches := tt.prepare()
				if patches != nil {
					defer patches.Reset()
				}
			}

			_, err := tt.client.Send([]byte{'1'})

			s.Equal(tt.wantErr, err != nil)
		})
	}
}
