package stub

import (
	"fmt"
	"sync/atomic"

	"github.com/dokidokikoi/webcrawler/errors"
	"github.com/dokidokikoi/webcrawler/module"
)

// 组件内部基础接口
type ModuleInternal interface {
	module.Module
	// 将调用次数加一
	IncrCalledCount()
	// 将接受计数加一
	IncrAcceptedCount()
	// 将成功计数加一
	IncrCompletedCount()
	// 将实时处理数加一
	IncrHandlingNumber()
	// 将实时处理数减一
	DecrHandlingNumber()
	// 清空所有计数
	Clear()
}

type myModule struct {
	mid module.MID
	// 组件内部网络地址
	addr string
	// 组件评分
	score uint64
	// 评分计数器
	scoreCalculator module.CalculateScore
	// 调用计数
	calledCount uint64
	// 接受计数
	acceptedCount uint64
	// 成功计数
	completedCount uint64
	// 实时处理数
	handlingNumber uint64
}

func (m *myModule) ID() module.MID {
	return m.mid
}

func (m *myModule) Addr() string {
	return m.addr
}

func (m *myModule) Score() uint64 {
	return atomic.LoadUint64(&m.score)
}

func (m *myModule) SetScore(score uint64) {
	atomic.StoreUint64(&m.score, score)
}

func (m *myModule) ScoreCalculator() module.CalculateScore {
	return m.scoreCalculator
}

func (m *myModule) CalledCount() uint64 {
	return atomic.LoadUint64(&m.calledCount)
}

func (m *myModule) AcceptedCount() uint64 {
	return atomic.LoadUint64(&m.acceptedCount)
}

func (m *myModule) CompletedCount() uint64 {
	return atomic.LoadUint64(&m.completedCount)
}

func (m *myModule) HandlingNumber() uint64 {
	return atomic.LoadUint64(&m.handlingNumber)
}

func (m *myModule) Counts() module.Counts {
	return module.Counts{
		CalledCount:    atomic.LoadUint64(&m.calledCount),
		AcceptedCount:  atomic.LoadUint64(&m.acceptedCount),
		CompletedCount: atomic.LoadUint64(&m.completedCount),
		HandlingNumber: atomic.LoadUint64(&m.handlingNumber),
	}
}

func (m *myModule) Summary() module.SummaryStruct {
	counts := m.Counts()
	return module.SummaryStruct{
		ID:        m.ID(),
		Called:    counts.CalledCount,
		Accepted:  counts.AcceptedCount,
		Completed: counts.CompletedCount,
		Handling:  counts.HandlingNumber,
		Extra:     nil,
	}
}

func (m *myModule) IncrCalledCount() {
	atomic.AddUint64(&m.calledCount, 1)
}

func (m *myModule) IncrAcceptedCount() {
	atomic.AddUint64(&m.acceptedCount, 1)
}

func (m *myModule) IncrCompletedCount() {
	atomic.AddUint64(&m.completedCount, 1)
}

func (m *myModule) IncrHandlingNumber() {
	atomic.AddUint64(&m.handlingNumber, 1)
}

func (m *myModule) DecrHandlingNumber() {
	atomic.AddUint64(&m.handlingNumber, ^uint64(0))
}

func (m *myModule) Clear() {
	atomic.StoreUint64(&m.calledCount, 0)
	atomic.StoreUint64(&m.acceptedCount, 0)
	atomic.StoreUint64(&m.completedCount, 0)
	atomic.StoreUint64(&m.handlingNumber, 0)
}

func NewModuleInternal(mid module.MID, scoreCalculator module.CalculateScore) (ModuleInternal, error) {
	parts, err := module.SplitMID(mid)
	if err != nil {
		return nil, errors.NewIllegalParameterError(fmt.Sprintf("illegal ID %q: %S", mid, err))
	}
	return &myModule{
		mid:             mid,
		addr:            parts[2],
		scoreCalculator: scoreCalculator,
	}, nil
}
