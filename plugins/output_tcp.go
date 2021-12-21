package plugins

import (
	"fmt"
	"hash/fnv"

	"goreplay/config"
	"goreplay/logger"
	"goreplay/protocol"
	"goreplay/stat"
)

// TCPOutput used for sending raw tcp payloads
// Currently used for internal communication between listener and replay server
// Can be used for transfering binary payloads like protocol buffers
type TCPOutput struct {
	address     string
	limit       int
	buf         []chan *Message
	bufStats    *stat.GorStat
	conf        *config.TCPOutputConfig
	workerIndex uint32
}

// NewTCPOutput constructor for TCPOutput
// Initialize X workers which hold keep-alive connection
func NewTCPOutput(address string, conf *config.TCPOutputConfig) PluginWriter {
	o := new(TCPOutput)

	o.address = address
	o.conf = conf

	if config.Settings.OutputTCPStats {
		o.bufStats = stat.NewGorStat("output_tcp", 5000)
	}

	// create X buffers and send the buffer index to the worker
	o.buf = make([]chan *Message, o.conf.Workers)
	for i := 0; i < o.conf.Workers; i++ {
		o.buf[i] = make(chan *Message, 100)
		go o.worker(i)
	}

	return o
}

func (o *TCPOutput) worker(bufferIndex int) {
	for {
		msg := <-o.buf[bufferIndex]
		if !protocol.IsRequestPayload(msg.Meta) {
			continue
		}

		o.write(msg.Data, 2)
	}
}

func (o *TCPOutput) write(data []byte, retries int) {
	for i := 0; i < retries; i++ {
		// 发送请求
		// TODO send tcp
		logger.Debug("output tcp")
		// if err != nil {
		// 	logger.Error("[TCP-OUTPUT] o.client.RoundTrip err", err, "  retries=", i)
		// } else {
		// 	break
		// }

	}
}
func (o *TCPOutput) getBufferIndex(data []byte) int {
	if !o.conf.Sticky {
		o.workerIndex++
		return int(o.workerIndex) % o.conf.Workers
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write(protocol.PayloadMeta(data)[1])
	return int(hasher.Sum32()) % o.conf.Workers

}

// PluginWrite writes message to this plugin
func (o *TCPOutput) PluginWrite(msg *Message) (n int, err error) {
	if !protocol.IsOriginPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	bufferIndex := o.getBufferIndex(msg.Data)
	o.buf[bufferIndex] <- msg

	if config.Settings.OutputTCPStats {
		o.bufStats.Write(len(o.buf[bufferIndex]))
	}

	return len(msg.Data) + len(msg.Meta), nil
}

// String output address
func (o *TCPOutput) String() string {
	return fmt.Sprintf("TCP output %s, limit: %d", o.address, o.limit)
}
