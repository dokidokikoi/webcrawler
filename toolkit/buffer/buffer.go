package buffer

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
