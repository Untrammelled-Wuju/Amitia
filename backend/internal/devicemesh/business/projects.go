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
	ID                 string             `json:"id"`
	Title              string             `json:"title"`
	Workspaces         []ProjectWorkspace `json:"workspaces"`
	PrimaryWorkspaceID string             `json:"primaryWorkspaceId"`
	CreatedAt          string             `json:"createdAt"`
	UpdatedAt          string             `json:"updatedAt"`
	PinnedAt           *string            `json:"pinnedAt,omitempty"`
	HiddenAt           *string            `json:"hiddenAt,omitempty"`
}

type ProjectWorkspace struct {
	ID       string `json:"id"`
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	RootURI  string `json:"rootUri"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

type CreateProjectInput struct {
	Title              string
	Workspaces         []ProjectWorkspace
	PrimaryWorkspaceID string
}

type ProjectResponse struct {
	Project         Project                      `json:"project"`
	Scope           coordination.ExecutionScope  `json:"executionScope"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
}

func (e *Engine) CreateProject(ctx context.Context, request Request, input CreateProjectInput) (ProjectResponse, error) {
	title := strings.TrimSpace(input.Title)
	workspaces, primaryWorkspaceID, err := normalizeProjectWorkspaces(input.Workspaces, input.PrimaryWorkspaceID)
	if title == "" || len(title) > 256 || !utf8.ValidString(title) || strings.ContainsAny(title, "\x00\r\n") || request.RequestID == "" || len(request.RequestID) > 128 || request.ExpectedScope == nil || err != nil {
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
	if err := coordination.ResolveMemoryScope(&scope, role, ""); err != nil {
		return ProjectResponse{}, err
	}
	if summaryAuthority(*request.ExpectedScope) != summaryAuthority(scope) {
		return ProjectResponse{}, coordination.ErrScopeExpired
	}
	if !projectWorkspacesBelongToDevice(workspaces, scope.TargetDeviceID) {
		return ProjectResponse{}, errors.New("项目工作区不属于当前执行设备")
	}
	ctx = coordination.WithScope(ctx, scope)
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return ProjectResponse{}, errors.New("数据来源不支持项目管理")
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00project\x00" + request.RequestID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope              coordination.ExecutionScope
		Title              string
		Workspaces         []ProjectWorkspace
		PrimaryWorkspaceID string
	}{summaryAuthority(scope), title, workspaces, primaryWorkspaceID}))
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
	existing, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{Management: true, ResourceKind: "project", Limit: 128})
	if err != nil {
		return ProjectResponse{}, err
	}
	requestedWorkspaces := make(map[string]bool, len(workspaces))
	for _, workspace := range workspaces {
		requestedWorkspaces[workspace.ID] = true
	}
	for _, row := range existing.Resources {
		if row.Kind != "project" {
			continue
		}
		var saved Project
		if json.Unmarshal(row.Body, &saved) != nil {
			continue
		}
		for _, workspace := range saved.Workspaces {
			if requestedWorkspaces[workspace.ID] {
				return ProjectResponse{}, errors.New("该文件夹已经属于一个项目")
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	response := ProjectResponse{Project: Project{ID: id, Title: title, Workspaces: workspaces, PrimaryWorkspaceID: primaryWorkspaceID, CreatedAt: now, UpdatedAt: now}, Scope: scope, Saved: true, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"project/" + id: 1, "checkpoint/" + receiptID: 1}}}
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

func normalizeProjectWorkspaces(items []ProjectWorkspace, primaryID string) ([]ProjectWorkspace, string, error) {
	if len(items) == 0 || len(items) > 32 {
		return nil, "", errors.New("项目工作区数量无效")
	}
	seen := make(map[string]bool, len(items))
	result := make([]ProjectWorkspace, 0, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.DeviceID = strings.TrimSpace(item.DeviceID)
		item.Name = strings.TrimSpace(item.Name)
		item.Kind = strings.TrimSpace(item.Kind)
		item.RootURI = strings.TrimSpace(item.RootURI)
		if item.ID == "" || len(item.ID) > 512 || item.DeviceID == "" || len(item.DeviceID) > 512 || item.Name == "" || len(item.Name) > 256 || !utf8.ValidString(item.Name) || strings.ContainsAny(item.ID+item.DeviceID+item.Name, "\x00\r\n") {
			return nil, "", errors.New("项目工作区参数无效")
		}
		if item.Kind != "local" && item.Kind != "saf" {
			return nil, "", errors.New("项目工作区类型无效")
		}
		if item.RootURI == "" || len(item.RootURI) > 4096 || !strings.HasPrefix(item.RootURI, "amitia://workspace/@"+item.ID+"/") {
			return nil, "", errors.New("项目工作区根地址无效")
		}
		if seen[item.ID] {
			return nil, "", errors.New("项目工作区不能重复")
		}
		seen[item.ID] = true
		result = append(result, item)
	}
	primaryID = strings.TrimSpace(primaryID)
	if primaryID == "" || !seen[primaryID] {
		return nil, "", errors.New("项目主工作区无效")
	}
	return result, primaryID, nil
}

func projectWorkspacesBelongToDevice(items []ProjectWorkspace, deviceID string) bool {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.DeviceID) != deviceID {
			return false
		}
	}
	return true
}
