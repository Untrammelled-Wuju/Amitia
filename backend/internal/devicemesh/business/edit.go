package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type EditRequest struct {
	ExpectedScope    *coordination.ExecutionScope `json:"expectedExecutionScope,omitempty"`
	TargetDeviceID   string                       `json:"targetDeviceId,omitempty"`
	RoleID           string                       `json:"characterId,omitempty"`
	RequestID        string                       `json:"requestId"`
	Kind             string                       `json:"kind"`
	ID               string                       `json:"id"`
	ExpectedRevision int64                        `json:"expectedRevision"`
	Deleted          bool                         `json:"deleted"`
	Clear            bool                         `json:"clear,omitempty"`
	Changes          map[string]json.RawMessage   `json:"changes,omitempty"`
}

func (e *Engine) Edit(ctx context.Context, authority Request, edit EditRequest) (coordination.Acknowledgement, error) {
	if edit.RequestID == "" || len(edit.RequestID) > 128 || edit.ID == "" || len(edit.ID) > 512 || edit.ExpectedRevision < 1 || len(body(edit.Changes)) > 256<<10 {
		return coordination.Acknowledgement{}, errors.New("数据修改参数无效")
	}
	allowed := map[string]bool{}
	switch edit.Kind {
	case "conversation":
		allowed = map[string]bool{"title": true, "archived": true, "pinned": true, "projectId": true, "lastReadTurnSequence": true, "modelConfigId": true, "reasoningEffort": true, "reasoningEnabled": true, "permissionMode": true, "messageStyle": true}
		if err := validateSettingsChanges(edit.Changes); err != nil {
			return coordination.Acknowledgement{}, err
		}
	case "project":
		if edit.ExpectedScope == nil {
			return coordination.Acknowledgement{}, errors.New("项目修改缺少原数据归属，请重新加载")
		}
		allowed = map[string]bool{"title": true, "pinned": true, "hidden": true, "workspaces": true, "primaryWorkspaceId": true}
	case "message":
		allowed = map[string]bool{"content": true}
	case "memory":
		allowed = map[string]bool{"content": true, "allowContextUse": true, "expiresAt": true, "archivedAt": true, "retentionLevel": true, "memoryStrength": true, "pinned": true}
	case "profile":
		allowed = map[string]bool{"content": true}
	case "episodic":
		allowed = map[string]bool{"content": true, "archivedAt": true}
	case "summary":
		allowed = map[string]bool{"content": true}
	case "emotion":
		if !edit.Deleted || len(edit.Changes) != 0 {
			return coordination.Acknowledgement{}, errors.New("情绪历史仅支持删除")
		}
	default:
		return coordination.Acknowledgement{}, errors.New("该类型由系统管理，请修改其原始数据")
	}
	for key, value := range edit.Changes {
		if !allowed[key] || !json.Valid(value) {
			return coordination.Acknowledgement{}, errors.New("禁止修改数据归属、角色、执行状态或系统字段")
		}
		if key == "title" || key == "content" && edit.Kind == "message" {
			var text string
			if json.Unmarshal(value, &text) != nil || len(text) > 128<<10 || strings.TrimSpace(text) == "" {
				return coordination.Acknowledgement{}, errors.New("内容不能为空或超过上限")
			}
			if edit.Kind == "project" && (len(text) > 256 || !utf8.ValidString(text) || strings.ContainsAny(text, "\x00\r\n")) {
				return coordination.Acknowledgement{}, errors.New("项目名称无效或超过上限")
			}
		}
		if key == "archived" || key == "pinned" || key == "hidden" || key == "allowContextUse" {
			var valueBool bool
			if json.Unmarshal(value, &valueBool) != nil {
				return coordination.Acknowledgement{}, errors.New("开关参数无效")
			}
		}
		if key == "workspaces" {
			var workspaces []ProjectWorkspace
			if json.Unmarshal(value, &workspaces) != nil {
				return coordination.Acknowledgement{}, errors.New("项目工作区参数无效")
			}
			if _, _, err := normalizeProjectWorkspaces(workspaces, workspacesPrimaryCandidate(workspaces)); err != nil {
				return coordination.Acknowledgement{}, err
			}
		}
		if key == "primaryWorkspaceId" {
			var primaryID string
			if json.Unmarshal(value, &primaryID) != nil || len(primaryID) > 512 || strings.ContainsAny(primaryID, "\x00\r\n") {
				return coordination.Acknowledgement{}, errors.New("项目主工作区参数无效")
			}
		}
		if key == "projectId" {
			var projectID string
			if json.Unmarshal(value, &projectID) != nil || len(projectID) > 512 || strings.ContainsAny(projectID, "\x00\r\n") {
				return coordination.Acknowledgement{}, errors.New("对话项目归属无效")
			}
		}
		if key == "expiresAt" || key == "archivedAt" {
			var text string
			if json.Unmarshal(value, &text) != nil {
				return coordination.Acknowledgement{}, errors.New("时间参数无效")
			}
			if text != "" {
				if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
					return coordination.Acknowledgement{}, errors.New("时间参数无效")
				}
			}
		}
		if key == "lastReadTurnSequence" {
			var sequence int64
			if json.Unmarshal(value, &sequence) != nil || sequence < 0 {
				return coordination.Acknowledgement{}, errors.New("已读位置无效")
			}
		}
	}
	if edit.Clear && (edit.Kind != "conversation" || edit.Deleted) {
		return coordination.Acknowledgement{}, errors.New("清空操作仅适用于现有对话")
	}
	if !edit.Deleted && !edit.Clear && len(edit.Changes) == 0 {
		return coordination.Acknowledgement{}, errors.New("没有需要修改的内容")
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, authority.SpaceID, authority.DeviceID, edit.TargetDeviceID, authority.CoreID, edit.RoleID, edit.RequestID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return coordination.Acknowledgement{}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	role, err := coordination.ResolveRole(edit.RoleID, roles)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	view := ""
	if edit.ExpectedScope != nil {
		if edit.ExpectedScope.MemoryRoleID == coordination.GlobalMemoryRole {
			view = "global"
		} else if edit.ExpectedScope.MemoryRoleID != "" {
			view = "role:" + edit.ExpectedScope.MemoryRoleID
		}
	}
	if err := coordination.ResolveMemoryScope(&scope, view); err != nil {
		return coordination.Acknowledgement{}, err
	}
	if edit.ExpectedScope != nil {
		expected, current := *edit.ExpectedScope, scope
		expected.RequestID, expected.TurnID, expected.ExecutionID = scope.RequestID, scope.TurnID, scope.ExecutionID
		expected.MemoryViewAll, current.MemoryViewAll = false, false
		if expected != current {
			return coordination.Acknowledgement{}, coordination.ErrScopeExpired
		}
	}
	ctx = coordination.WithScope(ctx, scope)
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.Acknowledgement{}, errors.New("数据来源不支持修改")
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.TargetDeviceID + "\x00edit\x00" + edit.RequestID)
	defer unlock()
	encoded := body(struct {
		Core   string
		Caller string
		Owner  string
		Role   string
		Edit   EditRequest
	}{scope.CoreID, scope.InitiatorDeviceID, scope.ResourceOwnerID, scope.RoleID, edit})
	digest := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "edit/" + edit.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return coordination.Acknowledgement{}, err
	} else if pending != nil {
		return e.commit(ctx, *pending)
	}
	receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	if receipt != nil {
		var saved struct {
			Hash            string                       `json:"hash"`
			RoleRevision    int64                        `json:"roleRevision"`
			Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
			return coordination.Acknowledgement{}, coordination.ErrRequestConflict
		}
		if saved.RoleRevision != scope.RoleRevision {
			return coordination.Acknowledgement{}, coordination.ErrScopeExpired
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return coordination.Acknowledgement{}, err
		}
		return saved.Acknowledgement, nil
	}
	resource, err := port.Resource(ctx, scope, edit.Kind, edit.ID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	if resource == nil || resource.Deleted || resource.Revision != edit.ExpectedRevision {
		return coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(resource.Body, &document); err != nil || document == nil {
		return coordination.Acknowledgement{}, errors.New("数据格式不支持修改")
	}
	if edit.Kind == "memory" {
		if err := editMemoryLifecycle(document, edit.Changes, time.Now()); err != nil {
			return coordination.Acknowledgement{}, err
		}
	}
	for key, value := range edit.Changes {
		if edit.Kind == "memory" && (key == "retentionLevel" || key == "memoryStrength" || key == "pinned") {
			continue
		}
		document[key] = value
		if (edit.Kind == "conversation" || edit.Kind == "project") && (key == "archived" || key == "pinned" || key == "hidden") {
			var enabled bool
			json.Unmarshal(value, &enabled)
			var timestamp any
			if enabled {
				timestamp = time.Now().UTC().Format(time.RFC3339Nano)
			}
			document[key+"At"] = body(timestamp)
		}
	}
	if edit.Kind == "conversation" {
		if raw, changed := edit.Changes["projectId"]; changed {
			var projectID string
			_ = json.Unmarshal(raw, &projectID)
			projectID = strings.TrimSpace(projectID)
			document["projectId"] = body(projectID)
			if projectID != "" {
				project, err := port.Resource(ctx, scope, "project", projectID)
				if err != nil {
					return coordination.Acknowledgement{}, err
				}
				if project == nil || project.Deleted || project.OwnerID != scope.ResourceOwnerID || project.RoleID != scope.RoleID {
					return coordination.Acknowledgement{}, coordination.ErrWrongOwner
				}
				var document Project
				if json.Unmarshal(project.Body, &document) != nil || document.ID != projectID {
					return coordination.Acknowledgement{}, errors.New("项目数据格式无效")
				}
				if document.HiddenAt != nil {
					return coordination.Acknowledgement{}, errors.New("该项目已从侧边栏移除")
				}
			}
		}
	}
	if edit.Kind == "project" {
		var project Project
		if encoded, err := json.Marshal(document); err != nil || json.Unmarshal(encoded, &project) != nil {
			return coordination.Acknowledgement{}, errors.New("项目数据格式无效")
		}
		workspaces, primaryID, err := normalizeProjectWorkspaces(project.Workspaces, project.PrimaryWorkspaceID)
		if err != nil || !projectWorkspacesBelongToDevice(workspaces, scope.TargetDeviceID) {
			return coordination.Acknowledgement{}, errors.New("项目工作区无效或不属于当前执行设备")
		}
		document["workspaces"] = body(workspaces)
		document["primaryWorkspaceId"] = body(primaryID)
	}
	if edit.Clear {
		document["clearRevision"] = body(resource.Revision + 1)
	}
	readStateOnly := len(edit.Changes) == 1 && edit.Changes["lastReadTurnSequence"] != nil && !edit.Deleted && !edit.Clear
	if !readStateOnly {
		document["updatedAt"] = body(time.Now().UTC().Format(time.RFC3339Nano))
	}
	ack := coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{edit.Kind + "/" + edit.ID: resource.Revision + 1, "checkpoint/" + receiptID: 1}}
	proof := body(map[string]any{"hash": fingerprint, "roleRevision": scope.RoleRevision, "acknowledgement": ack})
	return e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: edit.Kind, ID: edit.ID, RoleID: coordination.StorageRole(scope, edit.Kind), SourceID: resource.SourceID, ExpectedRevision: resource.Revision, Deleted: edit.Deleted, Body: body(document)}, {Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: proof}}})
}

func workspacesPrimaryCandidate(items []ProjectWorkspace) string {
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[0].ID)
}

func (e *Engine) ReadResource(ctx context.Context, authority Request, kind, id string) (*coordination.Resource, coordination.ExecutionScope, error) {
	ctx, scope, finish, err := e.coordination.Begin(ctx, authority.SpaceID, authority.DeviceID, authority.TargetDeviceID, authority.CoreID, authority.RoleID, authority.RequestID)
	if err != nil {
		return nil, scope, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return nil, scope, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return nil, scope, err
	}
	role, err := coordination.ResolveRole(authority.RoleID, roles)
	if err != nil {
		return nil, scope, err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	if err := coordination.ResolveMemoryScope(&scope, authority.MemoryView); err != nil {
		return nil, scope, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return nil, scope, errors.New("数据来源不支持记录查询")
	}
	ctx = coordination.WithScope(ctx, scope)
	resource, err := port.Resource(ctx, scope, kind, id)
	if err != nil {
		return nil, scope, err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return nil, scope, err
	}
	if resource != nil && (resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != coordination.StorageRole(scope, kind) || resource.Kind != kind || resource.ID != id || resource.Revision < 1 || !json.Valid(resource.Body)) {
		return nil, scope, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return nil, scope, err
	}
	return resource, scope, nil
}
