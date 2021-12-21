package plugins

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"goreplay/byteutils"
	"goreplay/client"
	"goreplay/codec"
	"goreplay/config"
	"goreplay/errors"
	"goreplay/logger"
	"goreplay/logreplay"
	"goreplay/protocol"
	"goreplay/remote"

	"github.com/coocood/freecache"
	jsoniter "github.com/json-iterator/go"
	"golang.org/x/net/http2"
)

const (
	freeCacheExpired = 60
	reportType       = "goReplay"
	cacheSizeMin     = 100
	recordLimit      = 10000
	defaultQPSLimit  = 10

	goreplay              = "goreplay"
	localhost             = "127.0.0.1"
	goReplayRequestKey    = "req_%s"
	goReplayReplayRespKey = "replay_rsp_%s"
	goReplayReqHeaderKey  = "req_header_%s"
)

// SendError 发送错误类型
type SendError int

const (
	sendSucc SendError = iota
	dialError
	writeError
	readError
	emptySettingFrame = "000000040000000000"
)

// LogReplayOutput plugin manage pool of workers which send request to logreplay server
// By default workers pool is dynamic and starts with 1 worker or workerMin workers
// You can specify maximum number of workers using `--output-logreplay-workers`
type LogReplayOutput struct {
	address                                string
	conf                                   *config.LogReplayOutputConfig
	tcpClient                              *client.TCPClient
	cache                                  *freecache.Cache
	buf                                    []chan *Message
	responses                              chan *response
	stop                                   chan bool // Channel used only to indicate goroutine should shutdown
	target                                 string
	recordNum                              uint32 // 已上报的总数
	curQPS                                 uint32
	taskID                                 uint32
	success, dialFail, writeFail, readFail uint32
	reportBuf                              chan logreplay.ReportItem
	lastSampleTime                         int64
	json                                   jsoniter.API
}

// NewLogReplayOutput constructor for LogReplayOutput
// Initialize workers
func NewLogReplayOutput(address string, conf *config.LogReplayOutputConfig) PluginReadWriter {
	o := new(LogReplayOutput)
	o.json = jsoniter.ConfigCompatibleWithStandardLibrary
	o.address = localhost
	instanceName, err := getEnvInfo()
	if err == nil {
		o.address = instanceName
	}

	checkLogReplayConfig(conf)

	o.target = conf.Target

	o.conf = conf
	if o.conf.TrackResponses {
		o.responses = make(chan *response, o.conf.Workers)
	}

	if o.conf.TargetTimeout <= 0 {
		o.conf.TargetTimeout = time.Second
		logger.Info("[LOGREPLAY-OUTPUT] targetTimeout default value : ", o.conf.TargetTimeout)
	}

	if conf.Target != "" {
		tcpClient := client.NewTCPClient(conf.Target, &client.TCPClientConfig{
			Debug:   true,
			Timeout: o.conf.TargetTimeout,
		})
		o.tcpClient = tcpClient
	}

	o.reportBuf = make(chan logreplay.ReportItem)
	o.stop = make(chan bool)
	o.cache = freecache.NewCache(conf.CacheSize * 1024 * 1024)

	o.buf = make([]chan *Message, o.conf.Workers)
	for i := 0; i < o.conf.Workers; i++ {
		o.buf[i] = make(chan *Message, 100)
		go o.startWorker(i)
	}

	for i := 0; i < 5; i++ {
		go o.startReporter()
	}

	return o
}

func checkLogReplayConfig(conf *config.LogReplayOutputConfig) {
	if conf.ModuleID == "" {
		logger.Fatal("using logreplay require moduleid: --output-logreplay-moduleid")
		return
	}

	if conf.APPID == "" {
		logger.Fatal("using logreplay require appid: --output-logreplay-appid")
		return
	}

	if conf.APPKey == "" {
		logger.Fatal("using logreplay require appkey: --output-logreplay-appkey")
		return
	}

	// 先必传 protocol
	if conf.Protocol == "" {
		logger.Fatal("using logreplay require protocol: --output-logreplay-protocol")
		return
	}

	if conf.CommitID == "" {
		logger.Fatal("using logreplay require commitID(or version of protocol): --output-logreplay-commitid")
		return
	}

	if conf.GatewayAddr == "" {
		logger.Fatal("using logreplay require gateway host: --output-logreplay-gateway")
		return
	}
	checkLogreplayOptionalConfig(conf)
}

func checkLogreplayOptionalConfig(conf *config.LogReplayOutputConfig) {
	if conf.Target != "" {
		checkAddress(conf.Target)
	}

	if conf.Env == "" {
		conf.Env = config.EnvFormal
	}

	if conf.Env != config.EnvFormal {
		conf.Env = config.EnvTest
		logger.Info("[LOGREPLAY-OUTPUT] env default value : ", conf.Env)
	}

	if conf.Timeout < time.Millisecond*100 {
		conf.Timeout = time.Second
		logger.Info("[LOGREPLAY-OUTPUT] timeout default value : ", conf.Timeout)
	}

	if conf.Workers <= 0 {
		conf.Workers = 1
		logger.Info("[LOGREPLAY-OUTPUT] workers default value : ", conf.Workers)
	}

	if conf.CacheSize <= 0 {
		conf.CacheSize = cacheSizeMin // cache 缓存大小，单位 M
		logger.Info("[LOGREPLAY-OUTPUT] cache-size default value : ", conf.CacheSize)
	}

	if conf.RecordLimit <= 0 {
		conf.RecordLimit = recordLimit
		logger.Info("[LOGREPLAY-OUTPUT] record-limit default value : ", conf.RecordLimit)
	}

	if conf.QPSLimit <= 0 {
		conf.QPSLimit = defaultQPSLimit
		logger.Info("[LOGREPLAY-OUTPUT] qps-limit default value : ", conf.QPSLimit)
	}
}

func (o *LogReplayOutput) checkModuleAuth(conf *config.LogReplayOutputConfig) {
	rsp := &logreplay.AuthRsp{}

	err := o.send(logreplay.GetCasKeyURL, &logreplay.AuthReq{ModuleID: conf.ModuleID}, rsp)
	if err != nil {
		logger.Fatal("[LOGREPLAY-OUTPUT] checkModuleAuth error, get auth failed: ", err)
		return
	}

	if rsp.ID != conf.APPID || rsp.KEY != conf.APPKey {
		logger.Fatal("[LOGREPLAY-OUTPUT] checkModuleAuth failed, appid and appkey not match")
		return
	}
}

func (o *LogReplayOutput) checkAndSetDefaultServiceName(conf *config.LogReplayOutputConfig) {
	if conf.ProtocolServiceName != "" {
		return
	}

	rsp := newLogreplayResponse()
	err := o.send(logreplay.GetGetModuleURL, &logreplay.GetModuleReq{ModuleID: conf.ModuleID}, rsp)
	if err != nil {
		logger.Fatal("[LOGREPLAY-OUTPUT] getGetModule error: ", err)
		return
	}

	if rsp.Module.AppNameEn == "" || rsp.Module.ModuleNameEn == "" {
		logger.Fatal("[LOGREPLAY-OUTPUT] getGetModule empty AppNameEn or ModuleNameEn")
		return
	}

	conf.ProtocolServiceName = fmt.Sprintf("%s.%s", rsp.Module.AppNameEn, rsp.Module.ModuleNameEn)
	logger.Info("[LOGREPLAY-OUTPUT] output-logreplay-protocol-service-name default value : ",
		conf.ProtocolServiceName)
}

func newLogreplayResponse() *logreplay.GetModuleRsp {
	return &logreplay.GetModuleRsp{}
}

func (o *LogReplayOutput) createTask() (uint32, error) {
	req := &logreplay.GoReplayReq{
		ModuleID:       o.conf.ModuleID,
		Operator:       goreplay,
		Total:          uint32(o.conf.RecordLimit),
		Rate:           100,
		RecordCommitID: goreplay,
		TargetModuleID: o.conf.ModuleID,
		Addrs:          o.conf.Target,
	}
	rsp := &logreplay.TaskRsp{}

	err := o.send(logreplay.GoReplayTaskURL, req, rsp)
	if err != nil {
		logger.Fatal("[LOGREPLAY-OUTPUT] createTask error, get taskID failed: ", err)
		return 0, err
	}

	return rsp.TaskID, nil
}

func (o *LogReplayOutput) startWorker(bufferIndex int) {
	for {
		if atomic.LoadUint32(&o.recordNum) > uint32(o.conf.RecordLimit) {
			logger.Fatal("[LOGREPLAY-OUTPUT] already access record max limit: ", o.conf.RecordLimit)
			return
		}
		msg := <-o.buf[bufferIndex]

		if !protocol.IsOriginPayload(msg.Meta) {
			return
		}

		o.sendMsgLogReplay(msg)
	}
}

func (o *LogReplayOutput) startReporter() {
	rp := &Reporter{
		items: []logreplay.ReportItem{},
		timer: time.NewTicker(3 * time.Second),
		o:     o,
		lock:  sync.Mutex{},
	}

	rp.run()
}

func (o *LogReplayOutput) report(items []logreplay.ReportItem) {
	if len(items) > 0 {
		rsp := &logreplay.ReportRsp{}
		err := o.send(logreplay.ReportURL, &logreplay.ReportData{Batch: items}, rsp)
		if err != nil {
			logger.Warn("[LOGREPLAY-OUTPUT] report LogReplay error: ", err)
			return
		}

		atomic.AddUint32(&o.recordNum, uint32(rsp.Succeed))
		logger.Info("[LOGREPLAY-OUTPUT] 上报总数: ", o.recordNum)
		logger.Debug2("[LOGREPLAY-OUTPUT] report rsp ", rsp)
	}
}

func (o *LogReplayOutput) isQPSOver() bool {
	now := time.Now().UnixNano()
	if (time.Now().UnixNano() - atomic.LoadInt64(&o.lastSampleTime)) > time.Second.Nanoseconds() {
		atomic.StoreInt64(&o.lastSampleTime, now)
		atomic.StoreUint32(&o.curQPS, 0)
	} else {
		atomic.AddUint32(&o.curQPS, 1)
	}

	return atomic.LoadUint32(&o.curQPS) > uint32(o.conf.QPSLimit)
}

func (o *LogReplayOutput) getBufferIndex(data []byte) int {
	hasher := fnv.New32a()
	_, _ = hasher.Write(data)

	return int(hasher.Sum32()) % o.conf.Workers
}

func getReqKey(uuid string) []byte {
	return []byte(fmt.Sprintf(goReplayRequestKey, uuid))
}

func getReqHeaderKey(uuid string) []byte {
	return []byte(fmt.Sprintf(goReplayReqHeaderKey, uuid))
}

func getReqRspKey(uuid string) []byte {
	return []byte(fmt.Sprintf(goReplayReplayRespKey, uuid))
}

func (o *LogReplayOutput) sendMsgLogReplay(msg *Message) {
	uuid := protocol.PayloadID(msg.Meta)
	uuidStr := string(uuid)
	cacheReq, _ := o.cache.Get(getReqKey(uuidStr))

	logger.Debug3(fmt.Sprintf("[LOGREPLAY-OUTPUT] sendMsgLogReplay msg meta: %s, uuidStr: %s",
		byteutils.SliceToString(msg.Meta), uuidStr))

	// 如果拦截的是 res payload，但是没有缓存 req，直接返回
	if msg.Meta[0] == protocol.ResponsePayload && len(cacheReq) == 0 {
		logger.Debug3("[LOGREPLAY-OUTPUT] sendMsgLogReplay, discard rsp uuid ", uuidStr)
		return
	}

	// 解析 request 包
	if msg.Meta[0] == protocol.RequestPayload {
		headerCodec := codec.GetHeaderCodec(o.conf.Protocol)
		if headerCodec == nil {
			logger.Fatal("[LOGREPLAY-OUTPUT] not supported protocol: %s", o.conf.Protocol)
		}

		o.parseReq(msg, headerCodec, uuidStr)
	} else if msg.Meta[0] == protocol.ResponsePayload { // 解析 response 包
		defer func() {
			o.cache.Del(getReqKey(uuidStr))
			o.cache.Del(getReqRspKey(uuidStr))
			o.cache.Del(getReqHeaderKey(uuidStr))
		}()

		logger.Debug3("[LOGREPLAY-OUTPUT] sendMsgLogReplay, start parse rsp, uuid: ", uuidStr)
		record, err := o.parseResponse(msg, cacheReq, uuidStr)
		if err != nil {
			logger.Info("[LOGREPLAY-OUTPUT] parseResponse err: ", err)
		}

		// 录制数据上报 LogReplay
		o.sendRequest(uuidStr, record)
	}
}

func (o *LogReplayOutput) doReplay(msg *Message, header codec.ProtocolHeader) []byte {
	if o.conf.Target != "" {
		if !protocol.IsRequestPayload(msg.Meta) {
			return nil
		}

		if o.conf.Protocol == codec.GrpcName && o.conf.GrpcReplayMethodName != "" &&
			!strings.Contains(o.conf.GrpcReplayMethodName, header.MethodName) {
			logger.Debug3("[LOGREPLAY-OUTPUT]  grpc protocol method not match, header method: ",
				header.MethodName, ",config interface name: "+o.conf.GrpcReplayMethodName, ",")

			return nil
		}

		// TODO tcp send repalay data
		var err error
		switch dispatchSendErr(err) {
		case sendSucc:
			atomic.AddUint32(&o.success, 1)
		case dialError:
			atomic.AddUint32(&o.dialFail, 1)
		case writeError:
			atomic.AddUint32(&o.writeFail, 1)
		case readError:
			atomic.AddUint32(&o.readFail, 1)
		default:
		}
		if err != nil {
			logger.Debug3("[LOGREPLAY-OUTPUT]  Request error:", err, " body: ",
				hex.EncodeToString(msg.Data), "meta: ", string(msg.Meta))
		}

		return nil
	}

	return nil
}

func (o *LogReplayOutput) parseResponse(msg *Message, cacheReq []byte,
	uuidStr string) (*logreplay.GoReplayMessage, error) {
	cacheHeaderBytes, err := o.cache.Get(getReqHeaderKey(uuidStr))
	if err != nil {
		return nil, err
	}

	var cacheReqHeader codec.ProtocolHeader
	if err = o.json.Unmarshal(cacheHeaderBytes, &cacheReqHeader); err != nil {
		return nil, err
	}

	// traceID := tracer.NewTraceIDWithParent("").String()
	// TODO 生成 traceID
	traceID := uuidStr
	if cacheReqHeader.CusTraceID != "" {
		traceID = cacheReqHeader.CusTraceID
	}

	data := &logreplay.GoReplayMessage{
		RecordMessage: logreplay.RecordMessage{
			ModuleID:      o.conf.ModuleID,
			CommitID:      o.conf.CommitID,
			Time:          float64(time.Now().UnixNano()),
			TraceID:       traceID,
			InstanceName:  o.address,
			ServiceName:   cacheReqHeader.ServiceName,
			APIName:       cacheReqHeader.APIName,
			Protocol:      o.conf.Protocol,
			Src:           goreplay,
			ResponseBytes: msg.Data,
		},
		ProtocolServiceName: o.conf.ProtocolServiceName,
		MethodName:          cacheReqHeader.MethodName,
		InterfaceName:       cacheReqHeader.InterfaceName,
	}

	tag := make(map[string]interface{})
	tag["isGoReplay"] = "true"
	tag["realServerName"] = o.conf.RealServerName
	tag["serverAddr"] = strings.Join(config.Settings.InputRAW, ";")
	tag["clientAddr"] = msg.SrcAddr

	data.RequestBytes = o.appendAfterClientPreface(cacheReq)
	cacheReplayResponse, _ := o.cache.Get(getReqRspKey(uuidStr))
	data.ReplayBytes = cacheReplayResponse
	data.Tag = tag

	return data, nil
}

func (o *LogReplayOutput) appendAfterClientPreface(src []byte) []byte {
	if o.conf.Protocol != codec.GrpcName {
		return src
	}

	esf, _ := hex.DecodeString(emptySettingFrame)
	ret := append([]byte(http2.ClientPreface), esf...)

	return append(ret, src...)
}

func (o *LogReplayOutput) parseReq(msg *Message, headerCodec codec.HeaderCodec, uuidStr string) {
	// 先解析请求头，注意这里解析的请求的 data
	ph, err := headerCodec.Decode(msg.Data, msg.ConnectionID)
	if err != nil {
		logger.Error("[LOGREPLAY-OUTPUT] decode request header error: ", err)
		return
	}

	if o.isQPSOver() {
		logger.Warn("[LOGREPLAY-OUTPUT] already access record qps limit: ", atomic.LoadUint32(&o.curQPS),
			o.conf.QPSLimit)
		return
	}

	logger.Debug3(fmt.Sprintf("[LOGREPLAY-OUTPUT] %s req cache Key: %s", uuidStr, getReqKey(uuidStr)))
	// 缓存请求数据
	err = o.cache.Set(getReqKey(uuidStr), msg.Data, freeCacheExpired)
	if err != nil {
		logger.Error("[LOGREPLAY-OUTPUT] cache req err: ", err)
		return
	}

	phBytes, err := o.json.Marshal(ph)
	if err != nil {
		logger.Info("[LOGREPLAY-OUTPUT] marshal req header err: ", err)
		return
	}
	logger.Debug3("[LOGREPLAY-OUTPUT] record header", string(phBytes))
	// 缓存请求的 header
	err = o.cache.Set(getReqHeaderKey(uuidStr), phBytes, freeCacheExpired)
	if err != nil {
		logger.Error("[LOGREPLAY-OUTPUT] cache rea header err: ", err)
		return
	}

	// 缓存回放响应
	err = o.cache.Set(getReqRspKey(uuidStr), o.doReplay(msg, ph), freeCacheExpired)
	if err != nil {
		logger.Error("[LOGREPLAY-OUTPUT] cache replay resp err: ", err)
	}
}

func (o *LogReplayOutput) sendRequest(uuidStr string, record *logreplay.GoReplayMessage) {
	// 生成回放任务
	if o.conf.Target != "" && o.taskID == 0 {
		taskID, err := o.createTask()
		if err != nil {
			logger.Debug2("[LOGREPLAY-OUTPUT] sendRequest, createTask failed uuid ", uuidStr)
			return
		}

		o.taskID = taskID
	}

	record.TaskID = o.taskID
	record.Success = atomic.LoadUint32(&o.success)
	record.DialFailed = atomic.LoadUint32(&o.dialFail)
	record.WriteFailed = atomic.LoadUint32(&o.writeFail)
	record.ReadFailed = atomic.LoadUint32(&o.readFail)
	record.SendFailed = record.DialFailed + record.WriteFailed + record.ReadFailed

	recordStr, marErr := o.json.Marshal(record)
	if marErr != nil {
		logger.Error("[LOGREPLAY-OUTPUT] sendRequest error, Marshal record failed: ",
			uuidStr, marErr)

		return
	}

	item := logreplay.ReportItem{Type: reportType, Data: string(recordStr)}

	o.reportBuf <- item
}

// PluginWrite writes message to this plugin
func (o *LogReplayOutput) PluginWrite(msg *Message) (n int, err error) {
	if !protocol.IsOriginPayload(msg.Meta) {
		return len(msg.Data), nil
	}

	uuid := protocol.PayloadID(msg.Meta)
	bufferIndex := o.getBufferIndex(uuid)
	o.buf[bufferIndex] <- msg

	return len(msg.Data) + len(msg.Meta), nil
}

// PluginRead reads message from this plugin
func (o *LogReplayOutput) PluginRead() (*Message, error) {
	if !o.conf.TrackResponses {
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

	msg.Meta = protocol.PayloadHeader(protocol.ReplayedResponsePayload, resp.uuid,
		resp.roundTripTime, resp.startedAt)

	return &msg, nil
}

// String string logreplay output
func (o *LogReplayOutput) String() string {
	return fmt.Sprintf("LogReplay output: %s", o.conf.ModuleID)
}

// Close closes the data channel so that data
func (o *LogReplayOutput) Close() error {
	close(o.stop)
	return nil
}

// send 发送请求
func (o *LogReplayOutput) send(url string, req, rsp interface{}) error {
	err := remote.Send(url, req, rsp)
	buf, _ := json.Marshal(req)
	logger.Debug3(fmt.Sprintf("send %s, req: %s, rsp: %+v, error: %v", url, string(buf), rsp, err))
	if err != nil {
		return err
	}

	return nil
}

func checkAddress(address string) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		logger.Fatal("[LOGREPLAY-OUTPUT] parsing address error: ", err)
		return
	}

	if host == "" || port == "" || net.ParseIP(host) == nil {
		logger.Fatal("[LOGREPLAY-OUTPUT] invalid address: ", address, err)
		return
	}
}

// 发送错误归类
func dispatchSendErr(err error) SendError {
	if err == nil {
		return sendSucc
	}

	if isWriteError(err) {
		return writeError
	}

	return readError
}

func isWriteError(err error) bool {

	return false
}

// Reporter reports the data to logreplay
type Reporter struct {
	items []logreplay.ReportItem
	timer *time.Ticker
	o     *LogReplayOutput
	lock  sync.Mutex
}

func (r *Reporter) run() {
	var (
		stop bool
	)

	for !stop {
		select {
		case item, isOpen := <-r.o.reportBuf:
			if isOpen {
				r.items = append(r.items, item)
				if len(r.items) > 100 {
					r.commit()
				}
			} else {
				r.timer.Stop()
				stop = true
				r.commit()

			}
		case <-r.timer.C:
			r.commit()
		}
	}
}

func (r *Reporter) commit() {
	r.lock.Lock()
	defer r.lock.Unlock()
	reqs := r.items

	r.o.report(reqs)
	r.items = make([]logreplay.ReportItem, 0)
}

// getEnvInfo 获取环境名称
func getEnvInfo() (string, error) {
	return os.Hostname()
}
