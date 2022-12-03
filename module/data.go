package module

import "net/http"

type Data interface {
	// 数据是否有效
	Vaild() bool
}

type Request struct {
	// HTTP 请求
	httpReq *http.Request
	// 请求的深度
	depth uint32
}

func (r *Request) HTTPReq() *http.Request {
	return r.httpReq
}

func (r *Request) Depth() uint32 {
	return r.depth
}

func (r *Request) Vaild() bool {
	return r.httpReq != nil && r.httpReq.URL != nil
}

func NewRequest(r *http.Request, depth uint32) *Request {
	return &Request{r, depth}
}

type Response struct {
	// HTTP 响应
	httpResp *http.Response
	// 请求的深度
	depth uint32
}

func (r *Response) HTTPResq() *http.Response {
	return r.httpResp
}

func (r *Response) Depth() uint32 {
	return r.depth
}

func (r *Response) Vaild() bool {
	return r.httpResp != nil && r.httpResp.Body != nil
}

func NewRespone(resp *http.Response, depth uint32) *Response {
	return &Response{resp, depth}
}

type Item map[string]interface{}

func (item *Item) Vaild() bool {
	return item != nil
}
