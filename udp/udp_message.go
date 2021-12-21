package udp

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"goreplay/config"
	"goreplay/logger"
)

// Message is the representation of a udp message
type Message struct {
	IsIncoming bool
	Start      time.Time
	End        time.Time
	SrcPort    uint16
	DstPort    uint16
	length     uint16
	checksum   uint16
	data       []byte
	isRequest  bool
	isResponse bool
}

// NewUDPMessage is new udp message
func NewUDPMessage(data []byte, isIncoming bool) (m *Message) {
	m = &Message{}
	udp := &layers.UDP{}
	err := udp.DecodeFromBytes(data, gopacket.NilDecodeFeedback)
	if err != nil {
		logger.Debug3("Error decode udp message", err)
	}
	m.SrcPort = uint16(udp.SrcPort)
	m.DstPort = uint16(udp.DstPort)
	m.isRequest = IsRequest(m.DstPort)
	m.isResponse = IsResponse(m.SrcPort)
	m.length = udp.Length
	m.checksum = udp.Checksum
	m.data = udp.Payload
	m.IsIncoming = isIncoming
	return
}

// UUID returns the UUID of a UDP request and its response.
func (m *Message) UUID() []byte {
	var key []byte

	if m.isRequest {
		key = strconv.AppendUint(key, uint64(m.DstPort), 10)
		key = strconv.AppendUint(key, uint64(m.SrcPort), 10)
	}

	if m.isResponse {
		key = strconv.AppendUint(key, uint64(m.SrcPort), 10)
		key = strconv.AppendUint(key, uint64(m.DstPort), 10)
	}

	uuid := make([]byte, 40)
	sha := sha1.Sum(key)
	hex.Encode(uuid, sha[:20])
	return uuid
}

// Data returns data in this message
func (m *Message) Data() []byte {
	return m.data
}

// IsRequest if is req of udp packet
func IsRequest(port uint16) bool {
	configPort, err := getConfigPort()
	if err != nil {
		logger.Error("invalid port")
	}

	return configPort == port
}

// IsResponse is resp of udp packet
func IsResponse(port uint16) bool {
	configPort, err := getConfigPort()
	if err != nil {
		logger.Error("invalid port")
	}

	return configPort == port
}

func getConfigPort() (uint16, error) {
	configAddr := config.Settings.InputUDP.String()
	addr := strings.Split(strings.ReplaceAll(configAddr, "]", ""), ":")
	if len(addr) < 2 {
		return 0, errors.New("invalid addr")
	}

	intPort, err := strconv.Atoi(addr[1])
	if err != nil {
		return 0, errors.New("invalid addr")
	}

	return uint16(intPort), nil
}
