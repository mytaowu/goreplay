package http

import "time"

// Options 客户端调用参数
type Options struct {
	Timeout time.Duration // 后端调用超时时间
}

// Option 调用参数工具函数
type Option func(*Options)
