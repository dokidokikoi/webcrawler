package buffer

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dokidokikoi/webcrawler/errors"
)

// 数据缓冲池接口
type Pool interface {
	// 用于获取池中缓冲器的统一容量
	BufferCap() uint32
	// 用于获取池中缓冲器的最大数量
	MaxBufferNumber() uint32
	// 用于获取池中缓冲器数量
	BufferNumber() uint32
	// 用于获取缓冲池中数据的总数
	Total() uint64
	// 向缓冲池放入数据
	// 注意：本方法是阻塞的，缓冲池已满 Put 阻塞
	// 若缓冲池已关闭，则会返回非 nil 的错误值
	Put(datum interface{}) error
	// 从缓冲池获取数据
	// 注意：本方法是阻塞的，缓冲池空闲 Get 阻塞
	// 若缓冲池已关闭，则会返回非 nil 的错误值
	Get() (datum interface{}, err error)
	// 关闭缓冲池，缓冲池关闭后调用返回 false
	Close() bool
	// 判断缓冲池是否关闭
	Closed() bool
}

type myPool struct {
	// 缓冲器统一容量
	bufferCap uint32
	// 缓冲器最大容量
	maxBufferNumber uint32
	// 缓冲器实际数量
	bufferNumber uint32
	// 池中数据总数
	total uint64
	// 存放缓冲器
	// 每个缓冲器只会被一个 goroutine 拿到，
	// 在放回 bufCh 之前，它对其它 goroutine 是不可见的
	// 一个缓冲器每次只能被 goroutine 拿走和放入一个数据
	// 即使一个 goroutine 连续调用多次 Put 和 Get 也一样
	// 这样缓冲器不至于一下被填满或取空
	bufCh  chan Buffer
	closed uint32
	rwlock sync.RWMutex
}

func (pool *myPool) Put(datum interface{}) (err error) {
	if pool.Closed() {
		return ErrClosedBufferPool
	}
	var count uint32
	maxCount := pool.BufferNumber() * 5
	var ok bool
	for buf := range pool.bufCh {
		ok, err = pool.putData(buf, datum, &count, maxCount)
		if ok || err != nil {
			break
		}
	}
	return
}

// 用于向给定的缓冲器放入数据，
// 并在必要时吧缓冲器归还
func (pool *myPool) putData(buf Buffer, datum interface{}, count *uint32, maxCount uint32) (ok bool, err error) {
	if pool.Closed() {
		return false, ErrClosedBufferPool
	}
	defer func() {
		pool.rwlock.RLock()
		// 方法结束后再次检查状态，以便及时释放资源
		if pool.Closed() {
			atomic.AddUint32(&pool.bufferNumber, ^uint32(0))
			err = ErrClosedBufferPool
		} else {
			pool.bufCh <- buf
		}
		pool.rwlock.RUnlock()
	}()

	ok, err = buf.Put(datum)
	if ok {
		atomic.AddUint64(&pool.total, 1)
		return
	}
	if err != nil {
		return
	}
	// 若缓冲器已满而未放入数据，就递增计数
	(*count)++

	// 如果尝试向缓冲器中加入数据的失败次数达到阈值
	// 并且池中缓冲器的数量未达到最大值
	// 那么就尝试创建一个新缓冲器，先放入数据再把它放入池中
	if (*count) >= maxCount && pool.BufferNumber() < pool.MaxBufferNumber() {
		// 加锁防止缓冲器数量超过阈值
		pool.rwlock.Lock()
		if pool.BufferNumber() < pool.MaxBufferNumber() {
			if pool.Closed() {
				pool.rwlock.Unlock()
				return
			}
			newBuf, _ := NewBuffer(pool.bufferCap)
			newBuf.Put(datum)
			pool.bufCh <- newBuf
			atomic.AddUint32(&pool.bufferNumber, 1)
			atomic.AddUint64(&pool.total, 1)
			ok = true
		}
		pool.rwlock.Unlock()
		*count = 0
	}
	return
}

func (pool *myPool) Get() (datum interface{}, err error) {
	if pool.Closed() {
		return nil, ErrClosedBufferPool
	}
	var count uint32
	// 遍历所有缓冲器 10 次后仍然没有获取到数据，
	// Get 方法就会从缓冲池中去掉一个缓冲器
	maxCount := pool.BufferNumber() * 10
	for buf := range pool.bufCh {
		datum, err = pool.getData(buf, &count, maxCount)
		if datum != nil || err != nil {
			break
		}
	}
	return
}

func (pool *myPool) getData(buf Buffer, count *uint32, maxCount uint32) (datum interface{}, err error) {
	if pool.Closed() {
		return nil, ErrClosedBufferPool
	}
	defer func() {
		// 如果尝试从缓冲器获取数据的失次数达到阈值
		// 同时当前缓冲器已空且池中缓冲器数量大于1
		// 那么直接关掉当前缓冲器，并不归还给池
		if *count >= maxCount && buf.Len() == 0 && pool.BufferNumber() > 1 {
			buf.Close()
			atomic.AddUint32(&pool.bufferNumber, ^uint32(0))
			*count = 0
			return
		}
		pool.rwlock.RLock()
		if pool.Closed() {
			atomic.AddUint32(&pool.bufferNumber, ^uint32(0))
			err = ErrClosedBufferPool
		} else {
			pool.bufCh <- buf
		}
		pool.rwlock.RUnlock()
	}()

	datum, err = buf.Get()
	if datum != nil {
		atomic.AddUint64(&pool.total, ^uint64(0))
		return
	}
	if err != nil {
		return
	}
	// 若因缓冲器已空未取出数据
	// 递增计数
	(*count)++
	return
}

func (pool *myPool) Close() bool {
	if !atomic.CompareAndSwapUint32(&pool.closed, 0, 1) {
		return false
	}
	pool.rwlock.Lock()
	defer pool.rwlock.Unlock()
	close(pool.bufCh)
	for buf := range pool.bufCh {
		buf.Close()
	}
	return true
}

func (pool *myPool) Closed() bool {
	return atomic.LoadUint32(&pool.closed) == 1
}

func (pool *myPool) BufferCap() uint32 {
	return pool.bufferCap
}

func (pool *myPool) MaxBufferNumber() uint32 {
	return pool.maxBufferNumber
}

func (pool *myPool) BufferNumber() uint32 {
	return atomic.LoadUint32(&pool.bufferNumber)
}

func (pool *myPool) Total() uint64 {
	return atomic.LoadUint64(&pool.total)
}

// 参数bufferCap代表池内缓冲器的统一容量。
// 参数maxBufferNumber代表池中最多包含的缓冲器的数量。
func NewPool(bufferCap uint32, maxBufferNumber uint32) (Pool, error) {
	if bufferCap == 0 {
		errMsg := fmt.Sprintf("illegal buffer cap for buffer pool: %d", bufferCap)
		return nil, errors.NewIllegalParameterError(errMsg)
	}
	if maxBufferNumber == 0 {
		errMsg := fmt.Sprintf("illegal max buffer number for buffer pool: %d", maxBufferNumber)
		return nil, errors.NewIllegalParameterError(errMsg)
	}
	bufCh := make(chan Buffer, maxBufferNumber)
	buf, _ := NewBuffer(bufferCap)
	bufCh <- buf
	return &myPool{
		bufferCap:       bufferCap,
		maxBufferNumber: maxBufferNumber,
		bufferNumber:    1,
		bufCh:           bufCh,
	}, nil
}
