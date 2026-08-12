package display

import (
	"context"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func Register(adapter *androidnative.NativeProviderAdapter, bridge androidnative.NativeBridge, cap DisplayCapability) *DisplayService {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	listener := NewListener()
	topology := NewTopologyAdapter(false)
	policy := DefaultSelectionPolicy
	resolver := NewDefaultResolver(store, policy)
	svc := NewDisplayService(store, classifier, listener, resolver, topology, policy, cap)
	handler := NewHandler(svc)
	adapter.RegisterHandler(OperationStatus, handler)
	adapter.RegisterHandler(OperationList, handler)
	adapter.RegisterHandler(OperationGet, handler)
	adapter.RegisterHandler(OperationResolve, handler)
	return svc
}

type Handler struct {
	svc *DisplayService
}

func NewHandler(svc *DisplayService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	return h.svc.Handle(ctx, request)
}

var _ androidnative.Handler = (*Handler)(nil)
