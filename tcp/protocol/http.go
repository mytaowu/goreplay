package protocol

import (
	"goreplay/proto"
	"goreplay/tcp"
)

func init() {
	tcp.RegisterFramerBuilder("http", &httpFramerBuilder{})
}

// httpFramerBuilder http framer builder
type httpFramerBuilder struct{}

// New 新建实例
func (fb *httpFramerBuilder) New(listenAddr string) tcp.Framer {
	return &httpFramer{CommonFramer: tcp.CommonFramer{ListenAddr: listenAddr}}
}

type httpFramer struct {
	tcp.CommonFramer
}

// Start hints message pool to start the reassembling the message
func (f *httpFramer) Start(pckt *tcp.Packet) (isIncoming, isOutgoing bool) {
	// 一些握手包或者保活包
	if len(pckt.Payload) == 0 {
		return false, false
	}

	if proto.HasRequestTitle(pckt.Payload) {
		return true, false
	}
	return false, proto.HasResponseTitle(pckt.Payload)
}

// End hints message pool to stop the session
func (f *httpFramer) End(msg *tcp.Message) bool {
	return proto.HasFullPayload(msg.Data(), msg)
}
