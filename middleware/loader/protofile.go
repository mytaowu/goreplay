package loader

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"goreplay/logger"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
)

const (
	zipExt    = ".zip"       // 压缩包文件后缀
	protoExt  = ".proto"     // 协议文件后缀
	unzipPath = "./protobuf" // 解压协议文件的路径
)

type Pbdesc struct {
	// 缓存pb文件中serviceName和methdod描述符的对应关系。(key对应grpc header中的“：path”字段)
	methodCache map[string]*desc.MethodDescriptor
}

// NewPbDesc 工厂方法
func NewPbDesc(filePath string) *Pbdesc {
	res := &Pbdesc{
		methodCache: make(map[string]*desc.MethodDescriptor),
	}
	paths := parsePath(filePath)
	logger.Info(fmt.Sprintf("pb desc loeader path: %v", paths))

	Parser := protoparse.Parser{}
	//加载并解析 proto文件,得到一组 FileDescriptor
	descs, err := Parser.ParseFiles(paths...)
	// Parser.ParseFiles()
	if err != nil {
		logger.Error(fmt.Sprintf("ParseFiles err=%v", err))
		return res
	}

	// 加载对应的协议文件
	for _, fd := range descs {
		sList := fd.GetServices()
		for i := 0; i < len(sList); i++ {
			mList := sList[i].GetMethods()
			for j := 0; j < len(mList); j++ {
				fullName := mList[j].GetFullyQualifiedName()
				res.methodCache[fullName] = mList[j]
				logger.Info(fmt.Sprintf("-----fuillname: %s, method: %v", fullName, mList[j]))
			}
		}
	}

	return res
}

// getMethodDesc 通过serviceName获取methodDescriptor
func (p *Pbdesc) GetMethodDesc(serviceName string) *desc.MethodDescriptor {
	serviceName = strings.TrimLeft(serviceName, "/")
	serviceName = strings.ReplaceAll(serviceName, "/", ".")
	logger.Info(fmt.Sprintf("----替换后的serviceName: %s", serviceName))
	return p.methodCache[serviceName]
}

// parsePath 获取 proto文件路径
func parsePath(filePath string) []string {
	ext := path.Ext(filePath)
	paths := make([]string, 0) // 记录所有的 protocol文件
	switch ext {
	case zipExt:
		zipPaths, err := unzipAndProto(filePath)
		if err != nil {
			logger.Error(fmt.Sprintf("ParseFiles err=%v", err))
			return paths
		}
		paths = append(paths, zipPaths...)
	case protoExt:
		paths = append(paths, filePath)
	}

	return paths
}

// unzipAndParse 解压压缩包到指定路径，并返回压缩包内proto文件
func unzipAndProto(src string) ([]string, error) {
	zr, err := zip.OpenReader(src)
	paths := make([]string, 0)
	defer zr.Close()
	if err != nil {
		return paths, err
	}
	// 创建文件夹（如果存在对应的文件夹就清空）
	if err := os.MkdirAll(unzipPath, 0755); err != nil {
		return paths, err
	}

	// 遍历 zr ，将文件写入到磁盘
	for _, file := range zr.File {
		path := filepath.Join(unzipPath, file.Name)
		// 如果是目录，就创建目录
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return paths, err
			}
			// 因为是目录，跳过当前循环，因为后面都是文件的处理
			continue
		}

		// 获取到 Reader
		fr, err := file.Open()
		if err != nil {
			return paths, err
		}

		// 创建要写出的文件对应的 Write
		fw, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, file.Mode())
		if err != nil {
			return paths, err
		}

		_, err = io.Copy(fw, fr)
		if err != nil {
			return paths, err
		}

		ext := filepath.Ext(path)
		if ext == protoExt {
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
