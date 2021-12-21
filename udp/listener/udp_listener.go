package listener

import (
	"fmt"
	"strconv"

	"goreplay/logger"
	"goreplay/udp"
)

// UDPListener is udp listener
type UDPListener struct {
	address      string
	port         uint16
	messagesChan chan *udp.Message
	underlying   *IPListener
}

// NewUDPListener new udp listener
func NewUDPListener(address string, port string, trackResponse bool) (l *UDPListener) {
	l = &UDPListener{}
	l.messagesChan = make(chan *udp.Message, 10000)
	l.address = address
	intPort, err := strconv.Atoi(port)
	if err != nil {
		logger.Error(fmt.Sprintf("Invaild Port: %s, %v\n", port, err))
		return
	}
	l.port = uint16(intPort)
	l.underlying = NewIPListener(address, l.port, trackResponse)

	if !l.underlying.IsReady() {
		logger.Error("IP Listener is not ready after 5 seconds")
		return
	}
	go l.recv()

	return
}

func (l *UDPListener) parseUDPPacket(packet *IPacket) (message *udp.Message) {
	data := packet.payload
	message = udp.NewUDPMessage(data, false)
	if message.DstPort == l.port {
		message.IsIncoming = true
	}
	message.Start = packet.timestamp
	return
}

func (l *UDPListener) recv() {
	for {
		ipPacketsChan := l.underlying.Receiver()
		if packet := <-ipPacketsChan; true {
			message := l.parseUDPPacket(packet)
			l.messagesChan <- message
		}
	}
}

// Receiver receive udp message
func (l *UDPListener) Receiver() chan *udp.Message {
	return l.messagesChan
}
