package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	cmap "github.com/dokidokikoi/go-cmap"
	"github.com/dokidokikoi/webcrawler/log"
	"github.com/dokidokikoi/webcrawler/module"
	"github.com/dokidokikoi/webcrawler/toolkit/buffer"
)

// 调度器接口
type Scheduler interface {
	// 初始化调度器
	// @Param requestArgs 代表请求相关参数
	// @Param dataArgs 代表数据相关参数
	// @Param moduleArgs 代表组件相关的参数
	Init(reqArgs RequestArgs, dataArgs DataArgs, moduleArgs ModuleArgs) (err error)
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

type myScheduler struct {
	// 爬取到最大深度，首次请求的深度为0
	maxDepth uint32
	// 可以接受的 URL 的主域名的字典
	acceptedDomainMap cmap.ConcurrentMap
	// 组件组册器
	registrar module.Registrar
	// 请求缓冲池
	reqBufferPool buffer.Pool
	// 响应缓冲池
	respBufferPool buffer.Pool
	// 条目缓冲池
	itemBufferPool buffer.Pool
	// 错误缓冲池
	errorBufferPool buffer.Pool
	// 已处理的 URL 字典
	urlMap cmap.ConcurrentMap
	// 上下文，用于感知调度器的停止
	ctx context.Context
	// 取消函数，用于停止调度器
	cancelFunc context.CancelFunc
	// 状态
	status Status
	// 状态的读写锁
	statusLock sync.RWMutex
	// 摘要信息
	summary SchedSummary
}

func (sched *myScheduler) Init(
	reqArgs RequestArgs,
	dataArgs DataArgs,
	moduleArgs ModuleArgs) (err error) {
	// 检查状态
	log.L().Sugar().Infof("Check status for initialization...")
	var oldStatus Status
	oldStatus, err = sched.checkAndSetStatus(SCHED_STATUS_INITIALIZING)
	if err != nil {
		return
	}

	defer func() {
		sched.statusLock.Lock()
		if err != nil {
			sched.status = oldStatus
		} else {
			sched.status = SCHED_STATUS_INITIALIZED
		}
		sched.statusLock.Unlock()
	}()

	// 检查参数
	log.L().Sugar().Infof("Check request argments...")
	if err = reqArgs.Check(); err != nil {
		return err
	}
	log.L().Sugar().Infof("Check data argments...")
	if err = dataArgs.Check(); err != nil {
		return err
	}
	log.L().Sugar().Infof("Data arguments are valid.")
	log.L().Sugar().Infof("Check module argments...")
	if err = moduleArgs.Check(); err != nil {
		return err
	}
	log.L().Sugar().Infof("Module argments are vaild.")

	// 初始化内部字段
	log.L().Sugar().Infof("Initialize scheduler's fields...")
	if sched.registrar == nil {
		sched.registrar = module.NewRegistrar()
	} else {
		sched.registrar.Clear()
	}
	sched.maxDepth = reqArgs.MaxDepth
	log.L().Sugar().Infof("-- Max depth: %d", sched.maxDepth)
	sched.acceptedDomainMap, _ = cmap.NewConcurrentMap(1, nil)
	for _, domain := range reqArgs.AcceptedDomains {
		sched.acceptedDomainMap.Put(domain, struct{}{})
	}
	log.L().Sugar().Infof("-- Accepted primary domain: %v", reqArgs.AcceptedDomains)
	sched.urlMap, _ = cmap.NewConcurrentMap(16, nil)
	log.L().Sugar().Infof("-- URL map: length: %d, concurrency: %d",
		sched.urlMap.Len(), sched.urlMap.Concurrency())
	sched.initBufferPool(dataArgs)
	sched.resetContext()
	sched.summary = newSchedSummary(reqArgs, dataArgs, moduleArgs, sched)

	// 注册组件
	log.L().Sugar().Info("Register modules...")
	if err = sched.registerModules(moduleArgs); err != nil {
		return err
	}
	log.L().Sugar().Info("Scheduler has been initialized.")
	return nil
}

func (sched *myScheduler) checkAndSetStatus(
	wantedStatus Status) (oldStatus Status, err error) {
	sched.statusLock.Lock()
	defer sched.statusLock.Unlock()

	oldStatus = sched.status
	err = checkStatus(oldStatus, wantedStatus, nil)
	if err == nil {
		sched.status = wantedStatus
	}
	return
}

// 初始化缓冲池
// 如果某个缓冲池可用且为关闭，就先关闭该缓冲池
func (sched *myScheduler) initBufferPool(dataArgs DataArgs) {
	// 初始化请求缓冲池
	if sched.reqBufferPool != nil && !sched.reqBufferPool.Closed() {
		sched.reqBufferPool.Close()
	}
	sched.reqBufferPool, _ = buffer.NewPool(
		dataArgs.ReqBufferCap, dataArgs.ReqMaxBufferNumber)
	log.L().Sugar().Info("-- Request buffer pool: bufferCap: %d, maxBufferNumber: %d",
		sched.reqBufferPool.BufferCap(), sched.reqBufferPool.MaxBufferNumber())

	// 初始化响应缓冲池
	if sched.respBufferPool != nil && !sched.respBufferPool.Closed() {
		sched.respBufferPool.Close()
	}
	sched.respBufferPool, _ = buffer.NewPool(
		dataArgs.RespBufferCap, dataArgs.RespMaxBufferNumber)
	log.L().Sugar().Info("-- Response buffer pool: bufferCap: %d, maxBufferNumber: %d",
		sched.respBufferPool.BufferCap(), sched.respBufferPool.MaxBufferNumber())

	// 初始化条目缓冲池
	if sched.itemBufferPool != nil && !sched.itemBufferPool.Closed() {
		sched.itemBufferPool.Close()
	}
	sched.itemBufferPool, _ = buffer.NewPool(
		dataArgs.ItemBufferCap, dataArgs.ItemMaxBufferNumber)
	log.L().Sugar().Info("-- Item buffer pool: bufferCap: %d, maxBufferNumber: %d",
		sched.itemBufferPool.BufferCap(), sched.itemBufferPool.MaxBufferNumber())

	// 初始化错误缓冲池
	if sched.errorBufferPool != nil && !sched.errorBufferPool.Closed() {
		sched.errorBufferPool.Close()
	}
	sched.errorBufferPool, _ = buffer.NewPool(
		dataArgs.ErrorBufferCap, dataArgs.ErrorMaxBufferNumber)
	log.L().Sugar().Info("-- Error buffer pool: bufferCap: %d, maxBufferNumber: %d",
		sched.errorBufferPool.BufferCap(), sched.errorBufferPool.MaxBufferNumber())
}

func (sched *myScheduler) resetContext() {
	sched.ctx, sched.cancelFunc = context.WithCancel(context.Background())
}

func (sched *myScheduler) registerModules(moduleArgs ModuleArgs) error {
	for _, d := range moduleArgs.Downloaders {
		if d == nil {
			continue
		}
		ok, err := sched.registrar.Register(d)
		if err != nil {
			return genErrorByError(err)
		}
		if !ok {
			errMsg := fmt.Sprintf("Couldn't register downloader instance with MID %q", d.ID())
			return genError(errMsg)
		}
	}
	log.L().Sugar().Infof("All download have been registered. (number: %d)",
		len(moduleArgs.Downloaders))

	for _, a := range moduleArgs.Analyzers {
		if a == nil {
			continue
		}
		ok, err := sched.registrar.Register(a)
		if err != nil {
			return genErrorByError(err)
		}
		if !ok {
			errMsg := fmt.Sprintf("Couldn't register analyzer instance with MID %q!", a.ID())
			return genError(errMsg)
		}
	}
	log.L().Sugar().Infof("All analyzers have been registered. (number: %d)",
		len(moduleArgs.Analyzers))

	for _, p := range moduleArgs.Pipelines {
		if p == nil {
			continue
		}
		ok, err := sched.registrar.Register(p)
		if err != nil {
			return genErrorByError(err)
		}
		if !ok {
			errMsg := fmt.Sprintf("Couldn't register pipeline instance wirh MID %q!", p.ID())
			return genError(errMsg)
		}
	}
	log.L().Sugar().Infof("All pipeline have been registered. (number: %d)",
		len(moduleArgs.Pipelines))
	return nil
}

func (sched *myScheduler) Start(firstHTTPReq *http.Request) (err error) {
	defer func() {
		if p := recover(); p != nil {
			errMsg := fmt.Sprintf("Fatal scheduler error: %s", p)
			log.L().Sugar().Info(errMsg)
			err = genError(errMsg)
		}
	}()
	log.L().Sugar().Info("Start scheduler...")

	// 检查状态
	log.L().Sugar().Info("Check status for start...")
	var oldStatus Status
	oldStatus, err = sched.checkAndSetStatus(SCHED_STATUS_STARTING)
	defer func() {
		sched.statusLock.Lock()
		if err != nil {
			sched.status = oldStatus
		} else {
			sched.status = SCHED_STATUS_STARTED
		}
		sched.statusLock.Unlock()
	}()
	if err != nil {
		return
	}

	// 检查参数
	log.L().Sugar().Info("Check first HTTP request...")
	if firstHTTPReq == nil {
		err = genParameterError("nil first HTTP request")
		return
	}
	log.L().Sugar().Info("The first HTTP request is valid.")
	// 获得首次请求的主域名，并将其添加到可接受的主域名的字典
	log.L().Sugar().Info("Get the primary domain...")
	log.L().Sugar().Info("-- Host: $s", firstHTTPReq.Host)
	var primaryDomain string
	primaryDomain, err = getPrimaryDomain(firstHTTPReq.Host)
	if err != nil {
		return
	}
	log.L().Sugar().Info("-- Primary domain: %s", primaryDomain)
	sched.acceptedDomainMap.Put(primaryDomain, struct{}{})

	// 开始调度数据和组件
	if err = sched.checkBufferPoolForStart(); err != nil {
		return
	}
	sched.download()
	sched.analyze()
	sched.pick()
	log.L().Sugar().Info("Scheduler has been started.")

	// 放入第一个请求
	firstReq := module.NewRequest(firstHTTPReq, 0)
	sched.sendReq(firstReq)
	return nil
}

// 检查缓冲池是否已为调度器的启动准备就绪
// 如果某个缓冲池不可用，就直接返回错误值报告此情况
// 如果某个缓冲池已关闭，就按原先的参数重新初始化它
func (sched *myScheduler) checkBufferPoolForStart() error {
	// 检查请求缓冲池
	if sched.reqBufferPool == nil {
		return genError("nil request buffer pool")
	}
	if sched.reqBufferPool != nil && sched.reqBufferPool.Closed() {
		sched.reqBufferPool, _ = buffer.NewPool(
			sched.reqBufferPool.BufferCap(), sched.reqBufferPool.MaxBufferNumber())
	}

	// 检查响应缓冲池
	if sched.respBufferPool == nil {
		return genError("nil response buffer pool")
	}
	if sched.respBufferPool != nil && sched.respBufferPool.Closed() {
		sched.respBufferPool, _ = buffer.NewPool(
			sched.respBufferPool.BufferCap(), sched.respBufferPool.MaxBufferNumber())
	}

	// 检查条目缓冲池
	if sched.itemBufferPool == nil {
		return genError("nil item buffer pool")
	}
	if sched.itemBufferPool != nil && sched.itemBufferPool.Closed() {
		sched.itemBufferPool, _ = buffer.NewPool(
			sched.itemBufferPool.BufferCap(), sched.itemBufferPool.MaxBufferNumber())
	}

	// 检查错误缓冲池
	if sched.errorBufferPool == nil {
		return genError("nil error buffer pool")
	}
	if sched.errorBufferPool != nil && sched.errorBufferPool.Closed() {
		sched.errorBufferPool, _ = buffer.NewPool(
			sched.errorBufferPool.BufferCap(), sched.errorBufferPool.MaxBufferNumber())
	}
	return nil
}

// 从缓冲池取出请求并下载
// 然后把得到的响应放入响应缓冲池
func (sched *myScheduler) download() {
	go func() {
		for {
			if sched.canceled() {
				break
			}
			datum, err := sched.reqBufferPool.Get()
			if err != nil {
				log.L().Sugar().Warnln("The request buffer pool was closed. Break request reception.")
				break
			}
			req, ok := datum.(*module.Request)
			if !ok {
				errMsg := fmt.Sprintf("incorrect request type: %T", datum)
				sendError(errors.New(errMsg), "", sched.errorBufferPool)
			}
			sched.downloadOne(req)
		}
	}()
}

// 根据给定的请求执行下载并把响应放入响应缓冲池
func (sched *myScheduler) downloadOne(req *module.Request) {
	if req == nil {
		return
	}
	if sched.canceled() {
		return
	}
	m, err := sched.registrar.Get(module.TYPE_DOWNLOADER)
	if err != nil || m == nil {
		errMsg := fmt.Sprintf("couldn't get a downloader: %s", err)
		sendError(errors.New(errMsg), "", sched.errorBufferPool)
		sched.sendReq(req)
		return
	}
	downloader, ok := m.(module.Downloader)
	if !ok {
		errMsg := fmt.Sprintf("incorrect downloader type: %T (MID: %s)",
			m, m.ID())
		sendError(errors.New(errMsg), m.ID(), sched.errorBufferPool)
		sched.sendReq(req)
		return
	}
	resp, err := downloader.Download(req)
	if resp != nil {
		sendResp(resp, sched.respBufferPool)
	}
	if err != nil {
		sendError(err, m.ID(), sched.errorBufferPool)
	}
}

func (sched *myScheduler) sendReq(req *module.Request) bool {
	if req == nil {
		return false
	}
	if sched.canceled() {
		return false
	}
	httpReq := req.HTTPReq()
	if httpReq == nil {
		log.L().Sugar().Warnln("Ignore the request! Its HTTP request is invalid!")
		return false
	}
	reqURL := httpReq.URL
	if reqURL == nil {
		log.L().Sugar().Warnln("Ignore the request! Its URL is invalid!")
		return false
	}
	scheme := strings.ToLower(reqURL.Scheme)
	if scheme != "http" && scheme != "https" {
		log.L().Sugar().Warnln("Ignore the request! Its URL scheme is %q, but should be %q or %q. (URL: %s)\n",
			scheme, "http", "https", reqURL)
		return false
	}
	if v := sched.urlMap.Get(reqURL.String()); v != nil {
		log.L().Sugar().Warnf("Ignore the request! Its URL is repeated. (URL: %s)\n",
			reqURL)
		return false
	}
	pd, _ := getPrimaryDomain(httpReq.Host)
	if sched.acceptedDomainMap.Get(pd) == nil {
		if pd == "bing.net" {
			panic(httpReq.URL)
		}
		log.L().Sugar().Warnf("Ignore the request! Its host %q is not in accepted primary domain map. (URL: %s)",
			httpReq.Host, reqURL)
		return false
	}
	if req.Depth() > sched.maxDepth {
		log.L().Sugar().Warnf("Ignore the request! Its depth %d is greater than %d. (URL: %s)",
			req.Depth(), sched.maxDepth, reqURL)
		return false
	}

	go func(req *module.Request) {
		if err := sched.reqBufferPool.Put(req); err != nil {
			log.L().Sugar().Warnln("The request buffer pool was closed. Ingnore request sending.")
		}
	}(req)
	sched.urlMap.Put(reqURL.String(), struct{}{})
	return true
}

func sendResp(resp *module.Response, respBufferPool buffer.Pool) bool {
	if resp == nil || respBufferPool == nil || respBufferPool.Closed() {
		return false
	}

	go func(resp *module.Response) {
		if err := respBufferPool.Put(resp); err != nil {
			log.L().Sugar().Warnln("The response buffer pool was closed. Ignore response sending.")
		}
	}(resp)
	return true
}

// canceled 用于判断调度器的上下文是否已被取消。
func (sched *myScheduler) canceled() bool {
	select {
	case <-sched.ctx.Done():
		return true
	default:
		return false
	}
}

// 从响应缓冲池取出响应并解析
// 然后把得到的条目或请求放入响应的缓冲池
func (sched *myScheduler) analyze() {
	go func() {
		for {
			if sched.canceled() {
				break
			}
			datum, err := sched.respBufferPool.Get()
			if err != nil {
				log.L().Sugar().Warnln("The response buffer pool was closed. Break response reception.")
				break
			}
			resp, ok := datum.(*module.Response)
			if !ok {
				errMsg := fmt.Sprintf("incorrect response type: %T", datum)
				sendError(errors.New(errMsg), "", sched.errorBufferPool)
			}
			sched.analyzeOne(resp)
		}
	}()
}

func (sched *myScheduler) analyzeOne(resp *module.Response) {
	if resp == nil {
		return
	}
	if sched.canceled() {
		return
	}
	m, err := sched.registrar.Get(module.TYPE_ANALYZER)
	if err != nil || m == nil {
		errMsg := fmt.Sprintf("couldn't get an analyzer: %s", err)
		sendError(errors.New(errMsg), "", sched.errorBufferPool)
		sendResp(resp, sched.respBufferPool)
		return
	}
	analyzer, ok := m.(module.Analyzer)
	if !ok {
		errMsg := fmt.Sprintf("incorrect analyzer type %T (MID: %s)",
			m, m.ID())
		sendError(errors.New(errMsg), m.ID(), sched.errorBufferPool)
		sendResp(resp, sched.respBufferPool)
		return
	}
	dataList, errs := analyzer.Analyze(resp)
	if dataList != nil {
		for _, data := range dataList {
			if data == nil {
				continue
			}
			switch d := data.(type) {
			case *module.Request:
				sched.sendReq(d)
			case module.Item:
				sendItem(d, sched.itemBufferPool)
			default:
				errMsg := fmt.Sprintf("Unsupported data type %T! (data: %#v)", d, d)
				sendError(errors.New(errMsg), m.ID(), sched.errorBufferPool)
			}
		}
	}
	if errs != nil {
		for _, err := range errs {
			sendError(err, m.ID(), sched.errorBufferPool)
		}
	}
}

func sendItem(item module.Item, itemBufferPool buffer.Pool) bool {
	if item == nil || itemBufferPool == nil || itemBufferPool.Closed() {
		return false
	}

	go func(item module.Item) {
		if err := itemBufferPool.Put(item); err != nil {
			log.L().Sugar().Warnln("The item buffer pool was closed. Ignore item sending.")
		}
	}(item)
	return true
}

// 从条目缓冲池取出条目并处理
func (sched *myScheduler) pick() {
	go func() {
		for {
			if sched.canceled() {
				break
			}
			datum, err := sched.itemBufferPool.Get()
			if err != nil {
				log.L().Sugar().Warnln("The item buffer pool was closed. Break item reception.")
				break
			}
			item, ok := datum.(module.Item)
			if !ok {
				errMsg := fmt.Sprintf("incorrect item type: %T", datum)
				sendError(errors.New(errMsg), "", sched.errorBufferPool)
			}
			sched.pickOne(item)
		}
	}()
}

func (sched *myScheduler) pickOne(item module.Item) {
	if sched.canceled() {
		return
	}
	m, err := sched.registrar.Get(module.TYPE_PIPELINE)
	if err != nil || m == nil {
		errMsg := fmt.Sprintf("couldn't get a pipeline: %s", err)
		sendError(errors.New(errMsg), "", sched.errorBufferPool)
		sendItem(item, sched.itemBufferPool)
		return
	}
	pipeline, ok := m.(module.Pipeline)
	if !ok {
		errMsg := fmt.Sprintf("incorrent pipeline type; %T (MID: %s",
			m, m.ID())
		sendError(errors.New(errMsg), m.ID(), sched.errorBufferPool)
		sendItem(item, sched.itemBufferPool)
		return
	}
	errs := pipeline.Send(item)
	if errs != nil {
		for _, err := range errs {
			sendError(err, m.ID(), sched.errorBufferPool)
		}
	}
}

func (sched *myScheduler) Stop() (err error) {
	log.L().Sugar().Info("Stop scheduler...")
	// 检查状态
	log.L().Sugar().Info("Check status for stop...")
	var oldStatus Status
	oldStatus, err = sched.checkAndSetStatus(SCHED_STATUS_STARTING)
	defer func() {
		sched.statusLock.Lock()
		if err != nil {
			sched.status = oldStatus
		} else {
			sched.status = SCHED_STATUS_STOPPED
		}
		sched.statusLock.Unlock()
	}()
	if err != nil {
		return
	}
	sched.cancelFunc()
	sched.reqBufferPool.Close()
	sched.respBufferPool.Close()
	sched.itemBufferPool.Close()
	sched.errorBufferPool.Close()
	log.L().Sugar().Info("Scheduler has been stopped.")
	return nil
}

func (sched *myScheduler) Status() Status {
	var status Status
	sched.statusLock.RLock()
	status = sched.status
	sched.statusLock.RUnlock()
	return status
}

func (sched *myScheduler) ErrorChan() <-chan error {
	errBuffer := sched.errorBufferPool
	errCh := make(chan error, errBuffer.BufferCap())
	go func(errBuffer buffer.Pool, errCh chan error) {
		for {
			if sched.canceled() {
				close(errCh)
				break
			}
			datum, err := errBuffer.Get()
			if err != nil {
				log.L().Sugar().Warnln("The error buffer pool was closed. Break error reception.")
				close(errCh)
				break
			}
			err, ok := datum.(error)
			if !ok {
				errMsg := fmt.Sprintf("incorrect error type: %T", datum)
				sendError(errors.New(errMsg), "", sched.errorBufferPool)
				continue
			}
			if sched.canceled() {
				close(errCh)
				break
			}
			errCh <- err
		}
	}(errBuffer, errCh)
	return errCh
}

func (sched *myScheduler) Idle() bool {
	moduleMap := sched.registrar.GetAll()
	for _, module := range moduleMap {
		if module.HandlingNumber() > 0 {
			return false
		}
	}
	if sched.reqBufferPool.Total() > 0 ||
		sched.respBufferPool.Total() > 0 ||
		sched.itemBufferPool.Total() > 0 {
		return false
	}
	return true
}

func (sched *myScheduler) Summary() SchedSummary {
	return sched.summary
}

func NewScheduler() Scheduler {
	return &myScheduler{}
}
