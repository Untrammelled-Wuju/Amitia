package virtualdisplay

import (
	"github.com/u-ai/backend/internal/androidnative"
)

const RuntimeID = "android_native_virtual_display"

func Register(provider *androidnative.NativeProviderAdapter, bridge androidnative.NativeBridge) *Service {
	store := &Store{}
	var virtualBridge VirtualBridge
	if bridge != nil {
		virtualBridge = NewNativeBridgeAdapter(bridge)
	}
	policy := DefaultPolicy()
	resolver := NewDefaultResolver(store)
	service := NewService(store, virtualBridge, policy, resolver)
	handler := NewHandler(service)
	provider.RegisterHandler(OperationStatus, handler)
	provider.RegisterHandler(OperationCreate, handler)
	provider.RegisterHandler(OperationGet, handler)
	provider.RegisterHandler(OperationResize, handler)
	provider.RegisterHandler(OperationRelease, handler)
	return service
}
