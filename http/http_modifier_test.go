package http

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"

	"goreplay/config"
	"goreplay/proto"
)

type httpModifierSuite struct {
	suite.Suite
}

func TestUnitHTTPModifier(t *testing.T) {
	suite.Run(t, new(httpModifierSuite))
}

func (s *httpModifierSuite) TestHTTPModifierHeaderFilters() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name:    "success",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderFilters{}
				_ = filters.Set("Host:^www.w3.org$")

				conf := &config.HTTPModifierConfig{
					HeaderFilters: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name:    "not match headers",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderFilters{}
				_ = filters.Set("Host:^www.w4.org$")

				conf := &config.HTTPModifierConfig{
					HeaderFilters: filters,
				}

				return conf
			},
			isZero: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierURLRegexp() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name:    "url regexp pass",
			payload: []byte("POST /v1/app/test HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPURLRegexp{}
				_ = filters.Set("/v1/app")

				conf := &config.HTTPModifierConfig{
					URLRegexp: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name:    "url regexp not pass",
			payload: []byte("POST /v1/app/test HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPURLRegexp{}
				_ = filters.Set("/other")

				conf := &config.HTTPModifierConfig{

					URLRegexp: filters,
				}

				return conf
			},
			isZero: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierNegativeFilters() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name:    "negative filters success",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w4.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderFilters{}
				_ = filters.Set("Host:^www.w3.org$")

				conf := &config.HTTPModifierConfig{
					HeaderNegativeFilters: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name:    "negative filters not match",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w4.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderFilters{}
				_ = filters.Set("Host:^www.w4.org$")

				conf := &config.HTTPModifierConfig{
					HeaderNegativeFilters: filters,
				}

				return conf
			},
			isZero: true,
		},
		{
			name:    "negative filters not match",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w4.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderFilters{}
				_ = filters.Set("Host: www*")

				conf := &config.HTTPModifierConfig{
					HeaderNegativeFilters: filters,
				}

				return conf
			},
			isZero: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierBasicAuthHeaderFilters() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name: "basic auth success",
			payload: []byte("POST / HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Authorization: Basic Y3VzdG9tZXIzOndlbGNvbWU=\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderBasicAuthFilters{}
				_ = filters.Set("^customer[0-9].*")

				conf := &config.HTTPModifierConfig{
					HeaderBasicAuthFilters: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name: "basic auth pass",
			payload: []byte("POST / HTTP/1.1\r\nContent-Length: 88\r\n" +
				"Authorization: Basic Y3VzdG9tZXI2OnJlc3RAMTIzXlRFU1Q==\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderBasicAuthFilters{}
				_ = filters.Set("^(homer simpson|mickey mouse).*")

				conf := &config.HTTPModifierConfig{
					HeaderBasicAuthFilters: filters,
				}

				return conf
			},
			isZero: true,
		},
		{
			name: "basic auth not pass",
			payload: []byte("POST /post HTTP/1.1\nContent-Length: 88\n" +
				"Authorization: Basic bWlja2V5IG1vdXNlOmhhcHB5MTIz\n\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaderBasicAuthFilters{}
				_ = filters.Set("^(homer simpson|mickey mouse).*")

				conf := &config.HTTPModifierConfig{
					HeaderBasicAuthFilters: filters,
				}

				return conf
			},
			isZero: false,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierHashFilters() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name:    "hash filter not exist",
			payload: []byte("POST / HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHashFilters{}
				_ = filters.Set("Header2:1/2")

				conf := &config.HTTPModifierConfig{
					HeaderHashFilters: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name:    "hash filter hash too high",
			payload: []byte("POST / HTTP/1.1\r\nHeader2: 3\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHashFilters{}
				_ = filters.Set("Header2:1/2")

				conf := &config.HTTPModifierConfig{
					HeaderHashFilters: filters,
				}

				return conf
			},
			isZero: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierParamFilters() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name:    "param filter not exist",
			payload: []byte("POST / HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHashFilters{}
				_ = filters.Set("user_id:1/2")

				conf := &config.HTTPModifierConfig{
					ParamHashFilters: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name:    "param filter not pass",
			payload: []byte("POST /?user_id=3 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHashFilters{}
				_ = filters.Set("user_id:1/2")

				conf := &config.HTTPModifierConfig{
					ParamHashFilters: filters,
				}

				return conf
			},
			isZero: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierURLRewrite() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		url          []byte
		modifierConf func() *config.HTTPModifierConfig
		isEqual      bool
	}{
		{
			name:    "rewrite success",
			payload: []byte("POST /v1/user/ping HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				rewrites := config.URLRewriteMap{}
				_ = rewrites.Set("/v1/user/([^\\/]+)/ping:/v2/user/$1/ping")

				return &config.HTTPModifierConfig{
					URLRewrite: rewrites,
				}
			},
			url:     []byte("/v1/user/ping"),
			isEqual: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())
			newURL := proto.Path(modifier.Rewrite(tt.payload))
			s.Equal(tt.isEqual, bytes.Equal(newURL, tt.url))
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierHeadersRewrite() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		url          []byte
		modifierConf func() *config.HTTPModifierConfig
		isEqual      bool
	}{
		{
			name:    "success",
			payload: []byte("GET / HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				rewrites := config.HeaderRewriteMap{}
				_ = rewrites.Set("Host: (.*).w3.org,$1.beta.w3.org")

				return &config.HTTPModifierConfig{
					Methods: [][]byte{
						[]byte("POST"),
						[]byte("GET"),
					},
					HeaderRewrite: rewrites,
				}
			},
			url:     []byte("www.beta.w3.org"),
			isEqual: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())
			newURL := proto.Header(modifier.Rewrite(tt.payload), []byte("Host"))
			s.Equal(tt.isEqual, bytes.Equal(newURL, tt.url))
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierHeaders() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		wantPayload  []byte
	}{
		{
			name:    "success",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				headers := config.HTTPHeaders{}
				_ = headers.Set("Header1:1")
				_ = headers.Set("Host:localhost")

				return &config.HTTPModifierConfig{
					Headers: headers,
				}
			},
			wantPayload: []byte("POST /post HTTP/1.1\r\nHeader1: 1\r\n" +
				"Content-Length: 7\r\nHost: localhost\r\n\r\na=1&b=2"),
		},
		{
			name:    "set param",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPParams{}
				_ = filters.Set("api_key=1")

				return &config.HTTPModifierConfig{
					Params: filters,
				}
			},
			wantPayload: []byte("POST /post?api_key=1 HTTP/1.1\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
		},
		{
			name:    "set header",
			payload: []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPHeaders{}
				_ = filters.Set("User-Agent:Gor")

				return &config.HTTPModifierConfig{
					Headers: filters,
				}
			},
			wantPayload: []byte("POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\n" +
				"Host: www.w3.org\r\n\r\na=1&b=2"),
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())
			payload := modifier.Rewrite(tt.payload)
			s.Equal(payload, tt.wantPayload)
		})
	}
}

func (s *httpModifierSuite) TestHTTPModifierNegativeRegexp() {
	for _, tt := range []struct {
		name         string
		payload      []byte
		modifierConf func() *config.HTTPModifierConfig
		isZero       bool
	}{
		{
			name:    "url regexp pass",
			payload: []byte("POST /v1/app/test HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPURLRegexp{}
				_ = filters.Set("/restricted1")
				_ = filters.Set("/some/restricted2")

				conf := &config.HTTPModifierConfig{
					URLNegativeRegexp: filters,
				}

				return conf
			},
			isZero: false,
		},
		{
			name:    "url regexp not pass",
			payload: []byte("POST /restricted1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"),
			modifierConf: func() *config.HTTPModifierConfig {
				filters := config.HTTPURLRegexp{}
				_ = filters.Set("/restricted1")
				_ = filters.Set("/some/restricted2")

				conf := &config.HTTPModifierConfig{
					URLNegativeRegexp: filters,
				}

				return conf
			},
			isZero: true,
		},
	} {
		s.Run(tt.name, func() {
			modifier := NewHTTPModifier(tt.modifierConf())

			s.Equal(len(modifier.Rewrite(tt.payload)) == 0, tt.isZero)
		})
	}
}

func TestHTTPModifierSetHeader(t *testing.T) {
	filters := config.HTTPHeaders{}
	_ = filters.Set("User-Agent:Gor")

	modifier := NewHTTPModifier(&config.HTTPModifierConfig{
		Headers: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	str := "POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2"
	payloadAfter := []byte(str)

	if payload = modifier.Rewrite(payload); !bytes.Equal(payloadAfter, payload) {
		t.Error("Should add new header", string(payload))
	}
}

func TestHTTPModifierSetParam(t *testing.T) {
	filters := config.HTTPParams{}
	_ = filters.Set("api_key=1")

	modifier := NewHTTPModifier(&config.HTTPModifierConfig{
		Params: filters,
	})

	payload := []byte("POST /post?api_key=1234 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter := []byte("POST /post?api_key=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payloadAfter, payload) {
		t.Error("Should override param", string(payload))
	}
}
