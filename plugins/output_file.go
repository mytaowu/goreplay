package plugins

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goreplay/config"
	"goreplay/logger"
	"goreplay/protocol"
	"goreplay/size"
)

var dateFileNameFuncs = map[string]func(*FileOutput) string{
	"%Y":  func(o *FileOutput) string { return time.Now().Format("2006") },
	"%m":  func(o *FileOutput) string { return time.Now().Format("01") },
	"%d":  func(o *FileOutput) string { return time.Now().Format("02") },
	"%H":  func(o *FileOutput) string { return time.Now().Format("15") },
	"%M":  func(o *FileOutput) string { return time.Now().Format("04") },
	"%S":  func(o *FileOutput) string { return time.Now().Format("05") },
	"%NS": func(o *FileOutput) string { return fmt.Sprint(time.Now().Nanosecond()) },
	"%r":  func(o *FileOutput) string { return string(o.currentID) },
	"%t":  func(o *FileOutput) string { return string(o.payloadType) },
}

// FileOutput output plugin
type FileOutput struct {
	sync.RWMutex
	pathTemplate   string
	currentName    string
	file           *os.File
	QueueLength    int
	chunkSize      int
	writer         io.Writer
	requestPerFile bool
	currentID      []byte
	payloadType    []byte
	closed         bool
	totalFileSize  size.Size

	config *config.FileOutputConfig
}

// NewFileOutput constructor for FileOutput, accepts path
func NewFileOutput(pathTemplate string, config *config.FileOutputConfig) *FileOutput {
	o := new(FileOutput)
	o.pathTemplate = pathTemplate
	o.config = config
	o.updateName()

	if strings.Contains(pathTemplate, "%r") {
		o.requestPerFile = true
	}

	if config.FlushInterval == 0 {
		config.FlushInterval = 100 * time.Millisecond
	}

	go func() {
		for {
			time.Sleep(config.FlushInterval)
			if o.IsClosed() {
				break
			}

			o.updateName()
			o.flush()
		}
	}()

	return o
}

func getFileIndex(name string) int {
	ext := filepath.Ext(name)
	withoutExt := strings.TrimSuffix(name, ext)

	if idx := strings.LastIndex(withoutExt, "_"); idx != -1 {
		if i, err := strconv.Atoi(withoutExt[idx+1:]); err == nil {
			return i
		}
	}

	return -1
}

func setFileIndex(name string, idx int) string {
	idxS := strconv.Itoa(idx)
	ext := filepath.Ext(name)
	withoutExt := strings.TrimSuffix(name, ext)

	if i := strings.LastIndex(withoutExt, "_"); i != -1 {
		if _, err := strconv.Atoi(withoutExt[i+1:]); err == nil {
			withoutExt = withoutExt[:i]
		}
	}

	return withoutExt + "_" + idxS + ext
}

func withoutIndex(s string) string {
	if i := strings.LastIndex(s, "_"); i != -1 {
		return s[:i]
	}

	return s
}

type sortByFileIndex []string

// Len 快排用长度方法
func (s sortByFileIndex) Len() int {
	return len(s)
}

// Swap 快排用交换方法
func (s sortByFileIndex) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less 快排用比较方法
func (s sortByFileIndex) Less(i, j int) bool {
	if withoutIndex(s[i]) == withoutIndex(s[j]) {
		return getFileIndex(s[i]) < getFileIndex(s[j])
	}

	return s[i] < s[j]
}

func (o *FileOutput) filename() string {
	o.RLock()
	defer o.RUnlock()

	path := o.pathTemplate

	for name, fn := range dateFileNameFuncs {
		path = strings.Replace(path, name, fn(o), -1)
	}

	if !o.config.Append {
		nextChunk := false
		if o.hasNextChunk() {
			nextChunk = true
		}

		ext := filepath.Ext(path)
		withoutExt := strings.TrimSuffix(path, ext)

		if matches, err := filepath.Glob(withoutExt + "*" + ext); err == nil {
			if len(matches) == 0 {
				return setFileIndex(path, 0)
			}

			sort.Sort(sortByFileIndex(matches))

			last := matches[len(matches)-1]
			fileIndex := 0
			if idx := getFileIndex(last); idx != -1 {
				fileIndex = idx
				if nextChunk {
					fileIndex++
				}
			}

			return setFileIndex(last, fileIndex)
		}
	}

	return path
}

func (o *FileOutput) hasNextChunk() bool {
	// 若队列未到头则有下一个chunk
	// 若chunkSize>0则存在下一个chunk
	// 若currentName为空，则存在下一个chunk
	if o.currentName == "" ||
		((o.config.QueueLimit > 0 && o.QueueLength >= o.config.QueueLimit) ||
			(o.config.SizeLimit > 0 && o.chunkSize >= int(o.config.SizeLimit))) {

		return true
	}

	return false
}

func (o *FileOutput) updateName() {
	name := filepath.Clean(o.filename())

	o.Lock()
	o.currentName = name
	o.Unlock()
}

// PluginWrite writes message to this plugin
func (o *FileOutput) PluginWrite(msg *Message) (int, error) {
	var (
		length     int
		tempLength int
		err        error
	)
	if o.requestPerFile {
		o.Lock()
		meta := protocol.PayloadMeta(msg.Meta)
		o.currentID = meta[1]
		o.payloadType = meta[0]
		o.Unlock()
	}

	o.updateName()
	o.Lock()
	defer o.Unlock()

	if o.file == nil || o.currentName != o.file.Name() {
		_ = o.closeLocked()
		o.file, err = os.OpenFile(o.currentName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
		_ = o.file.Sync()

		if strings.HasSuffix(o.currentName, ".gz") {
			o.writer = gzip.NewWriter(o.file)
		} else {
			o.writer = bufio.NewWriter(o.file)
		}

		if err != nil {
			logger.Fatal("[OUTPUT-FILE] Cannot open file %q. Error: ", o.currentName, err)
		}

		o.QueueLength = 0
	}

	length, _ = o.writer.Write(msg.Meta)
	tempLength, _ = o.writer.Write(msg.Data)
	length += tempLength
	tempLength, _ = o.writer.Write(payloadSeparatorAsBytes)
	length += tempLength
	o.totalFileSize += size.Size(length)
	o.QueueLength++

	if config.Settings.OutputFileConfig.OutputFileMaxSize > 0 &&
		o.totalFileSize >= config.Settings.OutputFileConfig.OutputFileMaxSize {

		return length, fmt.Errorf("file output reached size limit")
	}

	return length, err
}

func (o *FileOutput) flush() {
	// Don't exit on panic
	defer func() {
		if r := recover(); r != nil {
			logger.Info("[OUTPUT-FILE] PANIC while file flush: ", r, o, string(debug.Stack()))
		}
	}()

	o.Lock()
	defer o.Unlock()

	if o.file != nil {
		if strings.HasSuffix(o.currentName, ".gz") {
			_ = o.writer.(*gzip.Writer).Flush()
		} else {
			_ = o.writer.(*bufio.Writer).Flush()
		}

		if stat, err := o.file.Stat(); err == nil {
			o.chunkSize = int(stat.Size())
		} else {
			logger.Info("[OUTPUT-FILE] error accessing file size", err)
		}
	}
}

// String output address
func (o *FileOutput) String() string {
	return "File output: " + o.file.Name()
}

func (o *FileOutput) closeLocked() error {
	if o.file != nil {
		if strings.HasSuffix(o.currentName, ".gz") {
			_ = o.writer.(*gzip.Writer).Close()
		} else {
			_ = o.writer.(*bufio.Writer).Flush()
		}

		o.file.Close()
		if o.config.OnClose != nil {
			o.config.OnClose(o.file.Name())
		}
	}

	o.closed = true

	return nil
}

// Close closes the output file that is being written to.
func (o *FileOutput) Close() error {
	o.Lock()
	defer o.Unlock()

	return o.closeLocked()
}

// IsClosed returns if the output file is closed or not.
func (o *FileOutput) IsClosed() bool {
	o.Lock()
	defer o.Unlock()

	return o.closed
}
