package module

import "net/http"

// Counts 代表用于汇集组件内部计数的类型。
type Counts struct {
	// CalledCount 代表调用计数。
	CalledCount uint64
	// AcceptedCount 代表接受计数。
	AcceptedCount uint64
	// CompletedCount 代表成功完成计数。
	CompletedCount uint64
	// HandlingNumber 代表实时处理数。
	HandlingNumber uint64
}

// SummaryStruct 代表组件摘要结构的类型。
type SummaryStruct struct {
	ID        MID         `json:"id"`
	Called    uint64      `json:"called"`
	Accepted  uint64      `json:"accepted"`
	Completed uint64      `json:"completed"`
	Handling  uint64      `json:"handling"`
	Extra     interface{} `json:"extra,omitempty"`
}

// 组件的基础接口类型
// 该接口的实现类型必须是并发安全的
type Module interface {
	// 用于获取当前组件的 ID
	ID() MID
	// 用于获取当前组件的网络地址的字符串形式
	Addr() string
	// 用于获取当前组件的评分
	Score() uint64
	// 用于设置当前组件的评分
	SetScore(score uint64)
	// 用于获取评分计数器
	ScoreCalculator() CalculateScore
	// 用于获取当前组件被调用的计数
	CalledCount() uint64
	// 用于获取当前组件接受的调用的计数
	// 组件一般会由于超负荷或参数有误而拒绝调用
	AcceptedCount() uint64
	// 用于获取当前组件已成功完成的调用的计数
	CompletedCount() uint64
	// 用于获取当前组件正在处理的调用的计数
	HandlingNumber() uint64
	// 用于一次性获取所有的计数
	Counts() Counts
	// 用于获取组件摘要
	Summary() SummaryStruct
}

// 组件id模板
var midTemplate = "%s%d|%s"

// 组件类型
type Type string

// 当前认可的组件类型的常量
const (
	// 下载器
	TYPE_DOWNLOADER Type = "dowload"
	// 分析器
	TYPE_ANALYZER Type = "analyzer"
	// 条目处理器
	TYPE_PIPELINE Type = "pipeline"
)

// 合法的组件类型-字母的映射
var legalTypeLetterMap = map[Type]string{
	TYPE_DOWNLOADER: "D",
	TYPE_ANALYZER:   "A",
	TYPE_PIPELINE:   "P",
}

// 下载器接口
// 该接口的实现类型必须是并发安全的
type Downloader interface {
	Module
	Download(req *Request) (*Response, error)
}

// 用于解析 HTTP 响应的函数类型
type ParseResponse func(httpResp *http.Response, respDepth uint32) ([]Data, []error)

// 分析器接口
// 该接口的实现类型必须是并发安全的
type Analyzer interface {
	Module
	// 返回当前分析器使用的响应解析函数的列表
	RespParsers() []ParseResponse
	// 根据规则分析响应并返回请求和条目
	// 响应需要分别经过若干响应解析函数的处理，然后合并结果
	Analyzer(resp *Response) ([]Data, []error)
}

// 用于处理条目的函数类型
type ProcessItem func(item Item) (result Item, err error)

// 条目处理管道接口
// 该接口的实现类型必须是并发安全的
type Pipeline interface {
	Module
	// 用于返回当前条目处理管道使用的条目处理函数的列表
	ItemProcessors() []ProcessItem
	// 向条目处理管道发送条目
	// 条目需要依次经过若干条目处理函数的处理
	Send(item Item) []error
	// 返回当前条目处理管道是否是快速失败的
	// 快速失败指：只要在处理某个条目时在某一个步骤上出错
	// 那么条目处理管道就会忽略掉后续的所有处理步骤并报告错误
	FailFast() bool
	// 设置是否快速失败
	SetFailFast(failFast bool)
}
