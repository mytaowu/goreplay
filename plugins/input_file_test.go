package plugins

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"goreplay/config"
	"goreplay/protocol"
)

// 将十进制数字转化为二进制字符串
func convertToBin(num int) string {
	s := ""

	if num == 0 {
		return "0"
	}

	// num /= 2 每次循环的时候 都将num除以2  再把结果赋值给 num
	for ; num > 0; num /= 2 {
		lsb := num % 2
		// strconv.Itoa() 将数字强制性转化为字符串
		s = strconv.Itoa(lsb) + s
	}
	return s
}
func TestBinary(t *testing.T) {
	a := 999999999

	fmt.Println(convertToBin(int(a >> 24)))
	fmt.Println(convertToBin(int(a >> 16)))
	fmt.Println(convertToBin(int(a >> 8)))
	fmt.Println(convertToBin(int(a)))

	b := make([]byte, 4)
	b[0] = byte(a >> 24)
	b[1] = byte(a >> 16)
	b[2] = byte(a >> 8)
	b[3] = byte(a)
	fmt.Println(b)
}

func TestInputFileMultipleFilesWithRequestsOnly(t *testing.T) {
	rnd := rand.Int63()

	file1, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_0", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	_, _ = file1.Write([]byte("1 1 1\ntest1"))
	_, _ = file1.Write([]byte(protocol.PayloadSeparator))
	_, _ = file1.Write([]byte("1 1 3\ntest2"))
	_, _ = file1.Write([]byte(protocol.PayloadSeparator))
	file1.Close()

	file2, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_1", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	_, _ = file2.Write([]byte("1 1 2\ntest3"))
	_, _ = file2.Write([]byte(protocol.PayloadSeparator))
	_, _ = file2.Write([]byte("1 1 4\ntest4"))
	_, _ = file2.Write([]byte(protocol.PayloadSeparator))
	file2.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d*", rnd), false)

	for i := '1'; i <= '4'; i++ {
		msg, _ := input.PluginRead()
		if msg.Meta[4] != byte(i) {
			t.Error("Should emit requests in right order", string(msg.Meta))
		}
	}

	_ = os.Remove(file1.Name())
	_ = os.Remove(file2.Name())
}

func TestInputFileRequestsWithLatency(t *testing.T) {
	rnd := rand.Int63()

	file, _ := os.OpenFile(fmt.Sprintf("/tmp/%d", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	defer file.Close()

	_, _ = file.Write([]byte("1 1 100000000\nrequest1"))
	_, _ = file.Write([]byte(protocol.PayloadSeparator))
	_, _ = file.Write([]byte("1 2 150000000\nrequest2"))
	_, _ = file.Write([]byte(protocol.PayloadSeparator))
	_, _ = file.Write([]byte("1 3 250000000\nrequest3"))
	_, _ = file.Write([]byte(protocol.PayloadSeparator))

	input := NewFileInput(fmt.Sprintf("/tmp/%d", rnd), false)

	start := time.Now().UnixNano()
	for i := 0; i < 3; i++ {
		_, _ = input.PluginRead()
	}
	end := time.Now().UnixNano()

	var expectedLatency int64 = 300000000 - 100000000
	realLatency := end - start
	if realLatency > expectedLatency {
		t.Errorf("Should emit requests respecting latency. Expected: %v, real: %v", expectedLatency, realLatency)
	}
}

func TestInputFileMultipleFilesWithRequestsAndResponses(t *testing.T) {
	rnd := rand.Int63()

	file1, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_0", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	_, _ = file1.Write([]byte("1 1 1\nrequest1"))
	_, _ = file1.Write([]byte(protocol.PayloadSeparator))
	_, _ = file1.Write([]byte("2 1 1\nresponse1"))
	_, _ = file1.Write([]byte(protocol.PayloadSeparator))
	_, _ = file1.Write([]byte("1 2 3\nrequest2"))
	_, _ = file1.Write([]byte(protocol.PayloadSeparator))
	_, _ = file1.Write([]byte("2 2 3\nresponse2"))
	_, _ = file1.Write([]byte(protocol.PayloadSeparator))
	_ = file1.Close()

	file2, _ := os.OpenFile(fmt.Sprintf("/tmp/%d_1", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	_, _ = file2.Write([]byte("1 3 2\nrequest3"))
	_, _ = file2.Write([]byte(protocol.PayloadSeparator))
	_, _ = file2.Write([]byte("2 3 2\nresponse3"))
	_, _ = file2.Write([]byte(protocol.PayloadSeparator))
	_, _ = file2.Write([]byte("1 4 4\nrequest4"))
	_, _ = file2.Write([]byte(protocol.PayloadSeparator))
	_, _ = file2.Write([]byte("2 4 4\nresponse4"))
	_, _ = file2.Write([]byte(protocol.PayloadSeparator))
	_ = file2.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d*", rnd), false)

	for i := '1'; i <= '4'; i++ {
		msg, _ := input.PluginRead()
		if msg.Meta[0] != '1' && msg.Meta[4] != byte(i) {
			t.Error("Shound emit requests in right order", string(msg.Meta))
		}

		msg, _ = input.PluginRead()
		if msg.Meta[0] != '2' && msg.Meta[4] != byte(i) {
			t.Error("Shound emit responses in right order", string(msg.Meta))
		}
	}

	_ = os.Remove(file1.Name())
	_ = os.Remove(file2.Name())
}

func TestInputFileLoop(t *testing.T) {
	rnd := rand.Int63()

	file, _ := os.OpenFile(fmt.Sprintf("/tmp/%d", rnd), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	_, _ = file.Write([]byte("1 1 1\ntest1"))
	_, _ = file.Write([]byte(protocol.PayloadSeparator))
	_, _ = file.Write([]byte("1 1 2\ntest2"))
	_, _ = file.Write([]byte(protocol.PayloadSeparator))
	_ = file.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d", rnd), true)

	// Even if we have just 2 requests in file, it should indifinitly loop
	for i := 0; i < 1000; i++ {
		_, _ = input.PluginRead()
	}

	_ = input.Close()
	_ = os.Remove(file.Name())
}

func TestInputFileCompressed(t *testing.T) {
	rnd := rand.Int63()

	output := NewFileOutput(fmt.Sprintf("/tmp/%d_0.gz", rnd),
		&config.FileOutputConfig{FlushInterval: time.Minute, Append: true})
	for i := 0; i < 1000; i++ {
		_, _ = output.PluginWrite(&Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	}
	name1 := output.file.Name()
	_ = output.Close()

	output2 := NewFileOutput(fmt.Sprintf("/tmp/%d_1.gz", rnd),
		&config.FileOutputConfig{FlushInterval: time.Minute, Append: true})
	for i := 0; i < 1000; i++ {
		_, _ = output2.PluginWrite(&Message{Meta: []byte("1 1 1\r\n"), Data: []byte("test")})
	}
	name2 := output2.file.Name()
	_ = output2.Close()

	input := NewFileInput(fmt.Sprintf("/tmp/%d*", rnd), false)
	for i := 0; i < 2000; i++ {
		_, _ = input.PluginRead()
	}

	_ = os.Remove(name1)
	_ = os.Remove(name2)
}
