package pipeline

import (
	"fmt"

	"github.com/dokidokikoi/webcrawler/log"
	"github.com/dokidokikoi/webcrawler/module"
	"github.com/dokidokikoi/webcrawler/module/stub"
)

type myPipeline struct {
	stub.ModuleInternal
	// 条目处理器列表
	itemProcessors []module.ProcessItem
	// 处理是否需要快速失败
	failFast bool
}

func (p *myPipeline) ItemProcessors() []module.ProcessItem {
	processors := make([]module.ProcessItem, len(p.itemProcessors))
	copy(processors, p.itemProcessors)
	return processors
}

func (p *myPipeline) Send(item module.Item) []error {
	p.ModuleInternal.IncrHandlingNumber()
	defer p.ModuleInternal.DecrHandlingNumber()

	p.ModuleInternal.IncrCalledCount()
	var errs []error
	if item == nil {
		err := genParameterError("nil item")
		errs = append(errs, err)
		return errs
	}

	p.ModuleInternal.IncrAcceptedCount()
	log.L().Sugar().Infof("Process item %+v... \n", item)
	var currentItem = item
	for _, processor := range p.itemProcessors {
		processedItem, err := processor(currentItem)
		if err != nil {
			errs = append(errs, err)
			if p.failFast {
				break
			}
		}
		if processedItem != nil {
			currentItem = processedItem
		}
	}
	if len(errs) == 0 {
		p.ModuleInternal.IncrCompletedCount()
	}
	return errs
}

func (pipeline *myPipeline) FailFast() bool {
	return pipeline.failFast
}

func (pipeline *myPipeline) SetFailFast(failFast bool) {
	pipeline.failFast = failFast
}

// extraSummaryStruct 代表条目处理管道实额外信息的摘要类型。
type extraSummaryStruct struct {
	FailFast        bool `json:"fail_fast"`
	ProcessorNumber int  `json:"processor_number"`
}

func (pipeline *myPipeline) Summary() module.SummaryStruct {
	summary := pipeline.ModuleInternal.Summary()
	summary.Extra = extraSummaryStruct{
		FailFast:        pipeline.failFast,
		ProcessorNumber: len(pipeline.itemProcessors),
	}
	return summary
}

func New(
	mid module.MID,
	itemProcessors []module.ProcessItem,
	scoreCalculator module.CalculateScore) (module.Pipeline, error) {
	moduleBase, err := stub.NewModuleInternal(mid, scoreCalculator)
	if err != nil {
		return nil, err
	}
	if itemProcessors == nil {
		return nil, genParameterError("nil item processor list")
	}
	if len(itemProcessors) == 0 {
		return nil, genParameterError("empty item processor list")
	}
	var innerProcessors []module.ProcessItem
	for i, pipeline := range itemProcessors {
		if pipeline == nil {
			err := genParameterError(fmt.Sprintf("nil item processor[%d]", i))
			return nil, err
		}
		innerProcessors = append(innerProcessors, pipeline)
	}
	return &myPipeline{
		ModuleInternal: moduleBase,
		itemProcessors: innerProcessors,
	}, nil
}
