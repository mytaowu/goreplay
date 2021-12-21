package listener

import (
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"

	"goreplay/logger"
)

const or = " or "

// IPacket ip packet
type IPacket struct {
	srcIP     []byte
	dstIP     []byte
	payload   []byte
	timestamp time.Time
}

// IPListener is ip listener struct
type IPListener struct {
	mu            sync.Mutex
	addr          string
	port          uint16
	trackResponse bool
	pcapHandles   []*pcap.Handle
	ipPacketsChan chan *IPacket
	readyChan     chan bool
}

// NewIPListener new IPListener
func NewIPListener(addr string, port uint16, trackResponse bool) (l *IPListener) {
	l = &IPListener{}
	l.ipPacketsChan = make(chan *IPacket, 10000)
	l.readyChan = make(chan bool, 1)
	l.addr = addr
	l.port = port
	l.trackResponse = trackResponse
	go l.readPcap()

	return
}

// DeviceNotFoundError raised if user specified wrong ip
type DeviceNotFoundError struct {
	addr string
}

// Error get error string
func (e *DeviceNotFoundError) Error() string {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		logger.Error("device error")
	}

	if len(devices) == 0 {
		return "Can't get list of network interfaces, ensure that you running as root user or sudo"
	}

	var msg string
	msg += "Can't find interfaces with addr: " + e.addr + ". Provide available IP for intercepting traffic: \n"
	for _, device := range devices {
		msg += "Name: " + device.Name + "\n"
		if device.Description != "" {
			msg += "Description: " + device.Description + "\n"
		}
		for _, address := range device.Addresses {
			msg += "- IP address: " + address.IP.String() + "\n"
		}
	}

	return msg
}

func isLoopback(device pcap.Interface) bool {
	if len(device.Addresses) == 0 {
		return false
	}

	switch device.Addresses[0].IP.String() {
	case "127.0.0.1", "::1":
		return true
	default:
	}

	return false
}

func listenAllInterfaces(addr string) bool {
	switch addr {
	case "", "0.0.0.0", "[::]", "::":
		return true
	default:
		return false
	}
}

// findPcapDevices find all device
func findPcapDevices(addr string) (interfaces []pcap.Interface, err error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		logger.Error(err)
		return
	}

	for _, device := range devices {
		if listenAllInterfaces(addr) && len(device.Addresses) > 0 || isLoopback(device) {
			interfaces = append(interfaces, device)
			continue
		}

		for _, address := range device.Addresses {
			if device.Name == addr || address.IP.String() == addr {
				interfaces = append(interfaces, device)
				return interfaces, nil
			}
		}
	}

	if len(interfaces) == 0 {
		return nil, &DeviceNotFoundError{addr}
	}

	return interfaces, nil
}
func (l *IPListener) buildPacket(srcIP []byte, dstIP []byte, payload []byte, timestamp time.Time) *IPacket {
	return &IPacket{
		srcIP:     srcIP,
		dstIP:     dstIP,
		payload:   payload,
		timestamp: timestamp,
	}
}

func (l *IPListener) readPcap() {
	devices, err := findPcapDevices(l.addr)
	if err != nil {
		logger.Error(err)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(len(devices))

	for _, d := range devices {
		go func(device pcap.Interface) {
			inactive, err := pcap.NewInactiveHandle(device.Name)
			if err != nil {
				logger.Info("Pcap Error while opening device", device.Name, err)
				wg.Done()
				return
			}

			inactive = setInactiveConfig(device, inactive)
			handle, herr := inactive.Activate()
			if herr != nil {
				logger.Info("PCAP Activate error:", herr)
				wg.Done()
				return
			}

			defer handle.Close()
			l.mu.Lock()
			l.pcapHandles = append(l.pcapHandles, handle)

			if !isSupport() {
				logger.Info("the operating system is darwin,stop read")
				wg.Done()
				return
			}
			berkeleyPacketFilter := getBerkeleyPacketFilter(l)
			err = handle.SetBPFFilter(berkeleyPacketFilter)
			if err != nil {
				logger.Info("BPF filter error:", err, "Device:", device.Name, berkeleyPacketFilter)
				wg.Done()
				return
			}

			defer l.mu.Unlock()

			source := gopacket.NewPacketSource(handle, handle.LinkType())
			source.Lazy = true
			source.NoCopy = true

			wg.Done()

			getNextPacket(l, source)
		}(d)
	}
	wg.Wait()
	l.readyChan <- true
}

// IsReady IPListener ready
func (l *IPListener) IsReady() bool {
	select {
	case <-l.readyChan:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// Receiver receive ip packet
func (l *IPListener) Receiver() chan *IPacket {
	return l.ipPacketsChan
}

// isSupport identify the operating system
func isSupport() bool {
	logger.Info("os: ", runtime.GOOS)
	return runtime.GOOS != "darwin"
}

// getBerkeleyPacketFilter berkeley Packet Filter
func getBerkeleyPacketFilter(l *IPListener) string {
	srcHost, dstHost := getSrcHostAndDstHost(l)
	//	var bpf string
	bpf := "(udp dst port " + strconv.Itoa(int(l.port)) + " and (" + dstHost + ")) or (" +
		"udp src port " + strconv.Itoa(int(l.port)) + " and (" + srcHost + "))"
	if !l.trackResponse {
		bpf = "udp dst port " + strconv.Itoa(int(l.port)) + " and (" + dstHost + ")"
	}

	return bpf
}

func getSrcHostAndDstHost(l *IPListener) (string, string) {
	devices, err := findPcapDevices(l.addr)
	if err != nil {
		logger.Error(err)
	}
	var (
		allAddr          []string
		dstHost, srcHost string
	)

	for _, dc := range devices {
		if isLoopback(dc) {
			logger.Info("isLoopback: ", isLoopback(dc))
			for _, addr := range dc.Addresses {
				allAddr = append(allAddr, "(dst host "+addr.IP.String()+" and src host "+addr.IP.String()+")")
			}
			dstHost = strings.Join(allAddr, " or ")
			srcHost = dstHost
		}
		for i, addr := range dc.Addresses {
			dstHost += "dst host " + addr.IP.String()
			srcHost += "src host " + addr.IP.String()
			if i != len(dc.Addresses)-1 {
				dstHost += or
				srcHost += or
			}
		}
	}

	return srcHost, dstHost
}

func setInactiveConfig(device pcap.Interface, inactive *pcap.InactiveHandle) *pcap.InactiveHandle {
	it, err := net.InterfaceByName(device.Name)
	if err != nil {
		_ = inactive.SetSnapLen(65536)
	}
	// Auto-guess max length of ipPacket to capture
	err = inactive.SetSnapLen(it.MTU + 68*2)
	if err != nil {
		logger.Warn("Auto-guess ipPacket max length is failure")
	}
	err = inactive.SetTimeout(-1 * time.Second)
	if err != nil {
		logger.Warn("set timeout is failure")
	}
	err = inactive.SetPromisc(true)
	if err != nil {
		logger.Warn("set promisc is failure")
	}

	return inactive
}

func getNextPacket(l *IPListener, source *gopacket.PacketSource) {
	for {
		packet, err := source.NextPacket()
		if err == io.EOF {
			break
		} else if err != nil {
			logger.Info("NextPacket error:", err)
			continue
		}

		networkLayer := packet.NetworkLayer()
		srcIP := networkLayer.NetworkFlow().Src().Raw()
		dstIP := networkLayer.NetworkFlow().Dst().Raw()
		payload := networkLayer.LayerPayload()

		l.ipPacketsChan <- l.buildPacket(srcIP, dstIP, payload, packet.Metadata().Timestamp)
	}
}
