package proto

import (
	"bytes"
	"fmt"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestUnitProto(t *testing.T) {
	suite.Run(t, new(testProtoSuite))
}

type testProtoSuite struct {
	suite.Suite
	mockErr error
}

func (s *testProtoSuite) SetupTest() {
	s.mockErr = fmt.Errorf("mock error")
}

func (s *testProtoSuite) TestHeader() {
	for _, tt := range []struct {
		name    string
		payload string
		hName   string
		val     string
	}{
		{
			name:    "with space at start",
			payload: "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:   "Content-Length",
			val:     "7",
		},
		{
			name:    "with space at end",
			payload: "POST /post HTTP/1.1\r\nContent-Length: 7 \r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:   "Content-Length",
			val:     "7",
		},
		{
			name:    "without space at start",
			payload: "POST /post HTTP/1.1\r\nContent-Length:7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:   "Content-Length",
			val:     "7",
		},
		{
			name:    "is empty",
			payload: "GET /p HTTP/1.1\r\nCookie:\r\nHost: www.w3.org\r\n\r\n",
			hName:   "Cookie",
		},
		{
			name:    "lower 2 case headers",
			payload: "POST /post HTTP/1.1\r\ncontent-length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:   "Content-Length",
			val:     "7",
		},
		{
			name:    "lower 1 case headers",
			payload: "POST /post HTTP/1.1\r\ncontent-length: 7\r\nhost: www.w3.org\r\n\r\na=1&b=2",
			hName:   "Host",
			val:     "www.w3.org",
		},
	} {
		s.Run(tt.name, func() {
			val := Header([]byte(tt.payload), []byte(tt.hName))
			s.Equal(val, []byte(tt.val))
		})
	}
}

func (s *testProtoSuite) TestBody() {
	for _, tt := range []struct {
		name         string
		payload      string
		payloadAfter string
	}{
		{
			name:         "end pos",
			payload:      "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			payloadAfter: "a=1&b=2",
		},
	} {
		s.Run(tt.name, func() {
			end := Body([]byte(tt.payload))
			s.Equal(end, []byte(tt.payloadAfter))
		})
	}
}

func (s *testProtoSuite) TestMIMEHeadersEndPos() {
	for _, tt := range []struct {
		name    string
		payload string
		head    string
	}{
		{
			name:    "end pos",
			payload: "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			head:    "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\n",
		},
	} {
		s.Run(tt.name, func() {
			end := MIMEHeadersEndPos([]byte(tt.payload))
			val := []byte(tt.payload[:end])
			s.Equal(val, []byte(tt.head))
		})
	}
}

func (s *testProtoSuite) TestSetHeader() {
	for _, tt := range []struct {
		name         string
		payload      string
		payloadAfter string
		hName        string
		value        string
	}{
		{
			name:         "update header if it exists",
			payload:      "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			payloadAfter: "POST /post HTTP/1.1\r\nContent-Length: 14\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:        "Content-Length",
			value:        "14",
		},
		{
			name:         "add header if not found",
			payload:      "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			payloadAfter: "POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:        "User-Agent",
			value:        "Gor",
		},
		{
			name:         "not modify payload if request is invalid",
			payload:      "POST /post HTTP/1.1",
			payloadAfter: "POST /post HTTP/1.1",
			hName:        "User-Agent",
			value:        "Gor",
		},
	} {
		s.Run(tt.name, func() {
			payload := SetHeader([]byte(tt.payload), []byte(tt.hName), []byte(tt.value))
			s.Equal(payload, []byte(tt.payloadAfter))
		})
	}
}

func (s *testProtoSuite) TestDeleteHeader() {
	for _, tt := range []struct {
		name         string
		payload      string
		payloadAfter string
		hName        string
	}{
		{
			name:         "delete header if found",
			payload:      "POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			payloadAfter: "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:        "User-Agent",
		},
		{
			name:         "whitespace at end of User-Agent",
			payload:      "POST /post HTTP/1.1\r\nUser-Agent: Gor \r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			payloadAfter: "POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2",
			hName:        "User-Agent",
		},
	} {
		s.Run(tt.name, func() {
			payload := DeleteHeader([]byte(tt.payload), []byte(tt.hName))
			s.Equal(payload, []byte(tt.payloadAfter))
		})
	}
}

func (s *testProtoSuite) TestParseHeaders() {
	tests := []struct {
		name     string
		payload  [][]byte
		expected textproto.MIMEHeader
	}{
		{
			name: "success",
			payload: [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"),
				[]byte("g\r\nUser-Ag"), []byte("ent:Chrome\r\n\r\n"),
				[]byte("Fake-Header: asda")},
			expected: textproto.MIMEHeader{
				"Content-Length": []string{"7"},
				"Host":           []string{"www.w3.org"},
				"User-Agent":     []string{"Chrome"},
			},
		},
		{
			name: "with complex UserAgent",
			payload: [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"),
				[]byte("g\r\nUser-Ag"),
				[]byte("ent:Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko\r\n\r\n"),
				[]byte("Fake-Header: asda")},

			expected: textproto.MIMEHeader{"Content-Length": []string{"7"},
				"Host":       []string{"www.w3.org"},
				"User-Agent": []string{"Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko"},
			},
		},
		{
			name: "with Origin",
			payload: [][]byte{[]byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.or"),
				[]byte("g\r\nReferrer: http://127.0.0.1:3000\r\nOrigi"),
				[]byte("n: https://www.example.com\r\nUser-Ag"),
				[]byte("ent:Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko\r\n\r\n"),
				[]byte("in:https://www.example.com\r\n\r\n"), []byte("Fake-Header: asda")},
			expected: textproto.MIMEHeader{"Content-Length": []string{"7"},
				"Host":       []string{"www.w3.org"},
				"Origin":     []string{"https://www.example.com"},
				"Referrer":   []string{"http://127.0.0.1:3000"},
				"User-Agent": []string{"Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko"},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			headers := ParseHeaders(bytes.Join(tt.payload, nil))
			s.Equal(headers, tt.expected)
		})
	}
}

func (s *testProtoSuite) TestPath() {
	tests := []struct {
		name    string
		payload []byte
		path    []byte
	}{
		{
			name:    "find path",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			path:    []byte("/post"),
		},
		{
			name:    "not find path 1",
			payload: []byte("GET /get\r\n\r\nHost: www.w3.org\r\n\r\n"),
			path:    nil,
		},
		{
			name:    "not find path 2",
			payload: []byte("GET /get\n"),
			path:    nil,
		},
		{
			name:    "not find path 3",
			payload: []byte("GET /get\n"),
			path:    nil,
		},
		{
			name:    "not find path 4",
			payload: []byte("GET /get"),
			path:    nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			path := Path(tt.payload)
			s.Equal(path, tt.path)
		})
	}
}

func (s *testProtoSuite) TestSetPath() {
	tests := []struct {
		name         string
		payload      []byte
		payloadAfter []byte
		hName        []byte
	}{
		{
			name:         "replace path",
			payload:      []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST /new_path HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			hName:        []byte("/new_path"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			payload := SetPath(tt.payload, tt.hName)
			s.Equal(payload, tt.payloadAfter)
		})
	}
}

func (s *testProtoSuite) TestPathParam() {
	tests := []struct {
		name    string
		payload []byte
		value   []byte
		hName   []byte
	}{
		{
			name: "param",
			payload: []byte("POST /post?param=test&user_id=1&d_type=1&type=2&d_type=3 HTTP/1.1\r\n" +
				"Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("param"),
			value: []byte("test"),
		},
		{
			name: "user_id",
			payload: []byte("POST /post?param=test&user_id=1&d_type=1&type=2&d_type=3 HTTP/1.1\r\n" +
				"Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("user_id"),
			value: []byte("1"),
		},
		{
			name: "type",
			payload: []byte("POST /post?param=test&user_id=1&d_type=1&type=2&d_type=3 HTTP/1.1\r\n" +
				"Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("type"),
			value: []byte("2"),
		},
		{
			name: "d_type",
			payload: []byte("POST /post?param=test&user_id=1&d_type=1&type=2&d_type=3 HTTP/1.1\r\n" +
				"Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("d_type"),
			value: []byte("1"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			value, _, _ := PathParam(tt.payload, tt.hName)
			s.Equal(value, tt.value)
		})
	}
}

func (s *testProtoSuite) TestSetPathParam() {
	tests := []struct {
		name         string
		payload      []byte
		payloadAfter []byte
		hName        []byte
		value        []byte
	}{
		{
			name: "param",
			payload: []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\n" +
				"Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST /post?param=new&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("param"),
			value: []byte("new"),
		},
		{
			name: "user_id",
			payload: []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST /post?param=test&user_id=2 HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("user_id"),
			value: []byte("2"),
		},
		{
			name:    "set param if url have no params",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST /post?param=test HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("param"),
			value: []byte("test"),
		},
		{
			name: "set param at the end if url params",
			payload: []byte("POST /post?param=test HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST /post?param=test&user_id=1 HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			hName: []byte("user_id"),
			value: []byte("1"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			payload := SetPathParam(tt.payload, tt.hName, tt.value)
			s.Equal(payload, tt.payloadAfter)
		})
	}
}

func (s *testProtoSuite) TestSetHostHTTP10() {
	tests := []struct {
		name         string
		payload      []byte
		payloadAfter []byte
		url          []byte
		host         []byte
	}{
		{
			name: "replace host",
			payload: []byte("POST http://example.com/post HTTP/1.0\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST http://new.com/post HTTP/1.0\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
			url:  []byte("http://new.com"),
			host: []byte("new.com"),
		},
		{
			name:         "nil url",
			payload:      []byte("POST /post HTTP/1.0\r\nContent-Length: 7\r\nHost: example.com\r\n\r\na=1&b=2"),
			payloadAfter: []byte("POST /post HTTP/1.0\r\nContent-Length: 7\r\nHost: new.com\r\n\r\na=1&b=2"),
			url:          nil,
			host:         []byte("new.com"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			payload := SetHost(tt.payload, tt.url, tt.host)
			s.Equal(payload, tt.payloadAfter)
		})
	}
}

func (s *testProtoSuite) TestHasResponseTitle() {
	tests := map[string]bool{
		"HTTP":                      false,
		"":                          false,
		"HTTP/1.1 100 Continue":     false,
		"HTTP/1.1 100 Continue\r\n": true,
		"HTTP/1.1  \r\n":            false,
		"HTTP/4.0 100Continue\r\n":  false,
		"HTTP/1.0 10r Continue\r\n": false,
		"HTTP/1.1 200\r\n":          false,
		"HTTP/1.1 200\r\nServer: Tengine\r\nContent-Length: 0\r\nConnection: close\r\n\r\n": false,
	}
	for k, v := range tests {
		s.Equal(HasResponseTitle([]byte(k)), v)
	}
}

func (s *testProtoSuite) TestCheckChunks() {
	tests := []struct {
		name     string
		buf      []byte
		expected int
	}{
		{
			name: "success",
			buf:  []byte("4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n"),
			expected: bytes.Index([]byte("4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n"),
				[]byte("0\r\n")) + 5,
		},
		{
			name:     "failure",
			buf:      []byte("7\r\nMozia\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n"),
			expected: -1,
		},
		{
			name: "with trailers",
			buf:  []byte("4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\nEXpires"),
			expected: bytes.Index([]byte("4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\nEXpires"),
				[]byte("0\r\n")) + 5,
		},
		{
			name: "last chunk inside the the body with trailers",
			buf:  []byte("4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n3\r\n0\r\n\r\n0\r\n\r\nEXpires"),
			expected: bytes.Index([]byte("4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n"+
				"3\r\n0\r\n\r\n0\r\n\r\nEXpires"),
				[]byte("0\r\n")) + 10,
		},
		{
			name: "checks with chucks-extensions",
			buf: []byte("4\r\nWiki\r\n5\r\npedia\r\nE; name='quoted string'\r\n in\r\n\r\nchunks.\r\n" +
				"3\r\n0\r\n\r\n0\r\n\r\nEXpires"),
			expected: bytes.Index([]byte("4\r\nWiki\r\n5\r\npedia\r\nE; name='quoted string'\r\n in"+
				"\r\n\r\nchunks.\r\n3\r\n0\r\n\r\n0\r\n\r\nEXpires"),
				[]byte("0\r\n")) + 10,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			chunkEnd := CheckChunked(tt.buf)
			s.Equal(chunkEnd, tt.expected)
		})
	}
}

func (s *testProtoSuite) TestHasFullPayload() {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name: "success",
			data: []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n" +
				"\r\n7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n"),
			expected: true,
		},
		{
			name: "check with invalid chunk format",
			data: []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n" +
				"\r\n7\r\nMozia\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n"),
			expected: false,
		},
		{
			name: "check chunks with trailers",
			data: []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n" +
				"Trailer: Expires\r\n\r\n7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n" +
				"\r\nExpires: Wed, 21 Oct 2015 07:28:00 GMT\r\n\r\n"),
			expected: true,
		},
		{
			name: "check with missing trailers",
			data: []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n" +
				"Trailer: Expires\r\n\r\n7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\n" +
				"Network\r\n0\r\n\r\nExpires: Wed, 21 Oct 2015 07:28:00"),
			expected: false,
		},
		{
			name: "check with content-length",
			data: []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n" +
				"Content-Length: 23\r\n\r\nMozillaDeveloperNetwork"),
			expected: true,
		},
		{
			name: "check missing total length",
			data: []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n" +
				"Content-Length: 23\r\n\r\nMozillaDeveloperNet"),
			expected: false,
		},
		{
			name:     "check with no body",
			data:     []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"),
			expected: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := HasFullPayload(tt.data, nil)
			s.Equal(got, tt.expected)
		})
	}
}
