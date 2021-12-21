// Package logger provides log output stdout or file api
// call logger's Init function before use logger in init function
package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"goreplay/config"
)

// level define log level type
type level int8

// String 获取错误等级字段
func (l level) String() string {
	switch l {
	case fatalLevel:
		return "FATAL"
	case errorLevel:
		return "ERROR"
	case warnLevel:
		return "WARN"
	case debugLevel1, debugLevel2, debugLevel3:
		return fmt.Sprintf("DEBUG-%d", l)
	case infoLevel:
		fallthrough
	default:
		return "INFO"
	}
}

// log level, default level is 0
const (
	fatalLevel level = iota - 3
	errorLevel
	warnLevel
	infoLevel // 0 is default level
	debugLevel1
	debugLevel2
	debugLevel3
)

var log Logger
var lvl level
var previousDebugTime = time.Now()
var debugMutex sync.Mutex
var once sync.Once

// Init init logger, new zaplog error, use fmt.Println
func Init(logLevel int, logPath string) {
	once.Do(func() {
		lvl = level(logLevel)

		zaplog, err := newLogger(logPath)
		if err == nil {
			log = zaplog.Sugar()
			return
		}

		log = &defLog{}
	})

	print(infoLevel, fmt.Sprintf("logger %T level is %d", log, lvl))
}

// Logger define logger interface
type Logger interface {
	Info(args ...interface{})
}

type defLog struct{}

// Info default log info output
func (d *defLog) Info(args ...interface{}) {
	fmt.Println(args...)
}

// Debug output debug level log
func Debug(args ...interface{}) {
	print(debugLevel1, args...)
}

// Debug2 output debug level log
func Debug2(args ...interface{}) {
	print(debugLevel2, args...)
}

// Debug3 output debug level log
func Debug3(args ...interface{}) {
	print(debugLevel3, args...)
}

// Info output info level log
func Info(args ...interface{}) {
	print(infoLevel, args...)
}

// Warn output warn level log
func Warn(args ...interface{}) {
	print(warnLevel, args...)
}

// Error output error level log
func Error(args ...interface{}) {
	print(errorLevel, args...)
}

// Fatal output panic level log
func Fatal(args ...interface{}) {
	print(fatalLevel, args...)
	os.Exit(1)
}

func print(level level, args ...interface{}) {
	if log == nil {
		Init(config.Settings.Verbose, config.Settings.LogPath)
	}

	if lvl >= level {
		debugMutex.Lock()
		defer debugMutex.Unlock()

		now := time.Now()
		diff := now.Sub(previousDebugTime)
		previousDebugTime = now

		log.Info(fmt.Sprintf("[%s] [elapsed %s]: ", level, diff), args)
	}
}
