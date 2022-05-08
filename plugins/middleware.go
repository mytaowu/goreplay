package plugins

import (
	"goreplay/errors"
)

// msgWapper 中间件传输数据对包装
type msgWapper struct {
	msg *Message
	err error
}

// Middleware represents a middleware object
type Middleware struct {
	data        chan *msgWapper
	readers     []PluginReader
	midWareRoot MiddlewareFun
	stop        chan bool // Channel used only to indicate goroutine should shutdown
}

// NewMiddleware returns new middleware
func NewMiddleware(readers []PluginReader,
	midWares []MiddlewareFun) *Middleware {
	m := new(Middleware)
	m.data = make(chan *msgWapper, 1000)
	m.stop = make(chan bool)
	m.readers = readers

	// 构建中间件链条
	m.midWareRoot = midWares[0] // 能进入这里，保证至少有一个中间件
	tempWare := m.midWareRoot
	for i := 1; i < len(midWares); i++ {
		tempWare.SetNextMidWare(midWares[i])
		tempWare = midWares[i]
	}

	// 并发读
	for _, v := range m.readers {
		go func(p PluginReader) {
			m.readFrom(p)
		}(v)
	}

	return m
}

// ReadFrom start a worker to read from this plugin
func (m *Middleware) readFrom(p PluginReader) {
	for {
		msg, err := p.PluginRead()
		m.data <- &msgWapper{msg: msg, err: err}
	}
}

// PluginRead reads message from this plugin
func (m *Middleware) PluginRead() (*Message, error) {
	var msgWapper *msgWapper
	select {
	case <-m.stop:
		return nil, errors.ErrorStopped
	case msgWapper = <-m.data:
	}

	return m.midWareRoot.MidWareHandle(msgWapper.msg, msgWapper.err)
}

// Close closes this plugin
func (m *Middleware) Close() error {
	close(m.stop)
	return nil
}
