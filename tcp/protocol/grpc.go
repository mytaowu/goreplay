package protocol

import (
	"bytes"
	"encoding/hex"
	"io"
	"math/big"
	"strconv"

	"goreplay/logger"

	"github.com/golang/groupcache/lru"

	"golang.org/x/net/http2"

	"goreplay/framer"
	"goreplay/tcp"
)

func init() {
	tcp.RegisterFramerBuilder("grpc", &grpcFramerBuilder{})
}

// grpcFramerBuilder grpc framer builder
type grpcFramerBuilder struct{}

type grpcFramer struct {
	clientStreamCache *lru.Cache
	serverStreamCache *lru.Cache
	tcp.CommonFramer
}

// New 新建 grpc framer
func (fb *grpcFramerBuilder) New(listenAddr string) tcp.Framer {
	return &grpcFramer{
		clientStreamCache: lru.New(65535),
		serverStreamCache: lru.New(65535),
		CommonFramer:      tcp.CommonFramer{ListenAddr: listenAddr},
	}
}

// MessageGroupBy group by the packet by message key
func (g *grpcFramer) MessageGroupBy(pckt *tcp.Packet) map[string]*tcp.Packet {
	groupMap := make(map[string]*tcp.Packet)

	if pckt == nil || len(pckt.Payload) == 0 {
		return groupMap
	}

	payload := bytes.Replace(pckt.Payload, []byte(http2.ClientPreface), []byte{}, 1)
	srcKey := tcp.DefaultMessageKey(pckt, false)
	fr := framer.NewHTTP2Framer(payload, srcKey.String(), false)

	for {
		framePayload, frame, err := fr.ReadFrameAndBytes()
		if err != nil {
			if err != io.EOF {
				logger.Debug3("grpcFramer read frame err: ", hex.EncodeToString(pckt.Payload), err)
			}

			break
		}

		if hf, ok := frame.(*http2.MetaHeadersFrame); ok {
			framePayload, err = framer.ReEncodeMetaHeadersFrame(hf)
			if err != nil {
				logger.Debug("reEncodeMetaHeadersFrame err: ", err)
				return map[string]*tcp.Packet{}
			}
		}

		if frame.Header().StreamID > 0 {
			newKey := big.NewInt(srcKey.Int64())
			newKey.Lsh(newKey, 32)
			newKey.Or(newKey, big.NewInt(int64(frame.Header().StreamID)))
			cachePack, ok := groupMap[newKey.String()]

			if ok {
				// fixme cachePack Length是否要改
				cachePack.Payload = append(cachePack.Payload, framePayload...)
			} else {
				cp := pckt.Copy()
				cp.Payload = framePayload
				groupMap[newKey.String()] = cp
			}
		}
	}

	return groupMap
}

// ReqRspKey key for both req and rsp
func (g *grpcFramer) ReqRspKey(pckt *tcp.Packet) string {
	isOut := pckt.Src() == g.ListenAddr
	// if response get peer key, keep it is the same of request key
	return g.MessageKey(pckt, isOut).String()
}

// MessageKey key of the protocol
func (g *grpcFramer) MessageKey(pckt *tcp.Packet, isPeer bool) *big.Int {
	oldKey := tcp.DefaultMessageKey(pckt, isPeer)
	fr := framer.NewHTTP2Framer(pckt.Payload, "", false)
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			if err != io.EOF {
				logger.Debug3("grpcFramer messageKey read frame err: ", hex.EncodeToString(pckt.Payload))
			}

			break
		}

		if frame.Header().StreamID > 0 {
			newKey := big.NewInt(oldKey.Int64())
			newKey.Lsh(newKey, 32)
			newKey.Or(newKey, big.NewInt(int64(frame.Header().StreamID)))

			return newKey
		}
	}
	return oldKey
}

// Start hints message pool to start the reassembling the message
func (g *grpcFramer) Start(pckt *tcp.Packet) (isIncoming, isOutgoing bool) {
	if len(pckt.Payload) <= 0 {
		return
	}

	isIn, isOut := g.InOut(pckt)
	pckt.Payload = bytes.Replace(pckt.Payload, []byte(http2.ClientPreface), []byte{}, 1)
	fr := framer.NewHTTP2Framer(pckt.Payload, "", false)

	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			break
		}

		if frame.Header().StreamID != 0 {
			if isIn {

				// 是请求  而且第一次见到这个streamID
				isIncoming = !g.isInCache(pckt.Src(), pckt.Dst(), uint(frame.Header().StreamID), "isIn")
				g.cacheStreamID(pckt.Src(), pckt.Dst(), uint(frame.Header().StreamID), "isIn")
			} else if isOut {
				// 是响应  而且第一次见到这个streamID
				isOutgoing = !g.isInCache(pckt.Src(), pckt.Dst(), uint(frame.Header().StreamID), "isOut")
				g.cacheStreamID(pckt.Src(), pckt.Dst(), uint(frame.Header().StreamID), "isOut")
			}

			return
		}
	}

	return
}

// End hints message pool to stop the session
func (g *grpcFramer) End(msg *tcp.Message) bool {
	if msg.Length <= 0 {
		return false
	}

	// grpc last packet
	packet := msg.Packets()[len(msg.Packets())-1]
	if packet == nil || len(packet.Payload) <= 0 {
		return false
	}

	packet.Payload = bytes.Replace(packet.Payload, []byte(http2.ClientPreface), []byte{}, 1)

	fr := framer.NewHTTP2Framer(packet.Payload, "", false)
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			break
		}
		if frame.Header().Flags.Has(http2.FlagDataEndStream) || frame.Header().Flags.Has(http2.FlagHeadersEndStream) {
			return true
		}
	}

	return false
}

func cacheKey(src string, dst string, streamID uint, isIn string) string {
	return src + "_" + dst + "_" + strconv.Itoa(int(streamID)) + "_" + isIn
}

func (g *grpcFramer) isInCache(src string, dst string, streamID uint, isIn string) bool {
	_, ok := g.clientStreamCache.Get(cacheKey(src, dst, streamID, isIn))

	return ok
}

func (g *grpcFramer) cacheStreamID(src string, dst string, streamID uint, isIn string) {
	g.clientStreamCache.Add(cacheKey(src, dst, streamID, isIn), "")
}
