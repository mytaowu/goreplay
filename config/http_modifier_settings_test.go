package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type httpModifierSettingsSuite struct {
	suite.Suite
}

func TestUnitHTTPModifierSettings(t *testing.T) {
	suite.Run(t, new(httpModifierSettingsSuite))
}

func (s *httpModifierSettingsSuite) TestHTTPHeaderFilters() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "single value",
			value:   "Header1",
			wantErr: fmt.Errorf("need both header and value, colon-delimited (ex. user_id:^169$)"),
		},
		{
			name:    "both value",
			value:   "Header2:^:$",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HTTPHeaderFilters)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestHTTPParams() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "single value",
			value:   "Header1",
			wantErr: fmt.Errorf("expected `Key=Value`"),
		},
		{
			name:    "both value",
			value:   "Header2=1",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HTTPParams)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestHTTPHeaders() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "single value",
			value:   "Header1",
			wantErr: fmt.Errorf("expected `Key: Value`"),
		},
		{
			name:    "both value",
			value:   "Header2:1",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HTTPHeaders)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestHTTPURLRegexp() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "success",
			value:   ".",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HTTPURLRegexp)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestHTTPHeaderBasicAuthFilters() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "success",
			value:   ".",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HTTPHeaderBasicAuthFilters)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestHTTPHashFilters() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "single value",
			value:   "Header1",
			wantErr: fmt.Errorf("need both header and value, colon-delimited (ex. user_id:50%%)"),
		},
		{
			name:    "no % symbol",
			value:   "Header2:^:$",
			wantErr: fmt.Errorf("value should be percent and contain '%%'"),
		},
		{
			name:    "success",
			value:   "Header2:10%",
			wantErr: nil,
		},
		{
			name:    "not support old syntax",
			value:   "Header1:1/2",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HTTPHashFilters)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestUrlRewriteMap() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "success",
			value:   "/v1/user/([^\\/]+)/ping:/v2/user/$1/ping",
			wantErr: nil,
		},
		{
			name:    "error",
			value:   "/v1/user/([^\\\\/]+)/ping",
			wantErr: fmt.Errorf("need both src and target, colon-delimited (ex. /a:/b)"),
		},
	} {
		s.Run(tt.name, func() {
			h := new(URLRewriteMap)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}

func (s *httpModifierSettingsSuite) TestHeaderRewriteMap() {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:  "single value",
			value: "Header1",
			wantErr: fmt.Errorf("need both header, regexp and rewrite target, colon-delimited " +
				"(ex. Header: regexp,target)"),
		},
		{
			name:  "single header",
			value: "Header1:123",
			wantErr: fmt.Errorf("need both header, regexp and rewrite target, colon-delimited " +
				"(ex. Header: regexp,target)"),
		},
		{
			name:    "success",
			value:   "header1:header2:%d,1",
			wantErr: nil,
		},
	} {
		s.Run(tt.name, func() {
			h := new(HeaderRewriteMap)
			err := h.Set(tt.value)
			s.Equal(tt.wantErr, err)
		})
	}
}
