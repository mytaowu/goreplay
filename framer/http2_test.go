package framer

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/suite"
	"golang.org/x/net/http2"
)

// TestUnitHttp2 http2 test execute
func TestUnitHttp2(t *testing.T) {
	suite.Run(t, new(TestUnitHTTP2Suite))
}

// TestUnitHTTP2Suite testUnitHTTP2Suite test suite
type TestUnitHTTP2Suite struct {
	suite.Suite

	reqBuf    []byte
	framer    *HTTP2Framer
	errFramer *HTTP2Framer
}

// SetupTest which will run before each test in the suite.
func (t *TestUnitHTTP2Suite) SetupTest() {
	dataBytes, err := ioutil.ReadFile("data/http2.data")
	if err != nil {
		t.T().Errorf("ioutil.ReadFile err : %v", err)
	}
	t.reqBuf = dataBytes
	t.framer = NewHTTP2Framer(dataBytes, "logreplay", true)
	t.framer = NewHTTP2Framer(dataBytes, "", true)
	t.framer = NewHTTP2Framer(dataBytes, "logreplay", false)
	t.errFramer = NewHTTP2Framer(dataBytes[:40], "logreplay", false)
}

// TestReadFrameAndBytes test ReadFrameAndBytes method
func (t *TestUnitHTTP2Suite) TestReadFrameAndBytes() {
	const sc = "success"

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name: sc,
		},
		{
			name:    "fail",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			var err error
			if sc == tt.name {
				_, _, err = t.framer.ReadFrameAndBytes()
			} else {
				_, _, err = t.errFramer.ReadFrameAndBytes()
			}

			if (err != nil) != tt.wantErr {
				t.T().Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

// TestReEncodeMetaHeadersFrame test ReEncodeMetaHeadersFrame method
func (t *TestUnitHTTP2Suite) TestReEncodeMetaHeadersFrame() {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name: "success",
		},
		{
			name:    "fail",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			var err error
			var frame http2.Frame
			if "success" == tt.name {
				_, frame, _ = t.framer.ReadFrameAndBytes()
			} else {
				_, frame, _ = t.errFramer.ReadFrameAndBytes()
			}

			if hf, ok := frame.(*http2.MetaHeadersFrame); ok {
				_, err = ReEncodeMetaHeadersFrame(hf)
				if (err != nil) != tt.wantErr {
					t.T().Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}
		})
	}
}
