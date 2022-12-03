package errors

import (
	"bytes"
	"fmt"
	"strings"
)

type ErrorType string

const (
	// 下载器错误
	ERROR_TYPE_DOWNLOADER ErrorType = "downloader error"
	// 分析器错误
	ERROR_TYPE_ANALYZER ErrorType = "analyzer error"
	// 条目处理管道错误
	ERROR_TYPE_PIPELINE ErrorType = "pipeline error"
	// 调度器错误
	ERROR_TYPE_SCHEDULER ErrorType = "scheduler error"
)

type CrawlerError interface {
	Type() ErrorType
	Error() string
}

type myCrawlerError struct {
	errType    ErrorType
	errMsg     string
	fullErrMsg string
}

func (e *myCrawlerError) Type() ErrorType {
	return e.errType
}

func (e *myCrawlerError) Error() string {
	if e.fullErrMsg == "" {
		e.genFullErrMsg()
	}
	return e.fullErrMsg
}

func (e *myCrawlerError) genFullErrMsg() {
	var buffer bytes.Buffer
	buffer.WriteString("crawler error: ")
	if e.errType != "" {
		buffer.WriteString(string(e.errType))
		buffer.WriteString(": ")
	}
	buffer.WriteString(e.errMsg)
	e.fullErrMsg = fmt.Sprintf("%s", buffer.String())
}

func NewCrawlerError(errType ErrorType, errMsg string) CrawlerError {
	return &myCrawlerError{errType: errType, errMsg: strings.TrimSpace(errMsg)}
}
