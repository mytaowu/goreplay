package codec

const unknown = "unknown"

// emptyHeaderCodec use test for codec
type emptyHeaderCodec struct {
}

// Decode decode buff data
func (e *emptyHeaderCodec) Decode(_ []byte, _ string) (ProtocolHeader, error) {
	protoHead := ProtocolHeader{
		ServiceName:   unknown,
		APIName:       unknown,
		InterfaceName: unknown,
		MethodName:    unknown,
	}
	return protoHead, nil
}

// emptyHeaderCodecBuilder emptyCodec的建造者
type emptyHeaderCodecBuilder struct {
}

// New 创建 emptyCodec的实例对象
func (builder *emptyHeaderCodecBuilder) New() HeaderCodec {
	return &emptyHeaderCodec{}
}
