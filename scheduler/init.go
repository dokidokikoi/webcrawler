package scheduler

import (
	cmap "github.com/dokidokikoi/go-cmap"
	"github.com/dokidokikoi/webcrawler/log"
	"github.com/dokidokikoi/webcrawler/module"
)

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
