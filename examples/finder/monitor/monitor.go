// Package monitor
// 功能：
//   - 在适当的时候停止自身和调度器，
//     最长持续空闲时间 = 检查间隔时间 * 检查结果连续为 true 的最大次数
//     超过“最长持续空闲时间”关闭调度器
//   - 实时监控调度器及其中的各个模块的运行状况
//   - 一旦调度器及其模块在运行过程中发生错误，及时予以报告
package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/dokidokikoi/webcrawler/log"
	sched "github.com/dokidokikoi/webcrawler/scheduler"
)

// 监控结果摘要的结构
type summary struct {
	// goroutine 数量
	NumGoroutine int `json:"goroutine_number"`
	// 调度器摘要信息
	SchedSummary sched.SummaryStruct `json:"sched_summary"`
	// 从开始监控至今流逝的时间
	EscapedTime string `json:"escaped_time"`
}

// 已达到最大空闲计数的消息模板
var msgReachMaxIdleCount = "The scheduler has been idle for a period of time" +
	" (about %s). Consider to stop it now."

// 停止调度器的消息模版
var msgStopScheduler = "Stop scheduler...%s."

type Record func(level uint8, content string)

// 监控调度器
// @Param scheduler 代表作为监控目标的调度器
// @Param checkInterval 代表检查间隔时间，单位：纳秒
// @Param summarizeInterval 代表摘要获取时间，单位：纳秒
// @Param maxIdleCount 代表最大空闲计数
// @Param autoStop 用来指示该方法是否在调度器空闲足够长的时间之后自行停止调度器
// @Param record 代表日志记录函数
// 当监控结束之后，该方法会向作为唯一结果值当通道发送一个代表了空闲状态检查次数的数值
func Monitor(
	scheduler sched.Scheduler,
	checkInterval time.Duration,
	summarizeInterval time.Duration,
	maxIdleCount uint,
	autoStop bool,
	record Record) <-chan uint64 {
	// 防止调度器不可用
	if scheduler == nil {
		panic(errors.New("The scheduler is invalid!"))
	}
	// 防止过小的检查间隔时间对爬取流程造成不良影响
	if checkInterval < time.Microsecond*100 {
		checkInterval = time.Microsecond * 100
	}
	// 防止过小的摘要获取时间间隔对爬取流程造成不良影响
	if summarizeInterval < time.Second {
		summarizeInterval = time.Second
	}
	// 防止过小的最大空闲计数造成调度器的过早停止
	if maxIdleCount < 10 {
		maxIdleCount = 10
	}
	log.L().Sugar().Infof("Monitor paramters; checkInterval: %s, summarizeInterval: %s,"+
		" maxInterval: %d, autoStop: %v",
		checkInterval, summarizeInterval, maxIdleCount, autoStop)
	// 生成监控停止通知器
	stopNotifier, stopFunc := context.WithCancel(context.Background())
	// 接收和报告错误
	reportError(scheduler, record, stopNotifier)
	// 记录摘要信息
	recordSummary(scheduler, summarizeInterval, record, stopNotifier)
	// 检查计数通道
	checkCountChan := make(chan uint64, 2)
	// 检查空闲状态
	checkStatus(
		scheduler,
		checkInterval,
		maxIdleCount,
		autoStop,
		checkCountChan,
		record,
		stopFunc)

	return checkCountChan
}

// 检查状态，并在满足持续空闲时间的条件时采取必要的措施
func checkStatus(
	scheduler sched.Scheduler,
	checkInterval time.Duration,
	maxIdleCount uint,
	autoStop bool,
	checkCountChan chan<- uint64,
	record Record,
	stopFunc context.CancelFunc) {
	go func() {
		var checkCount uint64
		defer func() {
			stopFunc()
			checkCountChan <- checkCount
		}()
		// 等待调度器开启
		waitForSchedulerStart(scheduler)
		// 准备
		var idleCount uint
		var firstIdleTime time.Time
		for {
			// 检查调度器的空闲状态
			if scheduler.Idle() {
				idleCount++
				if idleCount == 1 {
					firstIdleTime = time.Now()
				}
				if idleCount >= maxIdleCount {
					msg := fmt.Sprintf(msgReachMaxIdleCount, time.Since(firstIdleTime).String())
					record(0, msg)
					// 再次检查调度器的空闲状态，确保它已经可以被停止
					if scheduler.Idle() {
						if autoStop {
							var result string
							if err := scheduler.Stop(); err == nil {
								result = "success"
							} else {
								result = fmt.Sprintf("failing(%s)", err)
							}
							msg = fmt.Sprintf(msgStopScheduler, result)
							record(0, msg)
						}
						break
					} else {
						if idleCount > 0 {
							idleCount = 0
						}
					}
				}
			} else {
				if idleCount > 0 {
					idleCount = 0
				}
			}
			checkCount++
			time.Sleep(checkInterval)
		}
	}()
}

// 记录摘要信息
func recordSummary(
	scheduler sched.Scheduler,
	summarizeInterval time.Duration,
	record Record,
	stopNotifier context.Context) {
	go func() {
		// 等待调度器开启
		waitForSchedulerStart(scheduler)
		// 准备
		var prevSchedSummaryStruct sched.SummaryStruct
		var preNumGoroutine int
		var recordCount uint64 = 1
		startTime := time.Now()
		for {
			// 检查监控停止通知器
			select {
			case <-stopNotifier.Done():
				return
			default:
			}
			// 获取 Goroutine 数量和调度器摘要信息
			currNumGoroutine := runtime.NumGoroutine()
			currSchedSummaryStruct := scheduler.Summary().Struct()
			// 对比前后两份摘要信息的一致性。只有不一致时才会记录
			if currNumGoroutine != preNumGoroutine ||
				!currSchedSummaryStruct.Same(prevSchedSummaryStruct) {
				// 记录摘要信息
				summary := summary{
					NumGoroutine: runtime.NumGoroutine(),
					SchedSummary: currSchedSummaryStruct,
					EscapedTime:  time.Since(startTime).String(),
				}
				b, err := json.MarshalIndent(summary, "", "    ")
				if err != nil {
					log.L().Sugar().Errorf("An error occurs when generating scheduler summary; %s\n", err)
					continue
				}
				msg := fmt.Sprintf("Monitor summary[%d]:\n%s", recordCount, b)
				record(0, msg)
				preNumGoroutine = currNumGoroutine
				prevSchedSummaryStruct = currSchedSummaryStruct
				recordCount++
			}
			time.Sleep(summarizeInterval)
		}
	}()
}

// 接收和报告错误
func reportError(
	scheduler sched.Scheduler,
	record Record,
	stopNotifier context.Context) {
	go func() {
		// 等待调度器开启
		waitForSchedulerStart(scheduler)
		errorChan := scheduler.ErrorChan()
		for {
			// 查看监控停止通知器
			select {
			case <-stopNotifier.Done():
				return
			default:
			}
			err, ok := <-errorChan
			if ok {
				errMsg := fmt.Sprintf("Received an error from error channel: %s", err)
				record(2, errMsg)
			}
			time.Sleep(time.Microsecond)
		}
	}()
}

// 等待调度器开启
func waitForSchedulerStart(scheduler sched.Scheduler) {
	for scheduler.Status() != sched.SCHED_STATUS_STARTED {
		time.Sleep(time.Microsecond)
	}
}
