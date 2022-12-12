package reader

import (
	"bytes"
	"fmt"
	"io"
)

// 多重读取器的接口
type MultipleReader interface {
	// 用于获取一个可关闭读取器的实例
	// 持有该多重读取器中的值
	Reader() io.ReadCloser
}

type myMultipleReader struct {
	data []byte
}

// 总是返回一个新的可关闭读取器
// 我们可以多次读取底层数据
func (reader *myMultipleReader) Reader() io.ReadCloser {
	return io.NopCloser(bytes.NewBuffer(reader.data))
}

func NewMultipleReader(reader io.Reader) (MultipleReader, error) {
	var data []byte
	var err error
	if reader != nil {
		data, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("multiple reader: couldn't create a new one: %s", err)
		}
	} else {
		data = []byte{}
	}
	return &myMultipleReader{data}, nil
}
