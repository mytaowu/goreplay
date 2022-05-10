package plugins

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goreplay/config"
	"goreplay/framer"
	"goreplay/logger"
	"goreplay/middleware/loader"
	"goreplay/protocol"
	"io"

	"github.com/golang/groupcache/lru"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"

	"golang.org/x/net/http2"
)

const (
	headerLength = 5
)

var (
	cache              = lru.New(65535)
	readMetaHeadersKey = "ReadMetaHeaders_%s"
)

type grpcMiddleWare struct {
	pbdesc *loader.Pbdesc
	next   MiddlewareFun
}

// NewGrpcMiddleWare 创建grpcMiddleWare对象
func NewGrpcMiddleWare(address string, config *config.MiddlewareGrpcConfig) *grpcMiddleWare {
	return &grpcMiddleWare{
		pbdesc: loader.NewPbDesc(config.MiddleGrpcProtoFile),
	}
}

// MidWareHandle 中间件处理
func (g *grpcMiddleWare) MidWareHandle(msg *Message,
	err error) (*Message, error) {
	// 若是开始就存在错误，则不进行处理
	if err != nil {
		return g.next.MidWareHandle(msg, err)
	}

	fr := framer.NewHTTP2Framer(msg.Data, "", true)
	header := make(map[string]string)
	var rowBody []byte
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			if err != io.EOF {
				logger.Warn("grpcMiddleWare read frame err: io.EOF")
			}

			break
		}

		switch f := frame.(type) {
		case *http2.MetaHeadersFrame:
			for _, hf := range f.Fields {
				header[hf.Name] = hf.Value
			}

			// 记录流ID
			header["stream_id"] = fmt.Sprint(f.StreamID)
		case *http2.DataFrame:
			rowBody = make([]byte, len(f.Data()))
			copy(rowBody, f.Data())
		}
	}

	logger.Info("header: %v", header)
	if rowBody != nil && len(rowBody) > headerLength {
		rowBody = rowBody[headerLength:]
	}

	logger.Info("row body: %s", string(rowBody))
	logger.Info("base64 row body: %s", base64.StdEncoding.EncodeToString(rowBody))
	logger.Info("path: %s", header[":path"])

	methodDesc := g.pbdesc.GetMethodDesc(header[":path"])

	if methodDesc == nil {
		v, ok := cache.Get(fmt.Sprintf(readMetaHeadersKey, header["stream_id"]))
		if ok {
			methodDesc = v.(*desc.MethodDescriptor)
		}
	}

	logger.Info(fmt.Sprintf("---header[stream_id]: %s, methodDesc: %v", header["stream_id"], methodDesc))

	if methodDesc == nil {
		if g.next != nil {
			return g.next.MidWareHandle(msg, err)
		}

		return msg, err
	}

	msgDesc := methodDesc.GetInputType()
	if !protocol.IsRequestPayload(msg.Meta) {
		msgDesc = methodDesc.GetOutputType()
	}

	dmsg := dynamic.NewMessage(msgDesc)

	err = dmsg.Unmarshal(rowBody)
	//把test.AddFriendReq 消息体序列化成 JSON 数据
	jsStr, _ := dmsg.MarshalJSON()

	res := map[string]interface{}{
		"header": header,
		"body":   jsStr,
	}

	cache.Add(fmt.Sprintf(readMetaHeadersKey, header["stream_id"]), methodDesc)

	msg.Data, _ = json.Marshal(res)
	logger.Info(fmt.Sprintf("----处理之后的结果: %s", string(msg.Data)))

	if g.next != nil {
		return g.next.MidWareHandle(msg, err)
	}

	return msg, err
}

// SetNextMidWare 设置下一个中间件处理
func (g *grpcMiddleWare) SetNextMidWare(next MiddlewareFun) {
	g.next = next
}
