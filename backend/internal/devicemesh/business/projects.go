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

type Project struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	PinnedAt  *string `json:"pinnedAt,omitempty"`
}

type ProjectResponse struct {
	Project         Project                      `json:"project"`
	Scope           coordination.ExecutionScope  `json:"executionScope"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
}

func (e *Engine) CreateProject(ctx context.Context, request Request, title string) (ProjectResponse, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 256 || !utf8.ValidString(title) || strings.ContainsAny(title, "\x00\r\n") || request.RequestID == "" || len(request.RequestID) > 128 || request.ExpectedScope == nil {
		return ProjectResponse{}, errors.New("项目名称或原数据归属参数无效")
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return ProjectResponse{}, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return ProjectResponse{}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return ProjectResponse{}, err
	}
	role, err := coordination.ResolveRole(request.RoleID, roles)
	if err != nil {
		return ProjectResponse{}, err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	if summaryAuthority(*request.ExpectedScope) != summaryAuthority(scope) {
		return ProjectResponse{}, coordination.ErrScopeExpired
	}
	ctx = coordination.WithScope(ctx, scope)
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return ProjectResponse{}, errors.New("数据来源不支持项目管理")
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00project\x00" + request.RequestID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope coordination.ExecutionScope
		Title string
	}{summaryAuthority(scope), title}))
	fingerprint := hex.EncodeToString(digest[:])
	identity := sha256.Sum256([]byte(scope.CoreID + "\x00" + scope.InitiatorDeviceID + "\x00" + request.RequestID))
	id := "project-" + hex.EncodeToString(identity[:])
	receiptID := "project/" + request.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return ProjectResponse{}, err
	} else if pending != nil {
		if _, err := e.commit(ctx, *pending); err != nil {
			return ProjectResponse{}, err
		}
	}
	receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID)
	if err != nil {
		return ProjectResponse{}, err
	}
	if receipt != nil {
		var saved struct {
			Hash     string          `json:"hash"`
			Response ProjectResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint || saved.Response.Project.ID != id || summaryAuthority(saved.Response.Scope) != summaryAuthority(scope) || saved.Response.Acknowledgement.OwnerID != scope.ResourceOwnerID {
			return ProjectResponse{}, coordination.ErrRequestConflict
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return ProjectResponse{}, err
		}
		if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
			return ProjectResponse{}, err
		}
		return saved.Response, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	response := ProjectResponse{Project: Project{ID: id, Title: title, CreatedAt: now, UpdatedAt: now}, Scope: scope, Saved: true, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"project/" + id: 1, "checkpoint/" + receiptID: 1}}}
	commit := coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "project", ID: id, RoleID: scope.RoleID, Body: body(response.Project)}, {Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": fingerprint, "response": response})}}}
	ack, err := e.commit(ctx, commit)
	if err != nil {
		if !errors.Is(err, coordination.ErrResourceVersion) && !errors.Is(err, coordination.ErrRequestConflict) && !errors.Is(err, coordination.ErrScopeExpired) {
			if queueErr := e.coordination.Enqueue(ctx, commit); queueErr != nil {
				return ProjectResponse{}, errors.Join(err, queueErr)
			}
		}
		return ProjectResponse{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || ack.Versions["project/"+id] != 1 || ack.Versions["checkpoint/"+receiptID] != 1 {
		return ProjectResponse{}, errors.New("项目所有者尚未确认保存")
	}
	response.Acknowledgement = ack
	return response, nil
}
