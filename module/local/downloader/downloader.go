package downloader

import (
	"net/http"

	"github.com/dokidokikoi/webcrawler/log"
	"github.com/dokidokikoi/webcrawler/module"
	"github.com/dokidokikoi/webcrawler/module/stub"
)

type myDownloader struct {
	stub.ModuleInternal
	// 下载用的 HTTP 客户端
	httpClient http.Client
}

func (d *myDownloader) Download(req *module.Request) (*module.Response, error) {
	d.ModuleInternal.IncrHandlingNumber()
	defer d.ModuleInternal.DecrHandlingNumber()

	d.ModuleInternal.IncrCalledCount()
	if req == nil {
		return nil, genParameterError("nil request")
	}
	httpReq := req.HTTPReq()
	if httpReq == nil {
		return nil, genParameterError("nil HTTP request")
	}
	d.ModuleInternal.IncrAcceptedCount()
	log.L().Sugar().Infof("Do the request (URL: %s, depth: %d)... \n", httpReq.URL, req.Depth())
	httpResp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	d.ModuleInternal.IncrCompletedCount()
	return module.NewResponse(httpResp, req.Depth()), nil
}

func New(mid module.MID, client *http.Client, scoreCalculator module.CalculateScore) (module.Downloader, error) {
	moduleBase, err := stub.NewModuleInternal(mid, scoreCalculator)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, genParameterError("nil http client")
	}
	return &myDownloader{moduleBase, *client}, nil
}
