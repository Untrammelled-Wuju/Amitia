package main

import (
	"fmt"
	"reflect"
)

func validateMeshBusinessWiring(services *AppServices) error {
	if services == nil {
		return fmt.Errorf("设备协同服务未初始化")
	}
	if !services.RuntimeProfile.IsCore() {
		return nil
	}
	if services.DeviceMesh == nil || services.OwnedBusiness == nil || services.DeviceMesh.LocalHandler == nil {
		return fmt.Errorf("Core 的设备协同业务未初始化")
	}
	if services.DB == nil || services.KernelContainer == nil || services.KernelContainer.DeviceRegistry == nil || services.KernelContainer.DeviceRegistry != services.DeviceMesh.DeviceReg || services.KernelContainer.DeviceRegistry.Database() != services.DeviceMesh.DB {
		return fmt.Errorf("Core 业务与内核设备认证必须使用同一权威数据源")
	}
	corePort, coreOK := services.DeviceMesh.CoreDataPort.(*meshLocalDataPort)
	sourcePort, sourceOK := services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
	if !coreOK || !sourceOK || corePort == nil || sourcePort == nil || corePort.services != services || sourcePort.services != services || corePort.store == nil || sourcePort.store == nil || meshReadinessPortMissing(corePort.roles) || meshReadinessPortMissing(sourcePort.roles) || sourcePort.ownerID != services.DeviceMesh.LocalDeviceID {
		return fmt.Errorf("Core 与来源设备的角色和数据端口未绑定当前服务与原所有者")
	}
	if err := services.DeviceMesh.LocalHandler.ValidateOwnerWiring(corePort.ownerID, sourcePort.ownerID); err != nil {
		return err
	}
	if err := services.OwnedBusiness.ValidateWiring(); err != nil {
		return err
	}
	if err := services.OwnedBusiness.ValidateRuntimeWiring(services.DeviceMesh.Coordination, services.DeviceMesh); err != nil {
		return err
	}
	return services.DeviceMesh.ValidateBusinessWiring()
}

func meshReadinessPortMissing(port any) bool {
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
