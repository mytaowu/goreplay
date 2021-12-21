package main

import (
	"time"

	"goreplay/logger"
)

// ExitProccess 退出goreplay
func ExitProccess(d time.Duration) {
	logger.Info("exit goreplay after ", d)
}
