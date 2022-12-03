package reader

import "io"

// 多重读取器的接口
type MultipleReader interface {
	// 用于获取一个可关闭读取器的实例
	// 持有该多重读取器中的值
	Reader() io.ReadCloser
}
