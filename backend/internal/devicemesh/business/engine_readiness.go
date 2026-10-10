package business

import (
	"errors"
	"reflect"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) ValidateRuntimeWiring(service *coordination.Service, data coordination.DataPort) error {
	if e == nil || service == nil || e.coordination != service || ownedDependencyMissing(data) || ownedDependencyMissing(e.data) || ownedDependencyMissing(e.model) {
		return errors.New("归属业务必须使用当前 Core 的协调、数据与计算端口")
	}
	expected, actual := reflect.ValueOf(data), reflect.ValueOf(e.data)
	if expected.Type() != actual.Type() || !expected.Comparable() || !expected.Equal(actual) {
		return errors.New("归属业务的数据端口未绑定当前 Core Runtime")
	}
	if _, ok := e.data.(coordination.ResourcePort); !ok {
		return errors.New("归属业务缺少原资源和幂等检查点读取端口")
	}
	if e.lanes == nil || e.active == nil || e.continuitySlots == nil {
		return errors.New("归属业务的串行执行与取消管理未初始化")
	}
	return nil
}

func ownedDependencyMissing(value any) bool {
	if value == nil {
		return true
	}
	dependency := reflect.ValueOf(value)
	switch dependency.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return dependency.IsNil()
	}
	return false
}
