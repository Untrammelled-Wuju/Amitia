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
		allowed = map[string]bool{"title": true, "archived": true, "pinned": true, "projectId": true, "lastReadTurnSequence": true}
	case "project":
		if edit.ExpectedScope == nil {
			return coordination.Acknowledgement{}, errors.New("项目修改缺少原数据归属，请重新加载")
		}
		allowed = map[string]bool{"title": true, "pinned": true}
	case "message":
		allowed = map[string]bool{"content": true}
	case "memory":
		allowed = map[string]bool{"content": true, "allowContextUse": true, "expiresAt": true, "archivedAt": true}
	case "summary":
		allowed = map[string]bool{"content": true}
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
		if key == "archived" || key == "pinned" || key == "allowContextUse" {
			var valueBool bool
			if json.Unmarshal(value, &valueBool) != nil {
				return coordination.Acknowledgement{}, errors.New("开关参数无效")
			}
		}
		if key == "projectId" {
			var projectID string
			if edit.ExpectedScope == nil || json.Unmarshal(value, &projectID) != nil || len(projectID) > 128 || strings.ContainsAny(projectID, "\x00\r\n") {
				return coordination.Acknowledgement{}, errors.New("项目归属参数无效，请重新加载")
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
	if edit.ExpectedScope != nil {
		expected := *edit.ExpectedScope
		expected.RequestID, expected.TurnID, expected.ExecutionID = scope.RequestID, scope.TurnID, scope.ExecutionID
		if expected != scope {
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
	var dependencies []coordination.ResourceVersion
	if rawProject, changed := edit.Changes["projectId"]; changed {
		var projectID string
		json.Unmarshal(rawProject, &projectID)
		if projectID != "" {
			project, err := port.Resource(ctx, scope, "project", projectID)
			if err != nil {
				return coordination.Acknowledgement{}, err
			}
			if project == nil || project.Deleted || project.OwnerID != scope.ResourceOwnerID || project.RoleID != scope.RoleID {
				return coordination.Acknowledgement{}, coordination.ErrWrongOwner
			}
			dependencies = append(dependencies, coordination.ResourceVersion{Kind: "project", ID: projectID, Revision: project.Revision})
		}
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(resource.Body, &document); err != nil || document == nil {
		return coordination.Acknowledgement{}, errors.New("数据格式不支持修改")
	}
	for key, value := range edit.Changes {
		document[key] = value
		if (edit.Kind == "conversation" || edit.Kind == "project") && (key == "archived" || key == "pinned") {
			var enabled bool
			json.Unmarshal(value, &enabled)
			var timestamp any
			if enabled {
				timestamp = time.Now().UTC().Format(time.RFC3339Nano)
			}
			document[key+"At"] = body(timestamp)
		}
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
	return e.commit(ctx, coordination.Commit{Scope: scope, Dependencies: dependencies, Mutations: []coordination.Mutation{{Kind: edit.Kind, ID: edit.ID, RoleID: scope.RoleID, SourceID: resource.SourceID, ExpectedRevision: resource.Revision, Deleted: edit.Deleted, Body: body(document)}, {Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: proof}}})
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
	if resource != nil && (resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != kind || resource.ID != id || resource.Revision < 1 || !json.Valid(resource.Body)) {
		return nil, scope, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return nil, scope, err
	}
	return resource, scope, nil
}
