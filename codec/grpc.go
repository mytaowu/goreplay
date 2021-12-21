package codec

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/http2"

	"goreplay/framer"
)

// GrpcName "grpc"常量，暴露出去供其他包使用
const GrpcName = "grpc"

func init() {
	RegisterHeaderCodec(GrpcName, &grpcHeaderCodecBuilder{})
}

type grpcHeaderCodecBuilder struct {
}

// New 实例化解码器
func (builder *grpcHeaderCodecBuilder) New() HeaderCodec {
	return &grpcHeaderCodec{}
}

// grpcHeaderCodec grpc请求头解析
type grpcHeaderCodec struct {
}

// Decode 请求解码
func (g *grpcHeaderCodec) Decode(payload []byte, _ string) (ProtocolHeader, error) {
	ret := ProtocolHeader{}

	fr := framer.NewHTTP2Framer(payload, "", true)
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			if err != io.EOF {
				return ret, fmt.Errorf("grpcHeaderCodece err: %v %s", err, hex.EncodeToString(payload))
			}

			break
		}

		switch f := frame.(type) {
		case *http2.MetaHeadersFrame:
			for _, hf := range f.Fields {
				if hf.Name == ":path" {
					ret.ServiceName, ret.APIName = parseServiceName(hf.Value)
					ret.InterfaceName = parseInterfaceName(ret.ServiceName)
					ret.MethodName = ret.APIName
				}

				if hf.Name == framer.LogReplayTraceID {
					ret.CusTraceID = hf.Value
				}
			}
		}
	}
	return ret, nil
}

func parseServiceName(sm string) (service, method string) {
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		service = unknown
		method = unknown
		return
	}
	service = sm[:pos]
	method = sm[pos+1:]

	return
}

func parseInterfaceName(serviceName string) (interfaceName string) {
	pos := strings.LastIndex(serviceName, ".")
	if pos == -1 {
		interfaceName = unknown
		return
	}
	interfaceName = serviceName[pos+1:]

	return
}
