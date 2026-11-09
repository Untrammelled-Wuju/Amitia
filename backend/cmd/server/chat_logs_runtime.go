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
)

type chatLogsHostAdapter struct {
	services *AppServices
	coreID   string
}

func (a chatLogsHostAdapter) ExecuteChatLogs(
	ctx context.Context,
	request host_api.CallRequest,
	method host_api.Method,
	operation string,
	input json.RawMessage,
) (any, error) {
	if a.services == nil || a.services.KernelContainer == nil || a.services.DeviceMesh == nil || a.services.OwnedBusiness == nil {
		return nil, errors.New("归档对话服务未接通")
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
	case host_api.MethodConversationLogsRead:
		return a.read(ctx, actor, operation, body)
	case host_api.MethodConversationLogsManage:
		return a.manage(ctx, actor, operation, body)
	case host_api.MethodConversationLogsExport:
		return a.export(ctx, actor, body)
	default:
		return nil, errors.New("不支持的归档对话 Host API")
	}
}

func (a chatLogsHostAdapter) read(ctx context.Context, actor storageActor, operation string, body map[string]any) (any, error) {
	roleID := storageString(body, "roleId")
	if roleID == "" {
		return nil, errors.New("归档对话缺少角色")
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
	switch operation {
	case "list":
		return a.services.OwnedBusiness.ManageChat(ctx, request, coordination.ChatManagementRequest{
			Operation:           "conversations",
			ConversationOwnerID: expected.ResourceOwnerID,
			Page:                storageInt(body, "page"),
			PageSize:            storageInt(body, "pageSize"),
			ArchivedOnly:        true,
			RequestID:           uuid.NewString(),
		})
	case "messages":
		return a.services.OwnedBusiness.ManageChat(ctx, request, coordination.ChatManagementRequest{
			Operation:           "message-range",
			ConversationOwnerID: expected.ResourceOwnerID,
			ConversationID:      storageString(body, "conversationId"),
			Limit:               max(storageInt(body, "limit"), 1),
			ArchivedOnly:        true,
			RequestID:           uuid.NewString(),
		})
	default:
		return nil, errors.New("不支持的归档读取操作: " + operation)
	}
}

func (a chatLogsHostAdapter) manage(ctx context.Context, actor storageActor, operation string, body map[string]any) (any, error) {
	roleID := storageString(body, "roleId")
	conversationID := storageString(body, "conversationId")
	if roleID == "" || conversationID == "" {
		return nil, errors.New("归档管理缺少角色或会话")
	}
	expected, err := a.workingScope(ctx, actor, roleID)
	if err != nil {
		return nil, err
	}
	base := business.Request{
		SpaceID:         actor.SpaceID,
		DeviceID:        actor.DeviceID,
		CoreID:          a.coreID,
		TargetDeviceID:  expected.TargetDeviceID,
		ResourceOwnerID: expected.ResourceOwnerID,
		RoleOwnerID:     expected.RoleOwnerID,
		RoleID:          roleID,
		ConversationID:  conversationID,
		ExpectedScope:   &expected,
		RequestID:       uuid.NewString(),
	}
	switch operation {
	case "restore":
		conversation, resultScope, err := a.archivedConversation(ctx, base, conversationID)
		if err != nil {
			return nil, err
		}
		expectedCopy := resultScope
		base.ExpectedScope = &expectedCopy
		base.TargetDeviceID = expectedCopy.TargetDeviceID
		base.ResourceOwnerID = expectedCopy.ResourceOwnerID
		base.RoleOwnerID = expectedCopy.RoleOwnerID
		ack, err := a.services.OwnedBusiness.Edit(ctx, base, business.EditRequest{
			RequestID:        uuid.NewString(),
			Kind:             "conversation",
			ID:               conversationID,
			ExpectedRevision: conversation.Revision,
			RoleID:           roleID,
			ExpectedScope:    &expectedCopy,
			Changes:          map[string]json.RawMessage{"archived": json.RawMessage(`false`)},
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"restored": true, "conversationId": conversationID, "acknowledgement": ack}, nil
	case "delete":
		if _, _, err := a.archivedConversation(ctx, base, conversationID); err != nil {
			return nil, err
		}
		return a.services.OwnedBusiness.ManageChat(ctx, base, coordination.ChatManagementRequest{
			Operation:           "conversation/delete",
			ConversationOwnerID: expected.ResourceOwnerID,
			ConversationID:      conversationID,
			ArchivedOnly:        true,
			RequestID:           uuid.NewString(),
		})
	case "summary":
		if _, _, err := a.archivedConversation(ctx, base, conversationID); err != nil {
			return nil, err
		}
		query, err := a.services.OwnedBusiness.Query(ctx, base, coordination.DataQuery{
			ResourceKind:   "summary",
			ConversationID: conversationID,
			Limit:          1,
		})
		if err != nil {
			return nil, err
		}
		expectedRevision := int64(0)
		for _, resource := range query.Snapshot.Resources {
			if resource.Kind == "summary" && resource.ID == conversationID+"/summary" && !resource.Deleted {
				expectedRevision = resource.Revision
			}
		}
		expectedCopy := query.Scope
		base.ExpectedScope = &expectedCopy
		base.TargetDeviceID = expectedCopy.TargetDeviceID
		base.ResourceOwnerID = expectedCopy.ResourceOwnerID
		base.RoleOwnerID = expectedCopy.RoleOwnerID
		return a.services.OwnedBusiness.GenerateSummary(ctx, base, expectedRevision)
	default:
		return nil, errors.New("不支持的归档管理操作: " + operation)
	}
}

func (a chatLogsHostAdapter) export(ctx context.Context, actor storageActor, body map[string]any) (any, error) {
	roleID := storageString(body, "roleId")
	conversationID := storageString(body, "conversationId")
	if roleID == "" || conversationID == "" {
		return nil, errors.New("归档导出缺少角色或会话")
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
	if _, _, err := a.archivedConversation(ctx, request, conversationID); err != nil {
		return nil, err
	}
	return a.services.OwnedBusiness.ManageChat(ctx, request, coordination.ChatManagementRequest{
		Operation:           "export",
		ConversationOwnerID: expected.ResourceOwnerID,
		ConversationID:      conversationID,
		Format:              "json",
		ArchivedOnly:        true,
		RequestID:           uuid.NewString(),
	})
}

func (a chatLogsHostAdapter) archivedConversation(ctx context.Context, request business.Request, conversationID string) (*coordination.Resource, coordination.ExecutionScope, error) {
	result, err := a.services.OwnedBusiness.Query(ctx, request, coordination.DataQuery{
		ResourceKind:   "conversation",
		ConversationID: conversationID,
		Limit:          1,
	})
	if err != nil {
		return nil, coordination.ExecutionScope{}, err
	}
	for index := range result.Snapshot.Resources {
		resource := &result.Snapshot.Resources[index]
		if resource.Kind != "conversation" || resource.ID != conversationID || resource.Deleted {
			continue
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			return nil, coordination.ExecutionScope{}, err
		}
		var archivedAt string
		if err := json.Unmarshal(document["archivedAt"], &archivedAt); err != nil || strings.TrimSpace(archivedAt) == "" {
			return nil, coordination.ExecutionScope{}, errors.New("归档对话不存在或已撤销归档")
		}
		return resource, result.Scope, nil
	}
	return nil, coordination.ExecutionScope{}, errors.New("归档对话不存在或已撤销归档")
}

func (a chatLogsHostAdapter) workingScope(ctx context.Context, actor storageActor, roleID string) (coordination.ExecutionScope, error) {
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

func (a chatLogsHostAdapter) resolveActor(ctx context.Context, request host_api.CallRequest) (storageActor, error) {
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
		return storageActor{}, errors.New("归档对话缺少设备身份")
	}
	return actor, nil
}
