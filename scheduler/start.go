package scheduler

import (
	"fmt"
	"net/http"

	"github.com/dokidokikoi/webcrawler/log"
	"github.com/dokidokikoi/webcrawler/module"
)

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
