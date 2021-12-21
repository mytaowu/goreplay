package codec

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

func init() {
	RegisterHeaderCodec("http", &httpHeaderCodecBuilder{})
}

type httpHeaderCodecBuilder struct {
}

// New 新建实例
func (builder *httpHeaderCodecBuilder) New() HeaderCodec {
	return &httpHeaderCodec{}
}

// httpHeaderCodec http请求头解析
type httpHeaderCodec struct {
	Request *http.Request
}

// MethodName get method name
func (h *httpHeaderCodec) MethodName() string {
	return ""
}

// InterfaceName get interface name
func (h *httpHeaderCodec) InterfaceName() string {
	return ""
}

// Decode 解码
func (h *httpHeaderCodec) Decode(reqBuf []byte, _ string) (ProtocolHeader, error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(reqBuf)))
	if err != nil {
		return ProtocolHeader{}, fmt.Errorf("read request err: %v", err)
	}

	h.Request = req

	return ProtocolHeader{
		ServiceName:   h.ServiceName(),
		APIName:       h.APIName(),
		MethodName:    h.MethodName(),
		InterfaceName: h.InterfaceName(),
	}, err
}

// ServiceName get service name
func (h *httpHeaderCodec) ServiceName() string {
	// http协议的url 是类似于 /grpc.logreplay.helloworld.EchoHttp/SayHello
	apiName := strings.Split(h.APIName(), "/")

	if len(apiName) < 2 {
		return h.APIName()
	}

	serviceNames := strings.Split(apiName[1], ".")
	if len(serviceNames) == 0 || serviceNames[len(serviceNames)-1] == "" {
		return h.APIName()
	}

	return serviceNames[len(serviceNames)-1]
}

// APIName get api name
func (h *httpHeaderCodec) APIName() string {
	uri := h.Request.RequestURI
	if uri == "" {
		return uri
	}

	uris := strings.Split(uri, "?")

	return uris[0]
}
