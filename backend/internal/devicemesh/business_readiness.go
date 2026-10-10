package devicemesh

import (
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"reflect"
)

func (rt *Runtime) ValidateBusinessWiring() error {
	if rt == nil || rt.DB == nil || rt.Coordination == nil || businessPortMissing(rt.CoreDataPort) || businessPortMissing(rt.LocalDeviceDataPort) || rt.LocalDeviceID == "" {
		return errors.New("Core 或来源设备的数据归属端口未接通")
	}
	if rt.Hub == nil || rt.Handler == nil || rt.BootstrapSvc == nil || rt.Probe == nil || rt.CredentialSvc == nil || rt.DeviceReg == nil || rt.DeviceReg.Database() != rt.DB || rt.sessions == nil || businessPortMissing(rt.dispatcher) {
		return errors.New("设备认证、连接会话或执行路由未接通")
	}
	if _, ok := rt.LocalDeviceDataPort.(coordination.SourceRoleExecutionGuard); !ok {
		return errors.New("来源设备角色执行守卫未接通")
	}
	if err := rt.Coordination.ValidateDatabaseWiring(rt.DB); err != nil {
		return err
	}
	if err := rt.CredentialSvc.ValidateDatabaseWiring(rt.DB); err != nil {
		return err
	}
	if err := rt.BootstrapSvc.ValidateDatabaseWiring(rt.DB); err != nil {
		return err
	}
	for _, port := range []coordination.DataPort{rt.CoreDataPort, rt.LocalDeviceDataPort} {
		if _, ok := port.(coordination.ResourcePort); !ok {
			return errors.New("所有者原始资源与幂等检查点端口未接通")
		}
	}
	if err := rt.LocalHandler.ValidateJournalDatabaseWiring(rt.DB); err != nil {
		return err
	}
	return rt.LocalHandler.ValidateOwnedExecutionWiring()
}

func businessPortMissing(port any) bool {
	if port == nil {
		return true
	}
	value := reflect.ValueOf(port)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}
