package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type UIDataSourceProvider struct {
	hostAPIGateway *host_api.DefaultGateway
}

func NewUIDataSourceProvider(gateway *host_api.DefaultGateway) *UIDataSourceProvider {
	return &UIDataSourceProvider{hostAPIGateway: gateway}
}

func (p *UIDataSourceProvider) Query(ctx context.Context, sessionID, extensionID, moduleID, sourceID string, scopeSnapshotID, permissionSnapshotID string, params json.RawMessage) (json.RawMessage, error) {
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  sessionID,
		ExtensionID: domain.ExtensionID(extensionID),
		ModuleID:    domain.ModuleID(moduleID),
	}

	switch sourceID {
	case "character.summary":
		return p.queryHostAPI(ctx, identity, host_api.MethodCharacterRead, scopeSnapshotID, permissionSnapshotID, params)
	case "conversation.summary":
		return p.queryHostAPI(ctx, identity, host_api.MethodConversationRead, scopeSnapshotID, permissionSnapshotID, params)
	case "memory.query":
		return p.queryHostAPI(ctx, identity, host_api.MethodMemoryQuery, scopeSnapshotID, permissionSnapshotID, params)
	case "extension.state":
		return p.queryHostAPI(ctx, identity, host_api.MethodStateGet, scopeSnapshotID, permissionSnapshotID, params)
	case "runtime.health":
		return p.queryHostAPI(ctx, identity, host_api.MethodRuntimeHealth, scopeSnapshotID, permissionSnapshotID, params)
	default:
		return nil, fmt.Errorf("data source not found: %s", sourceID)
	}
}

func (p *UIDataSourceProvider) queryHostAPI(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, method host_api.Method, scopeSnapshotID, permissionSnapshotID string, input json.RawMessage) (json.RawMessage, error) {
	callReq := host_api.CallRequest{
		CallID:               fmt.Sprintf("ui-data-%s-%s", identity.InstanceID, uuid.NewString()),
		RuntimeIdentity:      identity,
		Method:               method,
		Version:              1,
		Input:                input,
		ScopeSnapshotID:      scopeSnapshotID,
		PermissionSnapshotID: permissionSnapshotID,
	}
	result := p.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("data source query failed: %s", result.Error.Message)
	}
	return result.Output, nil
}
