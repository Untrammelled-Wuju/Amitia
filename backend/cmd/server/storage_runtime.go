package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/system"
)

type storageHostAdapter struct {
	service  system.Service
	services *AppServices
	coreID   string
}

type storageActor struct {
	SpaceID  string
	DeviceID string
}

func (a storageHostAdapter) ExecuteStorage(
	ctx context.Context,
	request host_api.CallRequest,
	method host_api.Method,
	operation string,
	input json.RawMessage,
) (any, error) {
	if a.service == nil || a.services == nil || a.services.KernelContainer == nil || a.services.DeviceMesh == nil {
		return nil, errors.New("存储服务未接通")
	}
	actor, err := a.resolveActor(ctx, request)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &body); err != nil {
			return nil, err
		}
	}
	if body == nil {
		body = map[string]any{}
	}
	switch method {
	case host_api.MethodStorageCleanup:
		return a.cleanup(ctx, actor, operation, body)
	case host_api.MethodStorageBackup:
		return a.backup(ctx, operation, body)
	case host_api.MethodStorageRestore:
		return a.restore(ctx, operation, body)
	default:
		return nil, errors.New("不支持的存储 Host API")
	}
}

func (a storageHostAdapter) cleanup(ctx context.Context, actor storageActor, operation string, body map[string]any) (any, error) {
	if operation == "info" {
		return a.service.GetStorageInfo(), nil
	}
	roleID := strings.TrimSpace(storageString(body, "roleId"))
	if roleID == "" {
		return nil, errors.New("清理任务缺少角色")
	}
	expected, err := a.workingScope(ctx, actor, roleID)
	if err != nil {
		return nil, err
	}
	request := business.Request{
		SpaceID:         actor.SpaceID,
		DeviceID:        actor.DeviceID,
		CoreID:          a.coreID,
		TargetDeviceID:  expected.TargetDeviceID,
		ResourceOwnerID: expected.ResourceOwnerID,
		RoleOwnerID:     expected.RoleOwnerID,
		RoleID:          roleID,
		ExpectedScope:   &expected,
		RequestID:       uuid.NewString(),
	}
	management := coordination.ChatManagementRequest{
		Operation:           operation,
		ConversationOwnerID: expected.ResourceOwnerID,
		RequestID:           uuid.NewString(),
		BeforeDate:          storageString(body, "beforeDate"),
		OlderThanDays:       storageInt(body, "olderThanDays"),
		Channels:            storageStrings(body, "channels"),
		Sources:             storageStrings(body, "sources"),
		IncludeMemories:     storageBool(body, "includeMemories"),
		PreviewID:           storageString(body, "previewId"),
		ConfirmText:         storageString(body, "confirmText"),
	}
	switch operation {
	case "stats":
		management.Operation = "stats"
	case "vacuum":
		management.Operation = "cleanup/vacuum"
	case "preview":
		management.Operation = "cleanup/preview"
	case "confirm":
		management.Operation = "cleanup/confirm"
	default:
		return nil, errors.New("不支持的存储清理操作: " + operation)
	}
	return a.services.OwnedBusiness.ManageChat(ctx, request, management)
}

func (a storageHostAdapter) backup(ctx context.Context, operation string, body map[string]any) (any, error) {
	switch operation {
	case "list":
		return a.service.ListDataPortabilityBackups()
	case "create":
		return a.service.CreateDataPortabilityBackup(ctx, storageString(body, "passphrase"))
	case "delete":
		if err := a.service.DeleteDataPortabilityBackup(storageString(body, "name")); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, errors.New("不支持的备份操作: " + operation)
	}
}

func (a storageHostAdapter) restore(ctx context.Context, operation string, body map[string]any) (any, error) {
	switch operation {
	case "preview":
		return a.service.PreviewDataPortabilityRestore(ctx, storageString(body, "name"), storageString(body, "passphrase"))
	case "execute":
		return a.service.ExecuteDataPortabilityRestore(ctx, storageString(body, "operationId"))
	case "cancel":
		if err := a.service.CancelDataPortabilityRestore(); err != nil {
			return nil, err
		}
		return map[string]any{"cancelled": true}, nil
	default:
		return nil, errors.New("不支持的恢复操作: " + operation)
	}
}

func (a storageHostAdapter) workingScope(ctx context.Context, actor storageActor, roleID string) (coordination.ExecutionScope, error) {
	result, err := a.services.OwnedBusiness.Query(ctx, business.Request{
		SpaceID:         actor.SpaceID,
		DeviceID:        actor.DeviceID,
		CoreID:          a.coreID,
		TargetDeviceID:  actor.DeviceID,
		ResourceOwnerID: actor.DeviceID,
		RoleOwnerID:     a.coreID,
		RoleID:          roleID,
		RequestID:       uuid.NewString(),
	}, coordination.DataQuery{ResourceKind: "working", Management: true, Limit: 1})
	if err != nil {
		return coordination.ExecutionScope{}, err
	}
	return result.Scope, nil
}

func (a storageHostAdapter) resolveActor(ctx context.Context, request host_api.CallRequest) (storageActor, error) {
	actor := storageActor{}
	if request.ScopeSnapshotID != "" {
		store := scope.NewSQLiteScopeStore(a.services.KernelContainer.DeviceRegistry.Database())
		snapshot, err := store.GetSnapshot(ctx, request.ScopeSnapshotID)
		if err != nil {
			return actor, err
		}
		if len(snapshot.OwnedExecutionScope) > 0 {
			var owned coordination.ExecutionScope
			if err := json.Unmarshal(snapshot.OwnedExecutionScope, &owned); err != nil {
				return actor, err
			}
			actor.SpaceID = owned.SpaceID
			actor.DeviceID = owned.InitiatorDeviceID
		}
	}
	if authenticated, ok := auth.FromContext(ctx); ok && authenticated != nil {
		actor.SpaceID = authenticated.SpaceID.String()
		if authenticated.DeviceID != "" {
			actor.DeviceID = authenticated.DeviceID.String()
		}
	}
	if actor.SpaceID == "" && a.services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
		actor.SpaceID = a.coreID
	}
	if actor.DeviceID == "" && a.services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
		actor.DeviceID = a.services.DeviceMesh.LocalDeviceID
	}
	if actor.SpaceID == "" || actor.DeviceID == "" {
		return storageActor{}, errors.New("存储操作缺少设备身份")
	}
	return actor, nil
}

func storageString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func storageInt(values map[string]any, key string) int {
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func storageBool(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func storageStrings(values map[string]any, key string) []string {
	raw, ok := values[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}
