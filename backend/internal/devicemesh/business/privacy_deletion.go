package business

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type PrivacyDeletionPort interface {
	PrivacyDeletion(context.Context, string, coordination.PrivacyDeletionRequest, string) (map[string]any, error)
}

func (e *Engine) ManagePrivacyDeletion(ctx context.Context, request Request, operation string, input coordination.PrivacyDeletionRequest, id string) (map[string]any, error) {
	var roleErr error
	request, roleErr = e.PrivacyRequestRole(ctx, request)
	if roleErr != nil {
		return nil, roleErr
	}
	if input.Target != nil {
		request.RoleID = input.Target.ExecutionScope.RoleID
	}

	view, err := e.Query(ctx, request, coordination.DataQuery{ResourceKind: "conversation", Management: true, Limit: 1})
	if err != nil {
		return nil, err
	}
	scope := view.Scope
	owners := []string{scope.ResourceOwnerID}
	if scope.Coordinated && scope.TargetDeviceID != scope.ResourceOwnerID {
		owners = append(owners, scope.TargetDeviceID)
	}
	selectedOwner := ""
	if input.Target != nil {
		expected := input.Target.ExecutionScope
		expected.RequestID, expected.TurnID, expected.ExecutionID = scope.RequestID, scope.TurnID, scope.ExecutionID
		if expected != scope {
			return nil, coordination.ErrScopeExpired
		}
		selectedOwner = input.Target.OwnerID
	}
	if input.ID != "" {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(input.ID, "owned-chat-delete."))
		var address []string
		if !strings.HasPrefix(input.ID, "owned-chat-delete.") || err != nil || json.Unmarshal(raw, &address) != nil || len(address) != 2 || !strings.HasPrefix(address[1], "privacy-deletion/") {
			return nil, errors.New("删除回执编号无效")
		}
		selectedOwner = address[0]
	}
	result := map[string]any{"targets": []any{}, "checks": []any{}, "tombstones": 0, "completed": 0, "pending": 0, "failed": 0, "cleaned": 0}
	matchedOwner := false
	for _, owner := range owners {
		if selectedOwner != "" && selectedOwner != owner {
			continue
		}
		matchedOwner = true
		current := request
		current.RoleID, current.TargetDeviceID, current.ExpectedScope = scope.RoleID, scope.TargetDeviceID, &scope
		response, err := e.ManageChat(ctx, current, coordination.ChatManagementRequest{Operation: "privacy/deletion-" + operation, RequestID: id, ConversationOwnerID: owner, HistoricalRoleID: scope.RoleID, PrivacyDeletion: &input})
		if err != nil {
			if operation == "checks" && strings.Contains(err.Error(), "未找到当前数据持有方的删除回执") {
				continue
			}
			return nil, err
		}
		if operation == "request" || operation == "status" {
			return response, nil
		}
		for _, key := range []string{"targets", "checks"} {
			if value, ok := response[key]; ok {
				raw, _ := json.Marshal(value)
				var items []any
				if json.Unmarshal(raw, &items) != nil {
					return nil, coordination.ErrResourceVersion
				}
				result[key] = append(result[key].([]any), items...)
			}
		}
		for _, key := range []string{"tombstones", "completed", "pending", "failed", "cleaned"} {
			raw, _ := json.Marshal(response[key])
			var number int
			_ = json.Unmarshal(raw, &number)
			result[key] = result[key].(int) + number
		}
	}
	if selectedOwner != "" && !matchedOwner {
		return nil, coordination.ErrWrongOwner
	}
	if operation == "checks" && len(result["checks"].([]any)) == 0 {
		return nil, errors.New("没有可核验的真实删除回执")
	}
	return result, nil
}

func (e *Engine) PrivacyRequestRole(ctx context.Context, request Request) (Request, error) {
	if request.RoleID != "" {
		return request, nil
	}
	inner, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, "", uuid.NewString())
	if err != nil {
		return request, err
	}
	defer finish()
	roles, err := e.data.Roles(inner, scope)
	if err != nil {
		return request, err
	}
	if len(roles) == 0 {
		return request, coordination.ErrRoleRequired
	}
	request.RoleID = roles[0].ID
	return request, e.coordination.Validate(inner, scope)
}
