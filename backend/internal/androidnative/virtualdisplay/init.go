package virtualdisplay

import (
	"github.com/u-ai/backend/internal/androidnative"
)

func Register(provider *androidnative.Provider, bridge androidnative.NativeBridge) (*Service, error) {
	store := &Store{}
	var virtualBridge VirtualBridge
	if bridge != nil {
		virtualBridge = NewNativeBridgeAdapter(bridge)
	}
	policy := DefaultPolicy()
	resolver := NewDefaultResolver(store)
	service := NewService(store, virtualBridge, policy, resolver)
	handler := NewHandler(service)

	registerOps := []string{OperationStatus, OperationCreate, OperationGet, OperationList, OperationResize, OperationRelease}
	for _, op := range registerOps {
		if err := provider.RegisterHandler(op, handler); err != nil {
			return nil, err
		}
	}
	return service, nil
}
