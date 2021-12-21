package plugins

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"goreplay/config"
	"goreplay/errors"
	"goreplay/logger"
	"goreplay/protocol"
	"goreplay/stat"
)

const initialDynamicWorkers = 10

type response struct {
	payload       []byte
	uuid          []byte
	roundTripTime int64
	startedAt     int64
}

// HTTPOutput plugin manage pool of workers which send request to replayed server
// By default workers pool is dynamic and starts with 1 worker or workerMin workers
// You can specify maximum number of workers using `--output-http-workers`
type HTTPOutput struct {
	activeWorkers int32
	Config        *config.HTTPOutputConfig
	queueStats    *stat.GorStat
	client        *HTTPClient
	stopWorker    chan struct{}
	queue         chan *Message
	responses     chan *response
	stop          chan bool // Channel used only to indicate goroutine should shutdown
}

// NewHTTPOutput constructor for HTTPOutput
// Initialize workers
func NewHTTPOutput(address string, config *config.HTTPOutputConfig) PluginReadWriter {
	o := new(HTTPOutput)
	var err error

	config.URL, err = url.Parse(address)
	if err != nil {
		logger.Fatal("[OUTPUT-HTTP] parse HTTP output URL error:", err)
	}

	if config.URL.Scheme == "" {
		config.URL.Scheme = "http"
	}

	config.RawURL = config.URL.String()
	if config.Timeout < time.Millisecond*100 {
		config.Timeout = time.Second
	}

	initConfigSize(config)

	o.Config = config
	o.stop = make(chan bool)
	if o.Config.Stats {
		o.queueStats = stat.NewGorStat("output_http", o.Config.StatsMs)
	}

	o.queue = make(chan *Message, o.Config.QueueLen)
	if o.Config.TrackResponses {
		o.responses = make(chan *response, o.Config.QueueLen)
	}
	// it should not be buffered to avoid races
	o.stopWorker = make(chan struct{})

	o.client = NewHTTPClient(o.Config)
	o.activeWorkers += int32(o.Config.WorkersMin)
	for i := 0; i < o.Config.WorkersMin; i++ {
		go o.startWorker()
	}

	go o.workerMaster()

	return o
}

// initConfigSize initial config workers` size and queue length
func initConfigSize(config *config.HTTPOutputConfig) {
	if config.BufferSize <= 0 {
		config.BufferSize = 100 * 1024 // 100kb
	}

	if config.WorkersMin <= 0 {
		config.WorkersMin = 1
	}

	if config.WorkersMin > 1000 {
		config.WorkersMin = 1000
	}

	if config.WorkersMax <= 0 {
		config.WorkersMax = math.MaxInt32 // idealy so large
	}

	if config.WorkersMax < config.WorkersMin {
		config.WorkersMax = config.WorkersMin
	}

	if config.QueueLen <= 0 {
		config.QueueLen = 1000
	}

	if config.RedirectLimit < 0 {
		config.RedirectLimit = 0
	}

	if config.WorkerTimeout <= 0 {
		config.WorkerTimeout = time.Second * 2
	}
}

func (o *HTTPOutput) workerMaster() {
	var timer = time.NewTimer(o.Config.WorkerTimeout)
	defer func() {
		// recover from panics caused by trying to send in
		// a closed chan(o.stopWorker)
		_ = recover()
	}()
	defer timer.Stop()
	for {
		select {
		case <-o.stop:
			return
		default:
			<-timer.C
		}
		// rollback worker
	rollback:
		if atomic.LoadInt32(&o.activeWorkers) > int32(o.Config.WorkersMin) && len(o.queue) < 1 {
			// close one worker
			o.stopWorker <- struct{}{}
			atomic.AddInt32(&o.activeWorkers, -1)
			goto rollback
		}
		timer.Reset(o.Config.WorkerTimeout)
	}
}

func (o *HTTPOutput) startWorker() {
	for {
		select {
		case <-o.stopWorker:
			return
		case msg := <-o.queue:
			o.sendRequest(o.client, msg)
		}
	}
}

// PluginWrite writes message to this plugin
func (o *HTTPOutput) PluginWrite(msg *Message) (n int, err error) {
	if !protocol.IsRequestPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	select {
	case <-o.stop:
		return 0, errors.ErrorStopped
	case o.queue <- msg:
	}

	if o.Config.Stats {
		o.queueStats.Write(len(o.queue))
	}

	if len(o.queue) > 0 {
		// try to start a new worker to serve
		if atomic.LoadInt32(&o.activeWorkers) < int32(o.Config.WorkersMax) {
			go o.startWorker()
			atomic.AddInt32(&o.activeWorkers, 1)
		}
	}

	return len(msg.Data) + len(msg.Meta), nil
}

// PluginRead reads message from this plugin
func (o *HTTPOutput) PluginRead() (*Message, error) {
	if !o.Config.TrackResponses {
		return nil, errors.ErrorStopped
	}
	var (
		resp *response
		msg  Message
	)

	select {
	case <-o.stop:
		return nil, errors.ErrorStopped
	case resp = <-o.responses:
		msg.Data = resp.payload
	}

	msg.Meta = protocol.PayloadHeader(protocol.ReplayedResponsePayload, resp.uuid, resp.roundTripTime, resp.startedAt)

	return &msg, nil
}

func (o *HTTPOutput) sendRequest(client *HTTPClient, msg *Message) {
	if !protocol.IsRequestPayload(msg.Meta) {
		return
	}
	uuid := protocol.PayloadID(msg.Meta)
	start := time.Now()
	resp, err := client.Send(msg.Data)
	stop := time.Now()

	if err != nil {
		logger.Debug("[HTTP-OUTPUT] error when sending: ", err)
		return
	}
	if resp == nil {
		return
	}

	if o.Config.TrackResponses {
		o.responses <- &response{resp, uuid, start.UnixNano(),
			stop.UnixNano() - start.UnixNano()}
	}
}

// String output address
func (o *HTTPOutput) String() string {
	return "HTTP output: " + o.Config.RawURL
}

// Close closes the data channel so that data
func (o *HTTPOutput) Close() error {
	close(o.stop)
	close(o.stopWorker)
	return nil
}

// HTTPClient holds configurations for a single HTTP client
type HTTPClient struct {
	config *config.HTTPOutputConfig
	Client *http.Client
}

// NewHTTPClient returns new http client with check redirects policy
func NewHTTPClient(config *config.HTTPOutputConfig) *HTTPClient {
	client := new(HTTPClient)
	client.config = config
	var transport *http.Transport

	client.Client = &http.Client{
		Timeout: client.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= client.config.RedirectLimit {
				logger.Debug(fmt.Sprintf("[HTTPCLIENT] maximum output-http-redirects[%d] reached!",
					client.config.RedirectLimit))

				return http.ErrUseLastResponse
			}
			lastReq := via[len(via)-1]
			resp := req.Response
			logger.Debug2(fmt.Sprintf("[HTTPCLIENT] HTTP redirects from %q to %q with %q",
				lastReq.Host, req.Host, resp.Status))

			return nil
		},
	}

	if config.SkipVerify {
		// clone to avoid modying global default RoundTripper
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client.Client.Transport = transport
	}

	return client
}

// Send sends an http request using client create by NewHTTPClient
func (c *HTTPClient) Send(data []byte) ([]byte, error) {
	var req *http.Request
	var resp *http.Response
	var err error

	req, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, err
	}
	// we don't send CONNECT or OPTIONS request
	if req.Method == http.MethodConnect {
		return nil, nil
	}

	if !c.config.OriginalHost {
		req.Host = c.config.URL.Host
	}

	// fix #862
	if c.config.URL.Path == "" && c.config.URL.RawQuery == "" {
		req.URL.Scheme = c.config.URL.Scheme
		req.URL.Host = c.config.URL.Host
	} else {
		req.URL = c.config.URL
	}

	// force connection to not be closed, which can affect the global client
	req.Close = false
	// it's an error if this is not equal to empty string
	req.RequestURI = ""

	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	if c.config.TrackResponses {
		return httputil.DumpResponse(resp, true)
	}

	return nil, nil
}
