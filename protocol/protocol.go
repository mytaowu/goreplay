package protocol

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// These constants help to indicate the type of payload
const (
	RequestPayload          = '1'
	ResponsePayload         = '2'
	ReplayedResponsePayload = '3'

	PayloadSeparator = "\n🐵🙈🙉\n"
)

// RandByte generate random byte slice
func RandByte(len int) []byte {
	b := make([]byte, len/2)
	_, _ = rand.Read(b)

	h := make([]byte, len)
	hex.Encode(h, b)

	return h
}

// UUID generate a uuid string
func UUID() []byte {
	return RandByte(24)
}

// PayloadHeader Timing is request start or round-trip time, depending on payloadType
func PayloadHeader(payloadType byte, uuid []byte, timing int64, latency int64) (header []byte) {
	// Example:
	//  3 f45590522cd1838b4a0d5c5aab80b77929dea3b3 13923489726487326 1231\n
	return []byte(fmt.Sprintf("%c %s %d %d\n", payloadType, uuid, timing, latency))
}

// PayloadMeta create payload meta
func PayloadMeta(payload []byte) [][]byte {
	headerSize := bytes.IndexByte(payload, '\n')
	if headerSize < 0 {
		return nil
	}

	return bytes.Split(payload[:headerSize], []byte{' '})
}

// PayloadMetaWithBody parse body of payload
func PayloadMetaWithBody(payload []byte) (meta, body []byte) {
	if i := bytes.IndexByte(payload, '\n'); i > 0 && len(payload) > i+1 {
		meta = payload[:i+1]
		body = payload[i+1:]

		return
	}

	// we assume the message did not have meta data
	return nil, payload
}

// PayloadID get payloadID of gor message
func PayloadID(payload []byte) (id []byte) {
	meta := PayloadMeta(payload)

	if len(meta) < 2 {
		return
	}
	return meta[1]
}

// IsOriginPayload if is request or response of tcp
func IsOriginPayload(payload []byte) bool {
	return payload[0] == RequestPayload || payload[0] == ResponsePayload
}

// IsRequestPayload if is request of tcp
func IsRequestPayload(payload []byte) bool {
	return payload[0] == RequestPayload
}
