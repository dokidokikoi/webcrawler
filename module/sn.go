package module

// 序列号生成器接口
type SNGenertor interface {
	// 用于获取预设的最小序列号
	Start() uint64
	// 用于获取预设的最大序列号
	Max() uint64
	// 用于获取下一个序列号
	Next() uint64
	// 用于获取循环计数
	CycleCount() uint64
	// 用于获取序列号并准备下一个序列号
	Get() uint64
}
