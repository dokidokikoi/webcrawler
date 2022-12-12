package analyzer

import (
	"fmt"

	"github.com/dokidokikoi/webcrawler/log"
	"github.com/dokidokikoi/webcrawler/module"
	"github.com/dokidokikoi/webcrawler/module/stub"
	"github.com/dokidokikoi/webcrawler/toolkit/reader"
)

type myAnalyzer struct {
	stub.ModuleInternal
	// 响应解析器列表
	respParsers []module.ParseResponse
}

func (analyzer *myAnalyzer) RespParsers() []module.ParseResponse {
	parsers := make([]module.ParseResponse, len(analyzer.respParsers))
	copy(parsers, analyzer.respParsers)
	return parsers
}

func (a *myAnalyzer) Analyze(resp *module.Response) (dataList []module.Data, errorList []error) {
	a.ModuleInternal.IncrHandlingNumber()
	defer a.ModuleInternal.DecrHandlingNumber()

	a.ModuleInternal.IncrCalledCount()
	if resp == nil {
		errorList = append(errorList, genParameterError("nil response"))
		return
	}
	httpResp := resp.HTTPResp()
	if httpResp == nil {
		errorList = append(errorList, genParameterError("nil HTTP response"))
		return
	}
	httpReq := httpResp.Request
	if httpReq == nil {
		errorList = append(errorList, genParameterError("nil HTTP request"))
		return
	}
	reqURL := httpReq.URL
	if reqURL == nil {
		errorList = append(errorList, genParameterError("nil HTTP request URL"))
		return
	}

	a.ModuleInternal.IncrAcceptedCount()
	respDepth := resp.Depth()
	log.L().Sugar().Infof("Parse the respose (URL: %s, depth: %d)...\n", reqURL, respDepth)

	// 解析 HTTP 响应
	if httpResp.Body != nil {
		defer httpResp.Body.Close()
	}
	multipleReader, err := reader.NewMultipleReader(httpReq.Body)
	if err != nil {
		errorList = append(errorList, genError(err.Error()))
		return
	}
	dataList = []module.Data{}
	for _, respParser := range a.respParsers {
		httpResp.Body = multipleReader.Reader()
		pDataList, pErrorList := respParser(httpResp, respDepth)
		if pDataList != nil {
			for _, pData := range pDataList {
				if pData == nil {
					continue
				}
				dataList = appendDataList(dataList, pData, respDepth)
			}
		}
		if pErrorList != nil {
			for _, pError := range pErrorList {
				if pError == nil {
					continue
				}
				errorList = append(errorList, pError)
			}
		}
	}
	if len(errorList) == 0 {
		a.ModuleInternal.IncrCompletedCount()
	}
	return dataList, errorList
}

// appendDataList 用于添加请求值或条目值到列表。
func appendDataList(dataList []module.Data, data module.Data, respDepth uint32) []module.Data {
	if data == nil {
		return dataList
	}
	req, ok := data.(*module.Request)
	if !ok {
		return append(dataList, data)
	}
	newDepth := respDepth + 1
	if req.Depth() != newDepth {
		req = module.NewRequest(req.HTTPReq(), newDepth)
	}
	return append(dataList, req)
}

func New(mid module.MID,
	respParsers []module.ParseResponse,
	scoreCalculator module.CalculateScore) (module.Analyzer, error) {
	moduleBase, err := stub.NewModuleInternal(mid, scoreCalculator)
	if err != nil {
		return nil, err
	}
	if respParsers == nil {
		return nil, genParameterError("nil response parsers")
	}
	if len(respParsers) == 0 {
		return nil, genParameterError("empty response parsers list")
	}
	var innerParsers []module.ParseResponse
	for i, parser := range respParsers {
		if parser == nil {
			return nil, genParameterError(fmt.Sprintf("nil response parser[%d]", i))
		}
		innerParsers = append(innerParsers, parser)
	}
	return &myAnalyzer{moduleBase, innerParsers}, nil
}
