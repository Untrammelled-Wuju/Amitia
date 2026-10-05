package execution

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ownedResultStoreKey struct{}

type OwnedToolResultReference struct {
	OwnedToolResult bool   `json:"ownedToolResult"`
	OwnerID         string `json:"ownerId"`
	ResourceID      string `json:"resourceId"`
	SHA256          string `json:"sha256"`
}

func WithOwnedToolResultStore(ctx context.Context, save func(context.Context, capability.UnifiedToolResult) (OwnedToolResultReference, error)) context.Context {
	return context.WithValue(ctx, ownedResultStoreKey{}, save)
}

func storeOwnedToolResult(ctx context.Context, result *capability.UnifiedToolResult) (json.RawMessage, error) {
	save, ok := ctx.Value(ownedResultStoreKey{}).(func(context.Context, capability.UnifiedToolResult) (OwnedToolResultReference, error))
	if !ok || result == nil {
		return nil, errors.New("设备工具结果缺少所有者保存端口")
	}
	reference, err := save(ctx, *result)
	if err != nil {
		return nil, err
	}
	if !reference.OwnedToolResult || reference.OwnerID == "" || reference.ResourceID == "" || len(reference.SHA256) != 64 {
		return nil, errors.New("设备工具结果保存确认无效")
	}
	if scope, owned := coordination.FromContext(ctx); !owned || reference.OwnerID != scope.ResourceOwnerID {
		return nil, coordination.ErrWrongOwner
	}
	if _, err := hex.DecodeString(reference.SHA256); err != nil {
		return nil, errors.New("设备工具结果完整性确认无效")
	}
	encoded, err := json.Marshal(reference)
	if err != nil {
		return nil, err
	}
	return json.Marshal(capability.UnifiedToolResult{InvocationID: result.InvocationID, Status: result.Status, Structured: encoded})
}
