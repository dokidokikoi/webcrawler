package scheduler

import "net/http"

// 调度器接口
type Schduler interface {
	// 初始化调度器
	// @Param requestArgs 代表请求相关参数
	// @Param dataArgs 代表数据相关参数
	// @Param moduleArgs 代表组件相关的参数
	Init(requestArgs RequestArgs, dataArgs DataArgs, moduleArgs ModuleArgs) (err error)
	// 启动调度器并执行爬虫程序
	// @Param firstHTTPReq 代表首次请求
	// 调度会以此为起点开始执行爬行流程
	Start(firstHTTPReq *http.Request) (err error)
	// 停止调度器的运行
	// 所有的处理模块执行的流程都会终止
	Stop() (err error)
	// 获取调度器状态
	Status() Status
	// 获取错误管道
	// 调度器以及各个处理模块运行过程中出现的所有的错误
	// 都会被发送到该通道
	// 若调度结果为 nil，则说明错误通道不可用或调度器已停止
	ErrorChan() <-chan error
	// 判断所有处理模块是否都处于空闲状态
	Idle() bool
	// 获取摘要实例
	Summary() SchedSummary
}
