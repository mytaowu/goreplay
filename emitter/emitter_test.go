package emitter

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"

	"goreplay/config"
	rerror "goreplay/errors"
	"goreplay/plugins"
	"goreplay/plugins/middleware"
	"goreplay/protocol"
)

type testUnitEmitterSuite struct {
	suite.Suite
	mockErr error
}

func TestUnitEmitter(t *testing.T) {
	suite.Run(t, new(testUnitEmitterSuite))
}

func (s *testUnitEmitterSuite) SetupTest() {
	s.mockErr = errors.New("mock err")
}

func (s *testUnitEmitterSuite) TestStart() {
	wg := new(sync.WaitGroup)
	tests := []struct {
		name       string
		Inputs     *testInput
		Outputs    []plugins.PluginWriter
		settings   Settings
		middleware string
		delta      int
		mock       func() *gomonkey.Patches
		check      func()
	}{
		{
			name: "normal",
			settings: Settings{
				PrettifyHTTP:   false,
				CopyBufferSize: 0,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.Settings.ModifierConfig,
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			delta: 1,
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(ignoreError, func(error) bool { return false })
			},
		},
		{
			name: "splitOut",
			settings: Settings{
				PrettifyHTTP:   false,
				CopyBufferSize: config.Settings.CopyBufferSize,
				Split:          true,
				ModifierConfig: config.Settings.ModifierConfig,
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			delta: 1,
		},
		{
			name: "GC",
			settings: Settings{
				PrettifyHTTP:   false,
				CopyBufferSize: 1,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.Settings.ModifierConfig,
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			mock: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(checkFilteredCount, func(int) bool { return false })
			},
			delta: 1,
		},
		{
			name: "splitOut error",
			settings: Settings{
				PrettifyHTTP:   false,
				CopyBufferSize: 0,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.Settings.ModifierConfig,
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			delta: 1,
		},
		{
			name: "middleware",
			settings: Settings{
				PrettifyHTTP:   config.Settings.PrettifyHTTP,
				CopyBufferSize: config.Settings.CopyBufferSize,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.Settings.ModifierConfig,
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			middleware: "echo",
			mock: func() *gomonkey.Patches {
				patches := gomonkey.ApplyFunc(middleware.NewMiddleware,
					func(string) *middleware.Middleware { return &middleware.Middleware{} })
				patches.ApplyMethod(reflect.TypeOf(&middleware.Middleware{}), "ReadFrom",
					func(*middleware.Middleware, plugins.PluginReader) {})
				return patches

			},
		},
		{
			name: "modifier",
			settings: Settings{
				PrettifyHTTP:   true,
				CopyBufferSize: config.Settings.CopyBufferSize,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.HTTPModifierConfig{Methods: config.HTTPMethods{[]byte("GET")}},
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			delta: 1,
		},
		{
			name: "prettifyHTTP",
			settings: Settings{
				PrettifyHTTP:   true,
				CopyBufferSize: config.Settings.CopyBufferSize,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.Settings.ModifierConfig,
			},
			Inputs: newTestInput(),
			Outputs: []plugins.PluginWriter{newTestOutput(func(*plugins.Message) {
				wg.Done()
			})},
			delta: 1,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.mock != nil {
				patches := tt.mock()
				if patches != nil {
					defer patches.Reset()
				}
			}

			emitter := NewEmitter(tt.settings)
			plug := &plugins.InOutPlugins{
				Inputs:  []plugins.PluginReader{plugins.PluginReader(tt.Inputs)},
				Outputs: tt.Outputs,
			}

			wg.Add(tt.delta)
			plug.All = append(plug.All, tt.Inputs)
			for _, i := range tt.Outputs {
				plug.All = append(plug.All, i)
			}

			go emitter.Start(plug, tt.middleware)

			tt.Inputs.EmitGET()
			wg.Wait()

			if emitter.inOutPlugins == nil {
				emitter.inOutPlugins = new(plugins.InOutPlugins)
			}

			emitter.Close()
		})
	}
}

func (s *testUnitEmitterSuite) TestGarbageCollect() {
	tests := []struct {
		name          string
		requests      map[string]int64
		lastCleanTime int64
		count         int
	}{
		{
			name: "overtime",
			requests: map[string]int64{
				"a": 1,
				"b": 2,
			},
			lastCleanTime: 0,
			count:         5,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			originCount := tt.count

			emitter := NewEmitter(Settings{
				PrettifyHTTP:   config.Settings.PrettifyHTTP,
				CopyBufferSize: config.Settings.CopyBufferSize,
				Split:          config.Settings.SplitOutput,
				ModifierConfig: config.Settings.ModifierConfig,
			})

			emitter.garbageCollect(tt.requests, &tt.lastCleanTime, &tt.count)
			if tt.lastCleanTime == 0 {
				s.Equal(originCount, tt.count)
			} else {
				s.Equal(tt.requests, map[string]int64{})
			}
		})
	}
}

// testInput used for testing purpose, it allows emitting requests on demand
type testInput struct {
	data       chan []byte
	skipHeader bool
	stop       chan bool // Channel used only to indicate goroutine should shutdown
}

// newTestInput constructor for TestInput
func newTestInput() (i *testInput) {
	i = new(testInput)
	i.data = make(chan []byte, 100)
	i.stop = make(chan bool)
	return
}

// PluginRead reads message from this plugin
func (i *testInput) PluginRead() (*plugins.Message, error) {
	var msg plugins.Message
	select {
	case buf := <-i.data:
		msg.Data = buf
		if !i.skipHeader {
			msg.Meta = protocol.
				PayloadHeader(protocol.RequestPayload, protocol.UUID(), time.Now().UnixNano(), -1)
		} else {
			msg.Meta, msg.Data = protocol.PayloadMetaWithBody(msg.Data)
		}

		return &msg, nil
	case <-i.stop:
		return nil, rerror.ErrorStopped
	}
}

// Close closes this plugin
func (i *testInput) Close() error {
	close(i.stop)
	return nil
}

// EmitGET emits GET request without headers
func (i *testInput) EmitGET() {
	i.data <- []byte("GET / HTTP/1.1\r\n\r\n")
}

func (i *testInput) String() string {
	return "Test Input"
}

type writeCallback func(*plugins.Message)

// TestOutput used in testing to intercept any output into callback
type testOutput struct {
	cb writeCallback
}

// newTestOutput constructor for TestOutput, accepts callback which get called on each incoming Write
func newTestOutput(cb writeCallback) plugins.PluginWriter {
	i := new(testOutput)
	i.cb = cb

	return i
}

// PluginWrite write message to this plugin
func (i *testOutput) PluginWrite(msg *plugins.Message) (int, error) {
	i.cb(msg)

	return len(msg.Data) + len(msg.Meta), nil
}

func (i *testOutput) String() string {
	return "Test Output"
}
