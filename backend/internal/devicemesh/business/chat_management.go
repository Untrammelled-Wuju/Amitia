package business

import (
	"context"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) ManageChat(ctx context.Context, request Request, operation coordination.ChatManagementRequest) (map[string]any, error) {
	if request.ExpectedScope == nil || request.ExpectedScope.RoleID != request.RoleID || request.ExpectedScope.TargetDeviceID != request.TargetDeviceID || request.ExpectedScope.SpaceID != request.SpaceID || request.ExpectedScope.InitiatorDeviceID != request.DeviceID || request.ExpectedScope.CoreID != request.CoreID {
		return nil, coordination.ErrScopeExpired
	}
	if operation.ConversationOwnerID == "" {
		operation.ConversationOwnerID = request.ExpectedScope.ResourceOwnerID
	}
	if operation.ConversationOwnerID != request.ExpectedScope.ResourceOwnerID && (!request.ExpectedScope.Coordinated || operation.ConversationOwnerID != request.ExpectedScope.TargetDeviceID) {
		return nil, coordination.ErrWrongOwner
	}
	if len(operation.ConversationIDs) > 256 || len(operation.Keyword) > 256 || len(operation.PreviewID) > 128 || len(operation.BeforeDate) > 64 || operation.OlderThanDays < 0 || operation.OlderThanDays > 365000 || len(operation.Channels) > 32 || len(operation.Sources) > 32 {
		return nil, coordination.ErrPendingLimit
	}
	for _, value := range append(append([]string{operation.ConversationID, operation.HistoricalRoleID}, operation.ConversationIDs...), append(operation.Channels, operation.Sources...)...) {
		if len(value) > 512 || strings.ContainsRune(value, 0) {
			return nil, coordination.ErrWrongOwner
		}
	}
	port, ok := e.data.(coordination.ChatManagementPort)
	if !ok {
		return nil, errors.New("Owner 聊天管理服务未接通")
	}
	var result map[string]any
	err := e.WithExecutionScope(ctx, *request.ExpectedScope, func(ctx context.Context) error {
		var err error
		result, err = port.ManageChat(ctx, *request.ExpectedScope, operation)
		if err == nil && operation.Operation == "from-import" {
			result, err = e.commitImportedChat(ctx, *request.ExpectedScope, operation, result)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	result["executionScope"] = *request.ExpectedScope
	return result, nil
}
