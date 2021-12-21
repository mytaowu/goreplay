package logreplay

// ReplayType 相关常量
const (
	NormalReplayType = iota
	TwoEnvReplayType
)

// RecordMessage defines record message
type RecordMessage struct {
	ModuleID      string                 `json:"module_id,omitempty"`      // 模块ID
	CommitID      string                 `json:"commit_id,omitempty"`      // Git提交ID
	PTraceID      string                 `json:"p_trace_id,omitempty"`     // 父Trace ID
	TraceID       string                 `json:"trace_id,omitempty"`       // Trace ID
	ServiceName   string                 `json:"service_name,omitempty"`   // 服务名
	InstanceName  string                 `json:"instance_name,omitempty"`  // 实例信息
	Protocol      string                 `json:"protocol,omitempty"`       // 协议
	APIName       string                 `json:"api_name,omitempty"`       // 接口名
	Time          float64                `json:"time,omitempty"`           // 时间
	Request       map[string]interface{} `json:"request,omitempty"`        // 请求内容
	RequestBytes  []byte                 `json:"request_bytes,omitempty"`  // 请求二进制
	Response      map[string]interface{} `json:"response,omitempty"`       // 响应内容
	ResponseBytes []byte                 `json:"response_bytes,omitempty"` // 响应二进制
	Tag           map[string]interface{} `json:"tag,omitempty"`            // 业务自定义tag
	Src           string                 `json:"src,omitempty"`            // 数据的类型, 如goSdk cppSdk javaSdk openApi (业务名称)等
}

// ReplayStatus 回放结果状态位
type ReplayStatus string

// const for aspect replay status
const (
	ReplayStatusRecordInit  ReplayStatus = "init"         // 录制状态，回放状态位初始化
	ReplayStatusMatched     ReplayStatus = "matched"      // 回放命中状态
	ReplayStatusUnmatched   ReplayStatus = "unmatched"    // 回放未命中状态
	ReplayStatusRepeatedKey ReplayStatus = "repeated_key" // 回放匹配到重复key
)

// AspectMessage defines aspect message
type AspectMessage struct {
	ModuleID     string       `json:"module_id,omitempty"`     // 模块ID
	TraceID      string       `json:"trace_id,omitempty"`      // Trace ID, 默认为empty
	Time         float64      `json:"time,omitempty"`          // 时间
	APIName      string       `json:"api_name,omitempty"`      // 接口名
	Request      []byte       `json:"request,omitempty"`       // 请求
	Response     []byte       `json:"response,omitempty"`      // 响应
	Error        string       `json:"error,omitempty"`         // 客户端调用错误
	TaskID       uint32       `json:"task_id,omitempty"`       // 任务 ID ，首次来源录制时默认为0，回放时记录任务ID
	ReplayStatus ReplayStatus `json:"replay_status,omitempty"` // 回放结果状态位

	// FIXME Deprecated 废弃 key 字段
	Key string `json:"key,omitempty"` // 用于数据匹配的Key

	// FIXME Deprecated 改名为 response
	Data []byte `json:"data,omitempty"` // mock数据
}

// ReplayMessage defines replay message
type ReplayMessage struct {
	ModuleID   string                 `json:"module_id,omitempty"`   // 模块ID
	CommitID   string                 `json:"commit_id,omitempty"`   // Git提交ID
	TaskID     uint32                 `json:"task_id,omitempty"`     // 任务ID
	TraceID    string                 `json:"trace_id,omitempty"`    // Trace ID
	Time       float64                `json:"time,omitempty"`        // 回放时间
	Response   map[string]interface{} `json:"response,omitempty"`    // 响应内容
	Protocol   string                 `json:"protocol,omitempty"`    // 协议
	ReplayType int32                  `json:"replay_type,omitempty"` // 标识回放的类型
}

// CoverageMessage defines coverage message
type CoverageMessage struct {
	ModuleID string  `json:"module_id,omitempty"` // 模块ID
	CommitID string  `json:"commit_id,omitempty"` // Git提交ID
	TaskID   uint32  `json:"task_id,omitempty"`   // 任务ID
	TraceID  string  `json:"trace_id,omitempty"`  // Trace ID
	Time     float64 `json:"time,omitempty"`      // 回访时间
	Report   []byte  `json:"report,omitempty"`    // 报告结果
}

// GoReplaySendDetail 回放失败的原因统计
type GoReplaySendDetail struct {
	Success     uint32 `json:"success"`      // 发送成功
	SendFailed  uint32 `json:"send_failed"`  // 客户端发送或接收响应失败
	DialFailed  uint32 `json:"dial_failed"`  // 发送连接失败
	WriteFailed uint32 `json:"write_failed"` // 发送写失败
	ReadFailed  uint32 `json:"read_failed"`  // 发送读失败
}

// GoReplayMessage 包含了RecordMessage 和 ReplayMessage
type GoReplayMessage struct {
	RecordMessage
	TaskID              uint32 `json:"task_id"`               // 任务ID
	ReplayBytes         []byte `json:"replay_bytes"`          // 回放响应内容
	ProtocolServiceName string `json:"protocol_service_name"` // 协议的serviceName
	MethodName          string `json:"method_name"`           // 协议的方法名
	InterfaceName       string `json:"interface_name"`        // 协议的interfaceName
	SerializeType       string `json:"serialize_type"`        // 业务序列化类型：jce/pb
	GoReplaySendDetail
}

// PerfMetricsMessage 监控指标信息
type PerfMetricsMessage struct {
	AppName      string  `json:"app_name,omitempty"`      // 应用名
	ModuleID     string  `json:"module_id,omitempty"`     // 模块ID
	CommitID     string  `json:"commit_id,omitempty"`     // Git提交ID
	InstanceName string  `json:"instance_name,omitempty"` // 实例信息
	TraceID      string  `json:"trace_id,omitempty"`      // 唯一标识ID
	Timestamp    float64 `json:"timestamp,omitempty"`     // 时间，milliseconds
	CPURate      float64 `json:"cpu_rate,omitempty"`      // cpu 占用百分比
	MemRate      float64 `json:"mem_rate,omitempty"`      // mem 占用百分比
	DiskRead     float64 `json:"disk_read,omitempty"`     // disk read mb/s
	DiskWrite    float64 `json:"disk_write,omitempty"`    // disk write mb/s
	NetworkRead  float64 `json:"network_read,omitempty"`  // network read mb/s
	NetworkWrite float64 `json:"network_write,omitempty"` // network write mb/s
	QPS          float64 `json:"qps,omitempty"`           // qps
	Duration     float64 `json:"duration,omitempty"`      // duration 耗时
	FailRate     float64 `json:"fail_rate,omitempty"`     // 请求失败率
	TimeoutRate  float64 `json:"timeout_rate,omitempty"`  // 请求超时率
}

// BaseRsp base response
type BaseRsp struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}
