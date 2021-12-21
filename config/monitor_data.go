package config

// 定义 CPU 和内存的默认阈值，超过该阈值就会发送退出命令
const (
	CPUThreshold   = 85.0 // CPU 的默认阈值
	MemThreshold   = 85.0 // 内存的默认阈值
	TimeOfDuration = 10   // 持续时间为10s
)
