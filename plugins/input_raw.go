package plugins

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"goreplay/byteutils"
	"goreplay/capture"
	"goreplay/config"
	"goreplay/errors"
	"goreplay/logger"
	"goreplay/proto"
	"goreplay/protocol"
	"goreplay/tcp"
)

const hostSplit = ","

// RAWInput used for intercepting traffic for given address
type RAWInput struct {
	sync.Mutex
	config.RAWInputConfig
	messageStats   []tcp.Stats
	listener       *capture.Listener
	message        chan *tcp.Message
	cancelListener context.CancelFunc
	selectHostMap  map[string]bool // 指定录制的host的map
}

// NewRAWInput constructor for RAWInput. Accepts raw input config as arguments.
func NewRAWInput(address string, config config.RAWInputConfig) (i *RAWInput) {
	i = new(RAWInput)
	i.RAWInputConfig = config
	i.message = make(chan *tcp.Message, 1000)
	i.Quit = make(chan bool)
	var host, _port string
	var err error
	var port int
	host, _port, err = net.SplitHostPort(address)
	if err != nil {
		logger.Error("input-raw: error while parsing address: %s", err)
	}
	if _port != "" {
		port, err = strconv.Atoi(_port)
	}

	if err != nil {
		log.Fatalf("parsing port error: %v", err)
	}
	i.Host = host
	i.Port = uint16(port)

	// 录制指定Host来源的流量
	if config.SelectHost != "" {
		i.checkSelectHost(config.SelectHost)
	}

	if config.Logreplay {
		i.checkAndSelectIP()

		logger.Info(fmt.Sprintf("listening %s:%d", i.Host, i.Port))

		// 请求报文  来源端口跟15取模后 符合采样条件
		sampleSrcPort := fmt.Sprintf(" and (( tcp[0:2] & 0x0f) < %d)", config.LogreplaySampleRate)
		// 响应报文  目标端口跟15取模后, 符合采样条件
		sampleDstPort := fmt.Sprintf(" and (( tcp[2:2] & 0x0f) < %d)", config.LogreplaySampleRate)

		i.BPFFilter = fmt.Sprintf("(tcp dst port %d and dst host %s %s) or (tcp src port %d and src host %s %s)",
			port, i.Host, sampleSrcPort, port, i.Host, sampleDstPort)

		// 设置默认的BufferTimeout, 避免cpu空转
		if i.BufferTimeout == 0 {
			i.BufferTimeout = 3 * time.Second
		}
	}

	i.listen(address)

	return
}

// checkSelectHost 检测输入的host是否合法，并且保存到selectHostMap中
func (i *RAWInput) checkSelectHost(hostStr string) {
	logger.Info(fmt.Sprintf("filter record msg, target host value:%s", hostStr))

	hostList := strings.Split(hostStr, hostSplit)

	i.selectHostMap = make(map[string]bool, len(hostList))
	for _, v := range hostList {
		if ip := net.ParseIP(v); ip == nil {
			logger.Fatal(fmt.Sprintf("illegal input-raw-select-host, host: %s", v))
		}

		logger.Info(fmt.Sprintf("select host target host:%s", v))
		i.selectHostMap[v] = true
	}
}

func (i *RAWInput) checkAndSelectIP() {
	if net.ParseIP(i.Host) == nil {
		ips, err := autoGetIP()
		if err != nil {
			log.Fatalf("autoGetIP fail: %v", err)
			return
		}
		if len(ips) <= 0 {
			log.Fatalf("autoGetIP empty")
			return
		}
		i.Host = ips[0]
		if len(ips) > 1 {
			// 本机IP大于2个时, 需要选择一个
			i.selectIP(ips)
		}
	}
}

func (i *RAWInput) selectIP(ips []string) {
	if i.AutoSelectIP {
		// 直接return 就是选择第一个
		return
	}

	displayIps(ips)

	inputReader := bufio.NewReader(os.Stdin)
	input, err := inputReader.ReadString('\n')
	input = strings.Replace(input, "\n", "", -1)
	if err != nil {
		log.Fatalf("There were errors reading, exiting program.")
		return
	}
	var selectNumber int
	for {
		selectNumber, err = strconv.Atoi(input)
		if err != nil || selectNumber <= 0 || selectNumber > len(ips) {
			displayIps(ips)

			input, err = inputReader.ReadString('\n')
			input = strings.Replace(input, "\n", "", -1)
			if err != nil {
				log.Fatalf("There were errors reading, exiting program.")
				return
			}
		} else {
			break
		}
	}

	i.Host = ips[selectNumber-1]
}

func displayIps(ips []string) {
	log.Printf("\n本机存在多个ip, 请输入编号(1到%d)选择一个ip:\n", len(ips))
	for i, ip := range ips {
		fmt.Printf("[%d] %s \n", i+1, ip)
	}
}

func autoGetIP() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		return ips, err
	}

	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	return ips, nil
}

// PluginRead reads meassage from this plugin
// PluginRead reads message from this plugin
func (i *RAWInput) PluginRead() (*Message, error) {
	var msgTCP *tcp.Message
	var msg Message
	select {
	case <-i.Quit:
		return nil, errors.ErrorStopped
	case msgTCP = <-i.message:
		msg.Data = msgTCP.Data()
		defer func() {
			msgTCP.Reset()
			msgTCP = nil
		}()
	}
	if msgTCP.LostData > 0 {
		logger.Debug("tcp包有被截断, 考虑使用--input-raw-override-snaplen :   ", msgTCP.Length, msgTCP.LostData)
	}

	var msgType byte = protocol.ResponsePayload
	if msgTCP.IsIncoming {
		msgType = protocol.RequestPayload
		if i.RealIPHeader != "" {
			msg.Data = proto.SetHeader(msg.Data, []byte(i.RealIPHeader), []byte(msgTCP.SrcAddr))
		}
	}

	// 在请求的时候，根据host进行流量的筛选
	if msgType == protocol.RequestPayload && len(i.selectHostMap) > 0 {
		host, _, err := net.SplitHostPort(msgTCP.SrcAddr)
		if err != nil {
			logger.Fatal(fmt.Sprintf("input raw tcp src add, split host port error:%v, addr: %s", err, msgTCP.SrcAddr))
		}

		if !i.selectHostMap[host] {
			logger.Debug(fmt.Sprintf("received a packet from %s, but the host does not match", host))

			return nil, errors.ErrorFilterFromIP
		}
	}

	// 响应包的时候，记录来源请求地址(即回包的目的地址)
	if msgType == protocol.ResponsePayload {
		host, _, err := net.SplitHostPort(msgTCP.DstAddr)
		if err != nil {
			logger.Debug("input raw tcp des add, split host port error:%v, addr: %s", err, msgTCP.DstAddr)
		}
		msg.SrcAddr = host
	}

	msg.Meta = protocol.PayloadHeader(msgType, msgTCP.UUID(), msgTCP.Start.UnixNano(),
		msgTCP.End.UnixNano()-msgTCP.Start.UnixNano())
	msg.ConnectionID = msgTCP.ConnectionID()

	logger.Debug3(fmt.Sprintf("[INPUT-RAW] msg meta: %s", byteutils.SliceToString(msg.Meta)))

	// to be removed....
	if msgTCP.Truncated {
		logger.Debug2("[INPUT-RAW] message truncated, increase copy-buffer-size")
	}
	// to be removed...
	if msgTCP.TimedOut && len(msgTCP.Data()) > 0 {
		logger.Debug2("[INPUT-RAW] message timeout reached, increase input-raw-expire")
	}
	if i.Stats {
		stat := msgTCP.Stats
		go i.addStats(stat)
	}

	return &msg, nil
}

func (i *RAWInput) listen(address string) {
	var err error
	i.listener, err = capture.NewListener(i.Host, i.Port, "", i.Engine, i.TrackResponse)
	if err != nil {
		log.Fatal(err)
	}
	i.listener.SetPcapOptions(i.PcapOptions)
	err = i.listener.Activate()
	if err != nil {
		log.Fatal(err)
	}
	pool := tcp.NewMessagePool(i.CopyBufferSize, i.Expire, i.handler)
	pool.MatchUUID(i.TrackResponse)

	// listen address: ip+port
	pool.Address(address)
	// set business protocol
	pool.Protocol(i.Protocol)

	var ctx context.Context
	ctx, i.cancelListener = context.WithCancel(context.Background())
	errCh := i.listener.ListenBackground(ctx, pool.Handler)
	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-i.listener.Reading:
		logger.Debug(i)
	}
}

func (i *RAWInput) handler(m *tcp.Message) {
	i.message <- m
}

// String input address
func (i *RAWInput) String() string {
	return fmt.Sprintf("Intercepting traffic from: %s:%d", i.Host, i.Port)
}

// GetStats returns the stats so far and reset the stats
func (i *RAWInput) GetStats() []tcp.Stats {
	i.Lock()
	defer func() {
		i.messageStats = []tcp.Stats{}
		i.Unlock()
	}()
	return i.messageStats
}

// Close closes the input raw listener
func (i *RAWInput) Close() error {
	i.cancelListener()
	close(i.Quit)
	return nil
}

func (i *RAWInput) addStats(mStats tcp.Stats) {
	i.Lock()
	if len(i.messageStats) >= 10000 {
		i.messageStats = []tcp.Stats{}
	}
	i.messageStats = append(i.messageStats, mStats)
	i.Unlock()
}
