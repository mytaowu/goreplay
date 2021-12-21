package client

import (
	"crypto/tls"
	"io"
	"net"
	"syscall"
	"time"

	"goreplay/logger"
)

const (
	readChunkSize   = 64 * 1024
	maxResponseSize = 1073741824
)

// TCPClientConfig client configuration
type TCPClientConfig struct {
	Debug              bool
	ConnectionTimeout  time.Duration
	Timeout            time.Duration
	ResponseBufferSize int
	Secure             bool
}

// TCPClient client connection properties
type TCPClient struct {
	baseURL string
	addr    string
	conn    net.Conn
	respBuf []byte
	config  *TCPClientConfig
}

// NewTCPClient returns new TCPClient
func NewTCPClient(addr string, config *TCPClientConfig) *TCPClient {
	if config.Timeout.Nanoseconds() == 0 {
		config.Timeout = 5 * time.Second
	}

	config.ConnectionTimeout = config.Timeout

	if config.ResponseBufferSize == 0 {
		config.ResponseBufferSize = 100 * 1024 // 100kb
	}

	client := &TCPClient{config: config, addr: addr}
	client.respBuf = make([]byte, config.ResponseBufferSize)

	return client
}

// Connect creates a tcp connection of the client
func (c *TCPClient) Connect() (err error) {
	c.Disconnect()

	c.conn, err = net.DialTimeout("tcp", c.addr, c.config.ConnectionTimeout)

	if c.config.Secure {
		tlsConn := tls.Client(c.conn, &tls.Config{InsecureSkipVerify: true})

		if err = tlsConn.Handshake(); err != nil {
			return
		}

		c.conn = tlsConn
	}

	return
}

// Disconnect closes the client connection
func (c *TCPClient) Disconnect() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		logger.Debug("[TCPClient] Disconnected: ", c.baseURL)
	}
}

func (c *TCPClient) isAlive() bool {
	one := make([]byte, 1)

	// Ready 1 byte from socket without timeout to check if it not closed
	_ = c.conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	_, err := c.conn.Read(one)

	if err == nil {
		return true
	} else if err == io.EOF {
		logger.Debug("[TCPClient] connection closed, reconnecting")
		return false
	} else if err == syscall.EPIPE {
		logger.Debug("Detected broken pipe.", err)
		return false
	}

	return true
}

func (c *TCPClient) doConnect() error {
	if c.conn == nil || !c.isAlive() {
		if err := c.Connect(); err != nil {
			logger.Debug("[TCPClient] Connection error:", err)
			return err
		}
	}

	return nil
}

func recoverSend(data []byte) {
	if r := recover(); r != nil {
		if _, ok := r.(error); !ok {
			logger.Debug("[TCPClient] Failed to send request: ", string(data))
		}
	}
}

// Send sends data over created tcp connection
func (c *TCPClient) Send(data []byte) (response []byte, err error) {
	// Don't exit on panic
	defer recoverSend(data)

	if err = c.doConnect(); err != nil {
		return nil, err
	}

	timeout := time.Now().Add(c.config.Timeout)

	_ = c.conn.SetWriteDeadline(timeout)

	if _, err = c.conn.Write(data); err != nil {
		logger.Error("[TCPClient] Write error:", err, c.baseURL)
		return
	}

	tcpConn, _ := c.conn.(*net.TCPConn)

	if err = tcpConn.CloseWrite(); err != nil {
		logger.Error("[TCPClient] CloseWrite error:", err)
		return
	}

	var readBytes, n int
	var currentChunk []byte
	timeout = time.Now().Add(c.config.Timeout)

	for {
		_ = c.conn.SetReadDeadline(timeout)
		if readBytes < len(c.respBuf) {
			n, err = c.conn.Read(c.respBuf[readBytes:])
			readBytes += n

			if err != nil {
				if err == io.EOF {
					err = nil
				}
				break
			}
		} else {
			if currentChunk == nil {
				currentChunk = make([]byte, readChunkSize)
			}

			n, err = c.conn.Read(currentChunk)

			if err == io.EOF {
				break
			} else if err != nil {
				logger.Error("[TCPClient] Read the whole body error:", err, c.baseURL)
				break
			}

			readBytes += int(n)
		}

		if readBytes >= maxResponseSize {
			logger.Error("[TCPClient] Body is more than the max size", maxResponseSize, c.baseURL)
			break
		}

		// For following chunks expect less timeout
		timeout = time.Now().Add(c.config.Timeout / 5)
	}

	if err != nil {
		logger.Error("[TCPClient] Response read error", err, c.conn, readBytes)
		return
	}

	if readBytes > len(c.respBuf) {
		readBytes = len(c.respBuf)
	}

	payload := make([]byte, readBytes)
	copy(payload, c.respBuf[:readBytes])

	return payload, err
}
