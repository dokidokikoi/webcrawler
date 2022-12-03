package module

// 组件注册接口
type Registrar interface {
	// 注册组件实例
	Register(module Module) (bool, error)
	// 注销组件实例
	Unregister(mid MID) (bool, error)
	// 用于获取一个指定类型的组件实例
	// 该函数基于负载均衡返回组件实例
	Get(moduleType Type) (Module, error)
	// 用于获取所有指定类型的组件实例
	GetAllByType(moduleType Type) (map[MID]Module, error)
	// 用于获取所有的组件实例
	GetAll() map[MID]Module
	// 清除所有组件的注册记录
	Clear()
}
