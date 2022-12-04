package buffer

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dokidokikoi/webcrawler/errors"
)

// 数据缓冲器接口
type Buffer interface {
	// 获取本缓冲器的容量
	Cap() uint32
	// 获取本缓冲器中数据的数量
	Len() uint32
	// 向缓冲器放入数据
	// 注意：本方法是非阻塞的
	// 若缓冲器已关闭，则会返回非 nil 的错误值
	Put(datum interface{}) (bool, error)
	// 从缓冲器获取数据
	// 注意：本方法是非阻塞的
	// 若缓冲器已关闭，则会返回非 nil 的错误值
	Get() (interface{}, error)
	// 关闭缓冲器，缓冲器关闭后调用返回 false
	Close() bool
	// 判断缓冲器是否关闭
	Closed() bool
}

// 缓冲器接口实现
type myBuffer struct {
	// 存放数据的通道
	ch chan interface{}
	// 缓冲器的关闭状态，0-未关闭；1-已关闭
	closed uint32
	// 为了消除因关闭缓冲器而产生的竞态条件的读写锁
	closingLock sync.RWMutex
}

// 注意 Put 使用读锁，”向通道发送值“的操作受到“关闭通道”的操作的影响
// 如果不关闭通道的话，根本不需要使用锁
// 这里使用 select 让 Put 变成非阻塞操作
func (buf *myBuffer) Put(datum interface{}) (ok bool, err error) {
	buf.closingLock.RLock()
	defer buf.closingLock.Unlock()

	if buf.Closed() {
		return false, ErrClosedBuffer
	}
	select {
	case buf.ch <- datum:
		ok = true
	default:
		ok = false
	}

	return
}

func (buf *myBuffer) Get() (interface{}, error) {
	select {
	case datum, ok := <-buf.ch:
		if !ok {
			return nil, ErrClosedBuffer
		}
		return datum, nil
	default:
		return nil, nil
	}
}

func (buf *myBuffer) Close() bool {
	if atomic.CompareAndSwapUint32(&buf.closed, 0, 1) {
		buf.closingLock.Lock()
		close(buf.ch)
		buf.closingLock.Unlock()
		return true
	}
	return false
}

func (buf *myBuffer) Closed() bool {
	return atomic.LoadUint32(&buf.closed) != 0
}

func (buf *myBuffer) Cap() uint32 {
	return uint32(cap(buf.ch))
}

func (buf *myBuffer) Len() uint32 {
	return uint32(len(buf.ch))
}

// @Param size 缓冲器容量
func NewBuffer(size uint32) (Buffer, error) {
	if size == 0 {
		errMsg := fmt.Sprintf("illegal size for buffer: %d", size)
		return nil, errors.NewIllegalParameterError(errMsg)
	}
	return &myBuffer{ch: make(chan interface{}, size)}, nil
}
