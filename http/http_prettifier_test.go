package http

import (
	"bytes"
	"compress/gzip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

type httpPrettifierSuite struct {
	suite.Suite
}

func TestUnitHTTPPrettifier(t *testing.T) {
	suite.Run(t, new(httpPrettifierSuite))
}

func (s *httpPrettifierSuite) TestHTTPPrettifier() {
	for _, tt := range []struct {
		name    string
		payload func() []byte
		want    []byte
	}{
		{
			name: "gzip success",
			payload: func() []byte {
				b := bytes.NewBufferString("")
				w := gzip.NewWriter(b)
				_, _ = w.Write([]byte("test"))
				_ = w.Close()

				size := strconv.Itoa(len(b.Bytes()))
				payload := []byte("HTTP/1.1 200 OK\r\nContent-Length: " + size + "\r\nContent-Encoding: gzip\r\n\r\n")
				payload = append(payload, b.Bytes()...)
				return payload
			},
			want: []byte("HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\ntest"),
		},
		{
			name: "chunked success",
			payload: func() []byte {
				return []byte("POST / HTTP/1.1\r\nTransfer-Encoding:chunked\r\n\r\n4\nWiki\n5\npedia\n")
			},
			want: []byte("POST / HTTP/1.1\r\nContent-Length: 4\r\n\r\nWiki"),
		},
	} {
		s.Run(tt.name, func() {
			newPayload := PrettifyHTTP(tt.payload())
			s.Equal(tt.want, newPayload)
		})
	}
}
