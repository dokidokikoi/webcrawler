package module

import (
	"fmt"
	"sync"

	"github.com/dokidokikoi/webcrawler/errors"
)

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

type myRegistrar struct {
	// 组件类型与对应的组件实例映射
	moduleTypeMap map[Type]map[MID]Module
	rwlock        sync.RWMutex
}

func (r *myRegistrar) Register(module Module) (bool, error) {
	// 检查参数
	if module == nil {
		return false, errors.NewIllegalParameterError("nil module instance")
	}
	mid := module.ID()
	parts, err := SplitMID(mid)
	if err != nil {
		return false, err
	}
	moduleType := legalLetterTypeMap[parts[0]]
	if !CheckType(moduleType, module) {
		errMsg := fmt.Sprintf("incorrect module type: %s, moduleType", moduleType)
		return false, errors.NewIllegalParameterError(errMsg)
	}

	// 存入 map
	r.rwlock.Lock()
	defer r.rwlock.Unlock()
	modules := r.moduleTypeMap[moduleType]
	if modules == nil {
		modules = map[MID]Module{}
	}
	if _, ok := modules[mid]; ok {
		return false, nil
	}
	modules[mid] = module
	r.moduleTypeMap[moduleType] = modules
	return true, nil
}

func (r *myRegistrar) Unregister(mid MID) (bool, error) {
	parts, err := SplitMID(mid)
	if err != nil {
		return false, err
	}
	moduleType := legalLetterTypeMap[parts[0]]
	var deleted bool
	r.rwlock.Lock()
	defer r.rwlock.Unlock()
	if modules, ok := r.moduleTypeMap[moduleType]; ok {
		if _, ok := modules[mid]; ok {
			delete(modules, mid)
			deleted = true
		}
	}
	return deleted, nil
}

// 获取一个指定类型的组件实例
// 基于负载均衡策略,返回得分最低者
func (r *myRegistrar) Get(moduleType Type) (Module, error) {
	modules, err := r.GetAllByType(moduleType)
	if err != nil {
		return nil, err
	}
	minScore := uint64(0)
	var selectedModule Module
	for _, module := range modules {
		SetScore(module)
		if err != nil {
			return nil, err
		}
		score := module.Score()
		if minScore == 0 || score < minScore {
			selectedModule = module
			minScore = score
		}
	}
	return selectedModule, nil
}

// GetAllByType 用于获取指定类型的所有组件实例。
func (registrar *myRegistrar) GetAllByType(moduleType Type) (map[MID]Module, error) {
	if !LegalType(moduleType) {
		errMsg := fmt.Sprintf("illegal module type: %s", moduleType)
		return nil, errors.NewIllegalParameterError(errMsg)
	}
	registrar.rwlock.RLock()
	defer registrar.rwlock.RUnlock()
	modules := registrar.moduleTypeMap[moduleType]
	if len(modules) == 0 {
		return nil, ErrNotFoundModuleInstance
	}
	result := map[MID]Module{}
	for mid, module := range modules {
		result[mid] = module
	}
	return result, nil
}

// GetAll 用于获取所有组件实例。
func (registrar *myRegistrar) GetAll() map[MID]Module {
	result := map[MID]Module{}
	registrar.rwlock.RLock()
	defer registrar.rwlock.RUnlock()
	for _, modules := range registrar.moduleTypeMap {
		for mid, module := range modules {
			result[mid] = module
		}
	}
	return result
}

// Clear 会清除所有的组件注册记录。
func (registrar *myRegistrar) Clear() {
	registrar.rwlock.Lock()
	defer registrar.rwlock.Unlock()
	registrar.moduleTypeMap = map[Type]map[MID]Module{}
}

// NewRegistrar 用于创建一个组件注册器的实例。
func NewRegistrar() Registrar {
	return &myRegistrar{
		moduleTypeMap: map[Type]map[MID]Module{},
	}
}
