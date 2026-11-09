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

type chatImportHostAdapter struct {
	handler  *system.Handler
	services *AppServices
	coreID   string
}

func (a chatImportHostAdapter) ExecuteChatImport(
	ctx context.Context,
	request host_api.CallRequest,
	operation string,
	input json.RawMessage,
) (any, error) {
	if a.handler == nil || a.services == nil || a.services.KernelContainer == nil || a.services.DeviceMesh == nil {
		return nil, errors.New("聊天导入服务未接通")
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
	switch operation {
	case "list_batches":
		return a.handler.ImportBatches(ctx, actor.SpaceID)
	case "batch_detail":
		return a.handler.ImportBatchDetail(ctx, actor.SpaceID, stringValue(body, "batchId"))
	case "batch_summary":
		return a.handler.ImportBatchSummary(ctx, actor.SpaceID, stringValue(body, "batchId"))
	case "delete_batch":
		if err := a.handler.DeleteImportBatch(ctx, actor.SpaceID, stringValue(body, "batchId")); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true}, nil
	case "generate_summary":
		return a.handler.GenerateImportBatchSummary(ctx, actor.SpaceID, stringValue(body, "batchId"))
	case "parse":
		return a.handler.ParseImportText(ctx, body)
	case "confirm":
		return a.handler.ConfirmImport(ctx, actor, body)
	case "promote":
		return a.promote(ctx, actor, body)
	default:
		return nil, errors.New("不支持的聊天导入操作: " + operation)
	}
}

func (a chatImportHostAdapter) resolveActor(ctx context.Context, request host_api.CallRequest) (system.ImportActor, error) {
	actor := system.ImportActor{}
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
		return system.ImportActor{}, errors.New("聊天导入缺少设备身份")
	}
	return actor, nil
}

func (a chatImportHostAdapter) promote(ctx context.Context, actor system.ImportActor, body map[string]any) (any, error) {
	batchID := strings.TrimSpace(stringValue(body, "batchId"))
	roleID := strings.TrimSpace(stringValue(body, "roleId"))
	if batchID == "" || roleID == "" {
		return nil, errors.New("导入批次和目标角色不能为空")
	}
	base := business.Request{
		SpaceID:         actor.SpaceID,
		DeviceID:        actor.DeviceID,
		CoreID:          a.coreID,
		TargetDeviceID:  actor.DeviceID,
		ResourceOwnerID: actor.DeviceID,
		RoleOwnerID:     a.coreID,
		RoleID:          roleID,
		RequestID:       uuid.NewString(),
	}
	prepared, err := a.services.OwnedBusiness.Query(ctx, base, coordination.DataQuery{
		ResourceKind: "working",
		Management:   true,
		Limit:        1,
	})
	if err != nil {
		return nil, err
	}
	scopeCopy := prepared.Scope
	expected := &scopeCopy
	request := base
	request.TargetDeviceID = expected.TargetDeviceID
	request.ResourceOwnerID = expected.ResourceOwnerID
	request.RoleOwnerID = expected.RoleOwnerID
	request.ExpectedScope = expected
	request.RequestID = uuid.NewString()
	return a.services.OwnedBusiness.ManageChat(ctx, request, coordination.ChatManagementRequest{
		Operation:     "from-import",
		ImportBatchID: batchID,
		RequestID:     uuid.NewString(),
	})
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
