package capture

import (
	"time"

	"github.com/google/gopacket"
)

// Socket is any interface that defines the behaviors of Socket
type Socket interface {
	// ReadPacketData read socket packet
	ReadPacketData() ([]byte, gopacket.CaptureInfo, error)
	// WritePacketData write data to socket
	WritePacketData([]byte) error
	// SetBPFFilter set socket bpf filter
	SetBPFFilter(string) error
	// SetPromiscuous set promiscous mode to the required value
	SetPromiscuous(bool) error
	// SetSnapLen set the maximum capture length to the given value
	SetSnapLen(int) error
	// GetSnapLen get the maximum capture length
	GetSnapLen() int
	// SetTimeout sets poll wait timeout for the socket
	SetTimeout(time.Duration) error
	// SetLoopbackIndex sets reading net interface index
	SetLoopbackIndex(i int32)
	// Close closes the underlying socket
	Close() error
}
