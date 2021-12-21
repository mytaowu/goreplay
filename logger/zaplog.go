package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const logFile = "goreplay.log"

// newLogger new zap logger instance
func newLogger(path string) (*zap.Logger, error) {
	outputPath := "stdout" // default output to stdout

	if path != "" {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("new log dir error: %v", err)
		}

		outputPath, err = ensureOutputPath(path)
		if err != nil {
			return nil, fmt.Errorf("ensure log path error: %v", err)
		}
	}

	cfg := zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.DebugLevel),
		Sampling: nil,
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:          "time",
			NameKey:          "logger",
			CallerKey:        "C",
			MessageKey:       "msg",
			ConsoleSeparator: " ",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeCaller:     zapcore.ShortCallerEncoder, // 全路径编码器
		},
		OutputPaths:      []string{outputPath},
		ErrorOutputPaths: []string{outputPath},
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("new logger error: %v", err)
	}

	logger = logger.WithOptions(zap.AddCallerSkip(2))

	// stdout and stderr redirect to zap logger
	zap.RedirectStdLog(logger)

	return logger, nil
}

func ensureOutputPath(path string) (string, error) {
	outputPath := path + string(os.PathSeparator) + logFile
	file, err := os.Open(outputPath)

	if err != nil && os.IsNotExist(err) {
		file, err = os.Create(outputPath)
		if err != nil {
			return "", err
		}
	}

	defer func() {
		err = file.Close()
		if err != nil {
			fmt.Printf("close file %s error: %v\n", file.Name(), err)
		}
	}()

	return outputPath, err
}
