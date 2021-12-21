package tcp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"github.com/google/gopacket"

	"goreplay/logger"
	"goreplay/size"
	"goreplay/tcp/ack"
)

// 常量
const (
	bufferSize = 1000 * 20 // buffer 的初始化大小，这里只是简单的取经验值 2k
)

// Stats every message carry its own stats object
type Stats struct {
	LostData   int
	Length     int       // length of the data
	Start      time.Time // first packet's timestamp
	End        time.Time // last packet's timestamp
	SrcAddr    string
	DstAddr    string
	IsIncoming bool
	TimedOut   bool // timeout before getting the whole message
	Truncated  bool // last packet truncated due to max message size
	IPversion  byte
}

// Message is the representation of a tcp message
type Message struct {
	reqRspKey string // reqRspKey message request and response match key
	packets   []*Packet
	pool      *MessagePool
	buf       *bytes.Buffer
	feedback  interface{}
	Stats
}

// NewMessage ...
func NewMessage(srcAddr, dstAddr string, ipVersion uint8, len int) (m *Message) {
	m = new(Message)
	m.DstAddr = dstAddr
	m.SrcAddr = srcAddr
	m.IPversion = ipVersion

	if len <= 0 {
		len = bufferSize
	}

	m.buf = bytes.NewBuffer(make([]byte, 0, len))
	return
}

// Reset resets the message
func (m *Message) Reset() {
	if m.pool != nil {
		m.pool = nil
	}
}

// UUID returns the UUID of a TCP request and its response.
func (m *Message) UUID() []byte {
	key := m.reqRspKey
	pckt := m.packets[0]

	// check if response or request have generated the ID before.
	if m.pool.uuidCache != nil {
		// if m.IsIncoming {
		// 	key = m.pool.framer.MessageKey(pckt, false).String()
		// } else {
		// 	key = m.pool.framer.MessageKey(pckt, true).String()
		// }

		if uuidHex, err := m.pool.uuidCache.Get([]byte(key)); err == nil {
			m.pool.uuidCache.Del([]byte(key))
			return uuidHex
		}
	}

	id := make([]byte, 12)
	binary.BigEndian.PutUint32(id, pckt.Seq)
	tStamp := m.End.UnixNano()
	binary.BigEndian.PutUint64(id[4:], uint64(tStamp))
	uuidHex := make([]byte, 24)
	hex.Encode(uuidHex[:], id[:])

	if m.pool.uuidCache != nil {
		_ = m.pool.uuidCache.Set([]byte(key), uuidHex, int(m.pool.messageExpire.Seconds()))
	}

	return uuidHex
}

// ConnectionID returns the ID of a TCP connection.
func (m *Message) ConnectionID() string {
	return DefaultMessageKey(m.packets[0], false).String()
}

func (m *Message) add(pckt *Packet) {
	m.Length += len(pckt.Payload)
	m.LostData += int(pckt.Lost)
	m.packets = append(m.packets, pckt)
	m.End = pckt.Timestamp
	m.buf.Write(pckt.Payload)
}

// Packets returns packets of the message
func (m *Message) Packets() []*Packet {
	return m.packets
}

// Data returns data in this message
func (m *Message) Data() []byte {
	return m.buf.Bytes()
}

// Truncate returns this message data length
func (m *Message) Truncate(n int) {
	m.buf.Truncate(n)
}

// SetFeedback set feedback/data that can be used later, e.g with End or Start hint
func (m *Message) SetFeedback(feedback interface{}) {
	m.feedback = feedback
}

// Feedback returns feedback associated to this message
func (m *Message) Feedback() interface{} {
	return m.feedback
}

// Sort a helper to sort packets
func (m *Message) Sort() {
	sort.SliceStable(m.packets, func(i, j int) bool { return m.packets[i].Seq < m.packets[j].Seq })
}

// Handler message handler
type Handler func(*Message)

// HintEnd hints the pool to stop the session, see MessagePool.End
// when set, it will be executed before checking FIN or RST flag
type HintEnd func(*Message) bool

// HintStart hints the pool to start the reassembling the message, see MessagePool.Start
// when set, it will be called after checking SYN flag
type HintStart func(*Packet) (isIncoming, isOutgoing bool)

// MessagePool holds data of all tcp messages in progress(still receiving/sending packets).
// message is identified by its source port and dst port, and last 4bytes of src IP.
type MessagePool struct {
	sync.Mutex
	maxSize        size.Size // maximum message size, default 5mb
	pool           map[string]*Message
	uuidCache      *freecache.Cache
	handler        Handler
	messageExpire  time.Duration // the maximum time to wait for the final packet, minimum is 100ms
	packetPool     *sync.Pool
	address        string // 录制服务地址，IP:PORT
	protocol       string
	framer         Framer
	longConnection bool
}

// NewMessagePool returns a new instance of message pool
func NewMessagePool(maxSize size.Size, messageExpire time.Duration, handler Handler) (pool *MessagePool) {
	pool = new(MessagePool)
	pool.handler = handler
	pool.messageExpire = time.Millisecond * 100
	if pool.messageExpire < messageExpire {
		pool.messageExpire = messageExpire
	}
	pool.maxSize = maxSize
	if pool.maxSize < 1 {
		pool.maxSize = 5 << 20
	}
	pool.pool = make(map[string]*Message)
	pool.packetPool = &sync.Pool{New: func() interface{} {
		return new(Packet)
	}}

	return pool
}

// MessageGroupBy group by all the pack
func (pool *MessagePool) MessageGroupBy(p *Packet) map[string]*Packet {
	if pool.framer != nil {
		return pool.framer.MessageGroupBy(p)
	}

	return map[string]*Packet{pool.MessageKey(p, false).String(): p}
}

// MessageKey generate key for msg
func (pool *MessagePool) MessageKey(pckt *Packet, isPeer bool) *big.Int {
	if pool.framer != nil {
		return pool.framer.MessageKey(pckt, isPeer)
	}

	return DefaultMessageKey(pckt, isPeer)
}

func (pool *MessagePool) needCacheServerAck(pckt *Packet) bool {
	return pool.longConnection && pckt.ACK && IsResponse(pckt, pool.address) && len(pckt.Payload) > 0
}

func (pool *MessagePool) afterHandler(pckt *Packet) {
	key := DefaultMessageKey(pckt, false).String()
	// ack server -> client, exclude establish connection ack

	if pool.needCacheServerAck(pckt) {
		// update server ack after every response, it will be next request seq
		ack.PutServerAck(key, pckt.Ack)

		logger.Debug3(fmt.Sprintf("cache response ack: %s -> %s, key: %s, ack: %d",
			pckt.Src(), pckt.Dst(), key, pckt.Ack), pckt.Flag())
	}
}

// Handler returns packet handler
func (pool *MessagePool) Handler(packet gopacket.Packet) {
	var in, out bool
	pckt, err := pool.ParsePacket(packet)
	if err != nil {
		logger.Debug3(fmt.Sprintf("error decoding packet(%dBytes):%s\n", packet.Metadata().CaptureLength, err))
		return
	}
	if pckt == nil {
		return
	}

	logger.Debug3(fmt.Sprintf("payload %s->%s seq:%d ack: %d, len: %d, pLen: %d, ts: %d, sum: %d, %s",
		pckt.Src(), pckt.Dst(), pckt.Seq, pckt.Ack, pckt.Length, len(pckt.Payload), pckt.Timestamp.Nanosecond(),
		pckt.Checksum, hex.EncodeToString(pckt.Payload)))

	pool.Lock()
	defer pool.Unlock()

	defer func(p *Packet) {
		pool.afterHandler(p)
	}(pckt)

	packsMap := pool.MessageGroupBy(pckt)
	if packsMap == nil {
		logger.Debug3(fmt.Sprintf("packsMap: %v", packsMap))
		return
	}

	for key, itemPckt := range packsMap {
		m, ok := pool.pool[key]

		if itemPckt.RST {
			if ok {
				pool.dispatch(key, m)
			}

			key = pool.MessageKey(itemPckt, true).String()

			m, ok = pool.pool[key]
			if ok {
				pool.dispatch(key, m)
			}
			go logger.Debug3(fmt.Sprintf("RST flag from %s to %s at %s\n",
				itemPckt.Src(), itemPckt.Dst(), itemPckt.Timestamp))
			continue
		}

		switch {
		case ok:
			pool.addPacket(key, m, itemPckt)
			continue
		case itemPckt.SYN:
			in = !itemPckt.ACK
		case pool.framer != nil:
			if in, out = pool.framer.Start(itemPckt); !(in || out) {
				logger.Debug(fmt.Sprintf("packet is not %s frame start: %t, %t", pool.protocol, in, out))
				continue
			}
		default:
			continue
		}

		// 指定 buffer 的初始化大小，减少扩容次数，这里无法知道最终的 buffer 应该多大，payload 的 3 倍只是经验值，不一定适合所有场景
		m = NewMessage(itemPckt.Src(), itemPckt.Dst(), itemPckt.Version, 3*len(itemPckt.Payload))
		m.IsIncoming = in
		if pool.framer == nil {
			// response get peer's key(request key)
			m.reqRspKey = DefaultMessageKey(pckt, !in).String()
		} else {
			m.reqRspKey = pool.framer.ReqRspKey(pckt)
		}

		logger.Debug3("first addPacket message:", key, itemPckt.Src(), itemPckt.Dst(), itemPckt.Flag(), m.reqRspKey)

		pool.pool[key] = m
		m.Start = itemPckt.Timestamp
		m.pool = pool
		pool.addPacket(key, m, itemPckt)
	}

}

// MatchUUID instructs the pool to use same UUID for request and responses
// this function should be called at initial stage of the pool
func (pool *MessagePool) MatchUUID(match bool) {
	if match {
		// 20M
		pool.uuidCache = freecache.NewCache(20 * 1024 * 1024)
		return
	}
	pool.uuidCache = nil
}

func (pool *MessagePool) dispatch(key string, m *Message) {
	delete(pool.pool, key)
	pool.handler(m)
}

func (pool *MessagePool) shouldDispatch(m *Message) bool {
	if m == nil {
		return false
	}

	return time.Since(m.Start) > pool.messageExpire
}

func (pool *MessagePool) addPacket(key string, m *Message, pckt *Packet) {
	trunc := m.Length + len(pckt.Payload) - int(pool.maxSize)
	if trunc > 0 {
		m.Truncated = true
		pckt.Payload = pckt.Payload[:int(pool.maxSize)-m.Length]
	}

	logger.Debug3("addPacket key:", key, pckt.Src(), pckt.Dst(), pckt.Flag())

	m.add(pckt)

	switch {
	// if one of this cases matches, we dispatch the message
	case trunc >= 0:
	case pckt.FIN:
	case pool.framer != nil && pool.framer.End(m):
		// ack client -> server, exclude disconnect ack
		logger.Debug3(fmt.Sprintf("end of %s -> %s key: %s, flag: %s", pckt.Src(), pckt.Dst(), key, pckt.Flag()))
		if pool.longConnection && IsRequest(pckt, pool.address) && !pckt.FIN {
			// update client ack after every request when request end, it is this request's response start seq
			ack.PutClientAck(DefaultMessageKey(pckt, false).String(), pckt.Ack)

			logger.Debug3(fmt.Sprintf("cache request ack: %s -> %s, key: %s, ack: %d",
				pckt.Src(), pckt.Dst(), key, pckt.Ack), pckt.Flag())
		}
	case pool.shouldDispatch(m):
		// avoid dispatching message twice
		if _, ok := pool.pool[key]; !ok {
			return
		}
		m.TimedOut = true
	default:
		// continue to receive packets
		logger.Debug3(fmt.Sprintf("default continue to receive packets, key %s m.packets length: %d",
			key, len(m.Packets())))
		return
	}

	logger.Debug3("addPacket -> dispatch", pckt.Src(), pckt.Dst(), pckt.Flag(), "Packets", len(m.Packets()),
		hex.EncodeToString(m.buf.Bytes()))

	pool.dispatch(key, m)
}

// Address listen destination address
func (pool *MessagePool) Address(address string) {
	pool.address = address
}

// Protocol record business protocol
func (pool *MessagePool) Protocol(protocol string) {
	if protocol == "" {
		log.Fatal("please specify input-raw-protocol")
	}
	pool.protocol = protocol
	// get record protocol frame builder
	fb := GetFramerBuilder(protocol)
	if fb != nil {
		pool.longConnection = true
		pool.framer = fb.New(pool.address)
	}

	logger.Debug3("longConnection:", pool.longConnection, protocol)
}

func _uint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}
