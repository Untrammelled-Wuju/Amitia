package business

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ChatPrivacyPort interface {
	PrivacyMessages(context.Context) ([]coordination.PrivacyMessage, error)
	MaskPrivacyMessage(context.Context, coordination.PrivacyMessage, string) error
}

func (e *Engine) PrivacyMessages(ctx context.Context, request Request) ([]coordination.PrivacyMessage, error) {
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return nil, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return nil, err
	}
	result := []coordination.PrivacyMessage{}
	if len(roles) == 0 {
		return nil, errors.New("当前数据持有方没有可用角色，无法建立完整隐私扫描范围")
	}
	roles = roles[:1]
	for _, role := range roles {
		current := request
		current.RoleID, current.RequestID = role.ID, uuid.NewString()
		view, err := e.Query(ctx, current, coordination.DataQuery{ResourceKind: "conversation", Management: true, Limit: 1})
		if err != nil {
			return nil, err
		}
		current.ExpectedScope, current.TargetDeviceID = &view.Scope, view.Scope.TargetDeviceID
		items, err := e.privacyMessagePages(ctx, current, coordination.ChatManagementRequest{Operation: "privacy/messages", ConversationOwnerID: view.Scope.ResourceOwnerID})
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(result) > 32768 {
			return nil, coordination.ErrPendingLimit
		}
		if role.ID == roles[0].ID && view.Scope.Coordinated && view.Scope.TargetDeviceID != view.Scope.ResourceOwnerID {
			items, err := e.privacyMessagePages(ctx, current, coordination.ChatManagementRequest{Operation: "privacy/messages", ConversationOwnerID: view.Scope.TargetDeviceID, HistoricalRoleID: view.Scope.RoleID})
			if err != nil {
				return nil, err
			}
			result = append(result, items...)
			if len(result) > 32768 {
				return nil, coordination.ErrPendingLimit
			}
		}
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Engine) MaskPrivacyMessage(ctx context.Context, request Request, item coordination.PrivacyMessage, id string) error {
	if item.Scope.SpaceID != request.SpaceID || item.Scope.InitiatorDeviceID != request.DeviceID || item.Scope.CoreID != request.CoreID {
		return coordination.ErrWrongOwner
	}
	request.ExpectedScope, request.RoleID, request.TargetDeviceID = &item.Scope, item.Scope.RoleID, item.Scope.TargetDeviceID
	response, err := e.ManageChat(ctx, request, coordination.ChatManagementRequest{Operation: "privacy/mask", RequestID: id, ConversationOwnerID: item.OwnerID, HistoricalRoleID: item.RoleID, PrivacyMessage: &item})
	if err != nil {
		return err
	}
	if response["masked"] != true {
		return errors.New("数据持有方未确认脱敏")
	}
	return nil
}

func (e *Engine) privacyMessagePages(ctx context.Context, request Request, operation coordination.ChatManagementRequest) ([]coordination.PrivacyMessage, error) {
	items := []coordination.PrivacyMessage{}
	for page := 1; page <= 256; page++ {
		operation.Page, operation.PageSize = page, 64
		response, err := e.ManageChat(ctx, request, operation)
		if err != nil {
			return nil, err
		}
		stamp, ok := response["snapshotFingerprint"].(string)
		if !ok || stamp == "" {
			return nil, coordination.ErrResourceVersion
		}
		operation.PreviewID = stamp
		raw, err := json.Marshal(response["items"])
		if err != nil {
			return nil, err
		}
		var batch []coordination.PrivacyMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		items = append(items, batch...)
		if response["hasMore"] != true {
			return items, nil
		}
	}
	return nil, coordination.ErrPendingLimit
}

func (e *Engine) WithPrivacyAuthority(ctx context.Context, request Request, run func(context.Context) error) error {
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, "", uuid.NewString())
	if err != nil {
		return err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return err
	}
	if err := run(coordination.WithScope(ctx, scope)); err != nil {
		return err
	}
	return e.coordination.Validate(ctx, scope)
}
