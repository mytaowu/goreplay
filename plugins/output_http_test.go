package plugins

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"goreplay/config"
)

type httpOutputSuite struct {
	suite.Suite
}

// TestUnitOutputHTTP httpOutput unit test suite
func TestUnitOutputHTTP(t *testing.T) {
	suite.Run(t, new(httpOutputSuite))
}

type plugData struct {
	name string
	msg  *Message
	size int
}

func preparePlugData() []plugData {
	return []plugData{
		{
			name: "success",
			msg: &Message{
				Meta: []byte("1"),
				Data: []byte("1"),
			},
			size: 2,
		},
		{
			name: "nil resp",
			msg: &Message{
				Meta: []byte("1"),
				Data: []byte("1"),
			},
			size: 2,
		},
	}
}
func (s *httpOutputSuite) TestPluginWrite() {
	for _, tt := range preparePlugData() {
		s.Run(tt.name, func() {
			output := NewHTTPOutput("123", &config.HTTPOutputConfig{
				SkipVerify: true,
				Stats:      true,
			})

			size, _ := output.PluginWrite(tt.msg)
			s.Equal(size, tt.size)

			_ = output.(*HTTPOutput).Close()
		})
	}
}

func (s *httpOutputSuite) TestPluginRead() {
	for _, tt := range preparePlugData() {
		s.Run(tt.name, func() {
			output := NewHTTPOutput("123", &config.HTTPOutputConfig{
				TrackResponses: true,
				WorkersMin:     1001,
				RedirectLimit:  -1,
			})

			go func() {
				output.(*HTTPOutput).responses <- &response{
					payload: tt.msg.Data,
				}
			}()

			msg, _ := output.PluginRead()
			s.Equal(msg.Data, tt.msg.Data)

			_ = output.(*HTTPOutput).Close()
		})
	}
}

func (s *httpOutputSuite) TestWorkerMaster() {
	for _, tt := range []struct {
		name   string
		output *HTTPOutput
	}{
		{
			name: "success",
			output: &HTTPOutput{
				Config: &config.HTTPOutputConfig{
					WorkerTimeout: time.Millisecond,
				},
			},
		},
	} {
		s.Run(tt.name, func() {
			go tt.output.workerMaster()
		})
	}
}

func (s *httpOutputSuite) TestSend() {
	for _, tt := range []struct {
		name  string
		error error
		conf  *config.HTTPOutputConfig
		data  []byte
	}{
		{
			name: "connect request",
			conf: &config.HTTPOutputConfig{
				OriginalHost: false,
				URL: &url.URL{
					Host:   "www.w3.org",
					Scheme: "1",
				},
			},
			data: []byte("CONNECT /post HTTP/1.1\nUser-Agent: Gor\nContent-Length: 7" +
				"\nHost: www.w3.org\n\na=1&b=2"),
			error: nil,
		},
		{
			name: "success",
			conf: &config.HTTPOutputConfig{
				OriginalHost: false,
				URL: &url.URL{
					Host:   "www.w3.org",
					Scheme: "http",
				},
			},
			data:  []byte("POST /post HTTP/1.1\nUser-Agent: Gor\nContent-Length: 7\nHost: www.w3.org\n\na=1&b=2"),
			error: nil,
		},
	} {
		s.Run(tt.name, func() {
			output := NewHTTPClient(tt.conf)

			_, err := output.Send(tt.data)
			s.Equal(err, tt.error)
		})
	}
}
