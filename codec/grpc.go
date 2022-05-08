package codec

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"goreplay/logger"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/http2"

	"goreplay/framer"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/jhump/protoreflect/dynamic"
)

// 常量
const (
	// GrpcName "grpc"常量，暴露出去供其他包使用
	GrpcName = "grpc"
	// grpc headerLength

	headerLength = 5
)

func init() {
	RegisterHeaderCodec(GrpcName, &grpcHeaderCodecBuilder{})
}

type grpcHeaderCodecBuilder struct {
}

// New 实例化解码器
func (builder *grpcHeaderCodecBuilder) New() HeaderCodec {
	return &grpcHeaderCodec{}
}

// grpcHeaderCodec grpc请求头解析
type grpcHeaderCodec struct {
}

// Decode 请求解码
func (g *grpcHeaderCodec) Decode(payload []byte, _ string) (ProtocolHeader, error) {
	ret := ProtocolHeader{}

	fr := framer.NewHTTP2Framer(payload, "", true)
	header := make(map[string]string)
	var rowBody []byte
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			if err != io.EOF {
				return ret, fmt.Errorf("grpcHeaderCodece err: %v %s", err, hex.EncodeToString(payload))
			}

			break
		}

		switch f := frame.(type) {
		case *http2.MetaHeadersFrame:
			// 处理对应的header信息
			for _, hf := range f.Fields {
				if hf.Name == ":path" {
					ret.ServiceName, ret.APIName = parseServiceName(hf.Value)
					ret.InterfaceName = parseInterfaceName(ret.ServiceName)
					ret.MethodName = ret.APIName
				}

				if hf.Name == framer.LogReplayTraceID {
					ret.CusTraceID = hf.Value
				}
				// 赋值给数组，方便后面打印
				header[hf.Name] = hf.Value
			}

			// 记录流ID
			header["stream_id"] = fmt.Sprint(f.StreamID)
		case *http2.DataFrame:
			rowBody = make([]byte, len(f.Data()))
			copy(rowBody, f.Data())
		}
	}

	logger.Info("header: %v", header)
	if rowBody != nil && len(rowBody) > headerLength {
		rowBody = rowBody[headerLength:]
	}

	logger.Info("row body: %s", string(rowBody))
	logger.Info("base64 row body: %s", base64.StdEncoding.EncodeToString(rowBody))

	Filename := "./protofile/helloworld.zip"

	ps, err := UnZip("./tempfile", Filename)
	if err != nil {
		logger.Error(fmt.Sprintf("ParseFiles err=%v", err))
		return ret, nil
	}

	logger.Info(fmt.Sprintf("-----ps: %s", ps))

	Parser := protoparse.Parser{}
	//加载并解析 proto文件,得到一组 FileDescriptor
	descs, err := Parser.ParseFiles(ps...)
	Parser.ParseFiles()
	if err != nil {
		logger.Error(fmt.Sprintf("ParseFiles err=%v", err))
		return ret, nil
	}

	//这里的代码是为了测试打印
	Printer := &protoprint.Printer{}
	var buf bytes.Buffer
	Printer.PrintProtoFile(descs[0], &buf)
	logger.Info(fmt.Sprintf("descsStr=%s\n", buf.String()))

	//descs 是一个数组，这里因为只有一个文件，就取了第一个元素.
	//通过proto的message名称得到MessageDescriptor 结构体定义描述符
	//这里其实通过对应的header信息可以确定message的信息
	service := descs[0].FindService("hello.Greeter")
	// msg := descs[0].FindMessage("hello.HelloRequest")
	//再用消息描述符，动态的构造一个pb消息体
	methods := service.GetMethods()
	for _, v := range methods {
		logger.Info(fmt.Sprintf("------name: %s", v.GetName()))
	}
	msgDesc := methods[0].GetInputType()

	dmsg := dynamic.NewMessage(msgDesc)

	//pb二进制消息 做反序列化 到 test.AddFriendReq 这个消息体
	err = dmsg.Unmarshal(rowBody)

	//把test.AddFriendReq 消息体序列化成 JSON 数据
	jsStr, _ := dmsg.MarshalJSON()
	logger.Info(fmt.Sprintf("-------grpc jsStr=%s\n", jsStr))

	return ret, nil
}

func parseServiceName(sm string) (service, method string) {
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		service = unknown
		method = unknown
		return
	}
	service = sm[:pos]
	method = sm[pos+1:]

	return
}

func parseInterfaceName(serviceName string) (interfaceName string) {
	pos := strings.LastIndex(serviceName, ".")
	if pos == -1 {
		interfaceName = unknown
		return
	}
	interfaceName = serviceName[pos+1:]

	return
}

func UnZip(dst, src string) (files []string, err error) {
	// 打开压缩文件，这个 zip 包有个方便的 ReadCloser 类型
	// 这个里面有个方便的 OpenReader 函数，可以比 tar 的时候省去一个打开文件的步骤
	zr, err := zip.OpenReader(src)
	paths := make([]string, 0)
	defer zr.Close()
	if err != nil {
		return
	}

	// 如果解压后不是放在当前目录就按照保存目录去创建目录
	if dst != "" {
		if err := os.MkdirAll(dst, 0755); err != nil {
			return files, err
		}
	}

	// 遍历 zr ，将文件写入到磁盘
	for _, file := range zr.File {
		path := filepath.Join(dst, file.Name)
		logger.Info(fmt.Sprintf("----获取到到path路径: %s", path))
		// 如果是目录，就创建目录
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return files, err
			}
			// 因为是目录，跳过当前循环，因为后面都是文件的处理
			continue
		}

		// 获取到 Reader
		fr, err := file.Open()
		if err != nil {
			return files, err
		}

		// 创建要写出的文件对应的 Write
		fw, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, file.Mode())
		if err != nil {
			return files, err
		}

		_, err = io.Copy(fw, fr)
		if err != nil {
			return files, err
		}

		ext := filepath.Ext(path)
		logger.Info(fmt.Sprintf("------文件路径：%s", path))
		logger.Info(fmt.Sprintf("------文件后缀名：%s", ext))

		if ext == ".proto" {
			paths = append(paths, path)
		}
		// 因为是在循环中，无法使用 defer ，直接放在最后
		// 不过这样也有问题，当出现 err 的时候就不会执行这个了，
		// 可以把它单独放在一个函数中，这里是个实验，就这样了
		fw.Close()
		fr.Close()
	}
	return paths, nil
}
