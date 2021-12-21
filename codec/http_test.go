package codec

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"

	"goreplay/byteutils"
)

// TestUnitHTTPCodec HttpCodec test execute
func TestUnitHTTPCodec(t *testing.T) {
	suite.Run(t, new(TestUnitHTTPCodecSuite))
}

// TestUnitHTTPCodecSuite HttpCodec test suite
type TestUnitHTTPCodecSuite struct {
	suite.Suite

	reqBuf  []byte
	builder *httpHeaderCodecBuilder
	codec   HeaderCodec
}

// SetupTest which will run before each test in the suite.
func (t *TestUnitHTTPCodecSuite) SetupTest() {
	dataBytes, err := ioutil.ReadFile("data/http.data")
	if err != nil {
		t.T().Errorf("ioutil.ReadFile err : %v", err)
	}
	t.reqBuf, _ = base64.StdEncoding.DecodeString(byteutils.SliceToString(dataBytes))
	t.builder = &httpHeaderCodecBuilder{}
	t.codec = t.builder.New()
}

// TestHttpHeaderCodecDecode test HttpHeaderCodecDecode Method
func (t *TestUnitHTTPCodecSuite) TestHttpHeaderCodecDecode() {
	tests := []struct {
		name    string
		reqBuf  []byte
		want    interface{}
		wantErr bool
	}{
		{
			name:   "success",
			reqBuf: t.reqBuf,
			want: ProtocolHeader{
				ServiceName:   "EchoHttp",
				APIName:       "/grpc.logreplay.helloworld.EchoHttp/SayHello",
				MethodName:    "",
				InterfaceName: "",
				CusTraceID:    "",
			},
		},
		{
			name:    "inavild http buff",
			reqBuf:  t.reqBuf[:5],
			want:    ProtocolHeader{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			protocolHeader, err := t.codec.Decode(tt.reqBuf, "")
			if (err != nil) != tt.wantErr {
				t.T().Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(protocolHeader, tt.want) {
				t.T().Errorf("Decode() got = %v, want %v", protocolHeader, tt.want)
			}
		})
	}
}

// TestAPIName test ApiName Method
func (t *TestUnitHTTPCodecSuite) TestAPIName() {

	tests := []struct {
		name    string
		req     *httpHeaderCodec
		wantErr bool
	}{
		{
			name: "success",
			req: &httpHeaderCodec{
				Request: &http.Request{
					RequestURI: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			str := tt.req.APIName()
			if !reflect.DeepEqual(str, "") {
				t.T().Errorf("APIName() got = %v, want %v", str, "")
			}
		})
	}
}

// TestServiceName test serviceName Method
func (t *TestUnitHTTPCodecSuite) TestServiceName() {

	tests := []struct {
		name    string
		req     *httpHeaderCodec
		want    string
		wantErr bool
	}{
		{
			name: "success1",
			req: &httpHeaderCodec{
				Request: &http.Request{
					RequestURI: "test info",
				},
			},
			want: "test info",
		},
		{
			name: "success2",
			req: &httpHeaderCodec{
				Request: &http.Request{
					RequestURI: "test/info",
				},
			},
			want: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			str := tt.req.ServiceName()
			if !reflect.DeepEqual(str, tt.want) {
				t.T().Errorf("ServiceName() got = %v, want %v", str, "")
			}
		})
	}
}
