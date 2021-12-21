// Package middleware provides
package middleware

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"goreplay/errors"
	"goreplay/http"
	"goreplay/protocol"

	"goreplay/config"

	"goreplay/logger"
	"goreplay/plugins"
)

// Middleware represents a middleware object
type Middleware struct {
	command       string
	data          chan *plugins.Message
	Stdin         io.Writer
	Stdout        io.Reader
	commandCancel context.CancelFunc
	stop          chan bool // Channel used only to indicate goroutine should shutdown
}

// NewMiddleware returns new middleware
func NewMiddleware(command string) *Middleware {
	m := new(Middleware)
	m.command = command
	m.data = make(chan *plugins.Message, 1000)
	m.stop = make(chan bool)

	commands := strings.Split(command, " ")
	ctx, cancl := context.WithCancel(context.Background())
	m.commandCancel = cancl
	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)

	m.Stdout, _ = cmd.StdoutPipe()
	m.Stdin, _ = cmd.StdinPipe()

	cmd.Stderr = os.Stderr

	go m.read(m.Stdout)

	go func() {
		err := cmd.Start()

		if err != nil {
			log.Fatal(err)
		}

		err = cmd.Wait()

		if err != nil {
			log.Fatal(err)
		}
	}()

	return m
}

// ReadFrom start a worker to read from this plugin
func (m *Middleware) ReadFrom(plugin plugins.PluginReader) {
	logger.Debug2("[MIDDLEWARE-MASTER] Starting reading from", plugin)
	go m.copy(m.Stdin, plugin)
}

func (m *Middleware) copy(to io.Writer, from plugins.PluginReader) {
	var buf, dst []byte

	for {
		msg, err := from.PluginRead()
		if err != nil {
			return
		}
		if msg == nil || len(msg.Data) == 0 {
			continue
		}

		if config.Settings.PrettifyHTTP {
			buf = http.PrettifyHTTP(msg.Data)
		}
		dst = make([]byte, len(buf)*2+1)
		hex.Encode(dst, buf)
		dst[len(buf)*2] = '\n'

		_, _ = to.Write(dst)

	}
}

func (m *Middleware) read(from io.Reader) {
	reader := bufio.NewReader(from)
	var line []byte
	var e error

	for {
		if line, e = reader.ReadBytes('\n'); e != nil {
			if e == io.EOF {
				continue
			} else {
				break
			}
		}

		buf := make([]byte, len(line)/2-1)
		if _, err := hex.Decode(buf, line[:len(line)-1]); err != nil {
			logger.Debug(fmt.Sprintf("[MIDDLEWARE] failed to decode err: %q", err))
			continue
		}
		var msg plugins.Message
		msg.Meta, msg.Data = protocol.PayloadMetaWithBody(buf)
		select {
		case <-m.stop:
			return
		case m.data <- &msg:
		}
	}
}

// PluginRead reads message from this plugin
func (m *Middleware) PluginRead() (msg *plugins.Message, err error) {
	select {
	case <-m.stop:
		return nil, errors.ErrorStopped
	case msg = <-m.data:
	}

	return
}

// String middleware command string
func (m *Middleware) String() string {
	return fmt.Sprintf("Modifying traffic using %q command", m.command)
}

// Close closes this plugin
func (m *Middleware) Close() error {
	m.commandCancel()
	close(m.stop)
	return nil
}
