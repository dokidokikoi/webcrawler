package scheduler

import "github.com/dokidokikoi/webcrawler/module"

// 容器的接口类型
type Args interface {
	// 自检参数的有效性
	// 为 nil 则未发现问题
	Check() error
}

// 请求相关参数
type RequestArgs struct {
	// 可以接受的 URL 的主域名列表
	// URL 主域名不在列表中的请求都会被忽略
	AcceptedDomains []string `json:"accepted_primary_domains"`
	// 需要爬取的最大深度
	// 实际深度大于此值的请求都会被忽略
	MaxDepth uint32 `json:"max_depth"`
}

func (args *RequestArgs) Check() error {
	if args.AcceptedDomains == nil {
		return genError("nil accepted primary domain list")
	}
	return nil
}

// 数据相关参数
type DataArgs struct {
	// 请求缓冲器的容量
	ReqBufferCap uint32 `json:"req_buffer_cap"`
	// 请求缓冲器的最大数量
	ReqMaxBufferNumber uint32 `json:"req_max_buffer_number"`
	// 响应缓冲器的容量
	RespBufferCap uint32 `json:"resp_buffer_cap"`
	// 响应缓冲器的最大数量
	RespMaxBufferNumber uint32 `json:"resp_max_buffer_number"`
	// 条目缓冲器的容量
	ItemBufferCap uint32 `json:"item_buffer_cap"`
	// 条目缓冲器的最大数量
	ItemMaxBufferNumber uint32 `json:"item_max_buffer_number"`
	// 错误缓冲器的容量
	ErrorBufferCap uint32 `json:"error_buffer_cap"`
	// 错误缓冲器的最大数量
	ErrorMaxBufferNumber uint32 `json:"error_max_buffer_number"`
}

func (args *DataArgs) Check() error {
	if args.ReqBufferCap == 0 {
		return genError("zero request buffer capacity")
	}
	if args.ReqMaxBufferNumber == 0 {
		return genError("zero max request buffer number")
	}
	if args.RespBufferCap == 0 {
		return genError("zero response buffer capacity")
	}
	if args.RespMaxBufferNumber == 0 {
		return genError("zero max response buffer number")
	}
	if args.ItemBufferCap == 0 {
		return genError("zero item buffer capacity")
	}
	if args.ItemMaxBufferNumber == 0 {
		return genError("zero max item buffer number")
	}
	if args.ErrorBufferCap == 0 {
		return genError("zero error buffer capacity")
	}
	if args.ErrorMaxBufferNumber == 0 {
		return genError("zero max error buffer number")
	}
	return nil
}

// 组件相关的参数
type ModuleArgs struct {
	// 下载器列表
	Downloaders []module.Downloader
	// 分析器列表
	Analyzers []module.Analyzer
	// 条目处理管道列表
	Pipelines []module.Pipeline
}

func (args *ModuleArgs) Check() error {
	if len(args.Downloaders) == 0 {
		return genError("empty downloader list")
	}
	if len(args.Analyzers) == 0 {
		return genError("empty analyzer list")
	}
	if len(args.Pipelines) == 0 {
		return genError("empty pipeline list")
	}
	return nil
}
