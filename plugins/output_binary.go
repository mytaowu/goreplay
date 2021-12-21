package plugins

import (
	"sync/atomic"
	"time"

	"goreplay/client"
	"goreplay/config"
	"goreplay/errors"
	"goreplay/logger"
	"goreplay/protocol"
)

// BinaryOutput plugin manage pool of workers which send request to replayed server
// By default workers pool is dynamic and starts with 10 workers
// You can specify fixed number of workers using `--output-tcp-workers`
type BinaryOutput struct {
	// Keep this as first element of struct because it guarantees 64bit
	// alignment. atomic.* functions crash on 32bit machines if operand is not
	// aligned at 64bit. See https://github.com/golang/go/issues/599
	activeWorkers int64
	address       string
	queue         chan *Message
	responses     chan response
	needWorker    chan int
	quit          chan struct{}
	config        *config.BinaryOutputConfig
}

// NewBinaryOutput constructor for BinaryOutput
// Initialize workers
func NewBinaryOutput(address string, config *config.BinaryOutputConfig) PluginReadWriter {
	o := new(BinaryOutput)

	o.address = address
	o.config = config

	o.queue = make(chan *Message, 1000)
	o.responses = make(chan response, 1000)
	o.needWorker = make(chan int, 1)
	o.quit = make(chan struct{})

	// Initial workers count
	if o.config.Workers == 0 {
		o.needWorker <- initialDynamicWorkers
	} else {
		o.needWorker <- o.config.Workers
	}

	go o.workerMaster()

	return o
}

func (o *BinaryOutput) workerMaster() {
	for {
		newWorkers := <-o.needWorker
		for i := 0; i < newWorkers; i++ {
			go o.startWorker()
		}

		// Disable dynamic scaling if workers poll fixed size
		if o.config.Workers != 0 {
			return
		}
	}
}

func (o *BinaryOutput) startWorker() {
	tcpClient := client.NewTCPClient(o.address, &client.TCPClientConfig{
		Debug:              o.config.Debug,
		Timeout:            o.config.Timeout,
		ResponseBufferSize: int(o.config.BufferSize),
	})

	deathCount := 0

	atomic.AddInt64(&o.activeWorkers, 1)
	for {
		select {
		case msg := <-o.queue:
			o.sendRequest(tcpClient, msg)
			deathCount = 0
		case <-time.After(time.Millisecond * 100):
			// When dynamic scaling enabled workers die after 2s of inactivity
			if o.config.Workers != 0 {
				continue
			}

			deathCount++
			if deathCount > 20 {
				workersCount := atomic.LoadInt64(&o.activeWorkers)

				// At least 1 startWorker should be alive
				if workersCount != 1 {
					atomic.AddInt64(&o.activeWorkers, -1)
					return
				}
			}
		}
	}
}

// PluginWrite writes a message to this plugin
func (o *BinaryOutput) PluginWrite(msg *Message) (n int, err error) {
	if !protocol.IsRequestPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	o.queue <- msg

	if o.config.Workers == 0 {
		workersCount := atomic.LoadInt64(&o.activeWorkers)

		if len(o.queue) > int(workersCount) {
			o.needWorker <- len(o.queue)
		}
	}

	return len(msg.Data) + len(msg.Meta), nil
}

// PluginRead reads a message from this plugin
func (o *BinaryOutput) PluginRead() (*Message, error) {
	var (
		resp response
		msg  Message
	)

	select {
	case <-o.quit:
		return nil, errors.ErrorStopped
	case resp = <-o.responses:
	}

	msg.Data = resp.payload
	msg.Meta = protocol.PayloadHeader(protocol.ReplayedResponsePayload, resp.uuid, resp.startedAt, resp.roundTripTime)

	return &msg, nil
}

func (o *BinaryOutput) sendRequest(client *client.TCPClient, msg *Message) {
	if !protocol.IsRequestPayload(msg.Meta) {
		return
	}

	uuid := protocol.PayloadID(msg.Meta)
	start := time.Now()
	resp, err := client.Send(msg.Data)
	if err != nil {
		logger.Warn("[OUTPUT-BINARY]Request error:", err)
	}
	// 计时
	stop := time.Now()
	if o.config.TrackResponses {
		o.responses <- response{resp, uuid, start.UnixNano(),
			stop.UnixNano() - start.UnixNano()}
	}
}

// String output address
func (o *BinaryOutput) String() string {
	return "Binary output: " + o.address
}

// Close closes this plugin for reading
func (o *BinaryOutput) Close() error {
	close(o.quit)

	return nil
}
