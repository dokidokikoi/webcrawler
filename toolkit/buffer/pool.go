package buffer

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
