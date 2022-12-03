package scheduler

// Status 代表调度器状态的类型。
type Status uint8

// 当调度器处于正在初始化、正在启动或正在停止状态时，
// 不能由外部触发状态的变化。也就是不能被初始化、启动或停止
//
// 处于未初始化状态时，调度器不能被启动或停止。
//
// 处于已启动状态时，调度器不能被初始化或启动。
// 调度器可以被再初始化，但是必须在未启动的情况下。
// 调用运行中调度器的 Start 方法是不会成功的
//
// 仅当调度器处于已启动状态时，才能被停止。
const (
	// 未初始化状态
	SCHED_STATUS_UNINITIALIZED Status = iota
	// 正在初始化
	SCHED_STATUS_INITIALIZING
	// 已初始化
	SCHED_STATUS_INITIALIZED
	// 启动中
	SCHED_STATUS_STARTING
	// 启动完成
	SCHED_STATUS_STARTED
	// 停止中
	SCHED_STATUS_STOPPING
	// 已停止
	SCHED_STATUS_STOPPED
)
