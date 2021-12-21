package plugins

import (
	"fmt"
	"net"

	"goreplay/config"
	"goreplay/errors"
	"goreplay/logger"
	"goreplay/protocol"
	"goreplay/udp"
	"goreplay/udp/listener"
)

// UDPInput used for internal communication
type UDPInput struct {
	config.UDPInputConfig
	message       chan *udp.Message
	address       string
	listener      *listener.UDPListener
	trackResponse bool
	stop          chan bool // Channel used only to indicate goroutine should shutdown
	Port          uint16
	Host          string
}

// NewUDPInput constructor for UDPInput, accepts address with port
func NewUDPInput(address string, config config.UDPInputConfig) *UDPInput {
	i := new(UDPInput)
	i.message = make(chan *udp.Message)
	i.address = address
	i.stop = make(chan bool)
	// track response
	logger.Debug3("track response: ", config.TrackResponse)
	i.trackResponse = config.TrackResponse
	i.listen(address)

	return i
}

// PluginRead returns data and details read from plugin
func (i *UDPInput) PluginRead() (*Message, error) {
	var message *udp.Message
	var msg Message
	select {
	case <-i.stop:
		return nil, errors.ErrorStopped
	case message = <-i.message:
	}
	msg.Data = message.Data()
	logger.Debug3(fmt.Sprintf("msg: %s", msg.Data))
	var msgType byte = protocol.ResponsePayload
	if message.IsIncoming {
		msgType = protocol.RequestPayload
	}
	msg.Meta = protocol.PayloadHeader(msgType, message.UUID(), message.Start.UnixNano(),
		message.End.UnixNano()-message.Start.UnixNano())

	return &msg, nil
}

func (i *UDPInput) listen(address string) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		logger.Fatal("input-udp: error while parsing address", err)
	}

	logger.Debug3("Listening for udp traffic on: " + address)

	i.listener = listener.NewUDPListener(host, port, i.trackResponse)
	ch := i.listener.Receiver()
	go func() {
		for {
			select {
			case <-i.stop:
				return
			default:
			}
			// Receiving UDPMessage
			m := <-ch
			i.message <- m
		}
	}()
}

// Close closes this plugin
func (i *UDPInput) Close() error {
	close(i.stop)
	return nil
}

// String input address
func (i *UDPInput) String() string {
	return fmt.Sprintf("Intercepting traffic from %s:%d", i.Host, i.Port)
}
