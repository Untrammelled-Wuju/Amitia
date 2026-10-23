package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) ListContinuity(ctx context.Context, request Request, management bool) ([]OwnedContinuity, coordination.ExecutionScope, error) {
	if e == nil || e.data == nil || e.coordination == nil {
		return nil, coordination.ExecutionScope{}, errors.New("持续事项数据服务未就绪")
	}
	result := []OwnedContinuity{}
	cursor := ""
	seen := map[string]bool{}
	scope := coordination.ExecutionScope{}
	for {
		query := coordination.DataQuery{ResourceKind: "continuity", Cursor: cursor, Limit: 128, Management: management}
		page, err := e.Query(ctx, request, query)
		if err != nil {
			return nil, scope, err
		}
		if scope.CoreID == "" {
			scope = page.Scope
		} else if summaryAuthority(scope) != summaryAuthority(page.Scope) {
			return nil, scope, coordination.ErrScopeExpired
		}
		for _, resource := range page.Snapshot.Resources {
			if resource.Kind != "continuity" || resource.Deleted {
				continue
			}
			var document OwnedContinuity
			if json.Unmarshal(resource.Body, &document) != nil || document.Thread.ID != resource.ID || document.OwnerID != page.Snapshot.OwnerID {
				return nil, scope, coordination.ErrWrongOwner
			}
			if management {
				document = e.PresentManagedContinuity(ctx, document, page.Scope, resource.RoleID)
			}
			result = append(result, document)
		}
		cursor = page.Snapshot.NextCursors["continuity"]
		if cursor == "" {
			break
		}
		if seen[cursor] {
			return nil, scope, coordination.ErrRequestConflict
		}
		seen[cursor] = true
	}
	return result, scope, nil
}

func (e *Engine) ImportContinuity(ctx context.Context, request Request, document OwnedContinuity) error {
	if e == nil || e.data == nil || e.coordination == nil {
		return errors.New("持续事项数据服务未就绪")
	}
	document.Thread.ID = strings.TrimSpace(document.Thread.ID)
	document.Thread.Title = strings.TrimSpace(document.Thread.Title)
	if document.Thread.ID == "" || document.Thread.Title == "" {
		return errors.New("迁移持续事项缺少编号或标题")
	}
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return errors.New("持续事项数据端口未就绪")
	}
	existing, err := port.Resource(ctx, scope, "continuity", document.Thread.ID)
	if err != nil {
		return err
	}
	if existing != nil && !existing.Deleted {
		var current OwnedContinuity
		if json.Unmarshal(existing.Body, &current) != nil || current.Thread.ID != document.Thread.ID || current.OwnerID != scope.ResourceOwnerID {
			return coordination.ErrWrongOwner
		}
		return nil
	}
	now := time.Now().UTC()
	document.Thread.SpaceID = scope.SpaceID
	document.Thread.CharacterID = scope.RoleID
	if document.Thread.Status == "" {
		document.Thread.Status = continuity.ThreadStatusActive
	}
	if !continuity.ValidThreadStatus(document.Thread.Status) {
		return errors.New("迁移持续事项状态无效")
	}
	if document.Thread.Revision < 1 {
		document.Thread.Revision = 1
	}
	if document.Thread.CreatedAt.IsZero() {
		document.Thread.CreatedAt = now
	}
	if document.Thread.UpdatedAt.IsZero() {
		document.Thread.UpdatedAt = document.Thread.CreatedAt
	}
	if document.Thread.LastActiveAt.IsZero() {
		document.Thread.LastActiveAt = document.Thread.UpdatedAt
	}
	if document.Thread.Confidence <= 0 {
		document.Thread.Confidence = 1
	}
	for index := range document.Bindings {
		document.Bindings[index].ThreadID = document.Thread.ID
	}
	for index := range document.Waits {
		document.Waits[index].ThreadID = document.Thread.ID
	}
	for index := range document.Events {
		document.Events[index].ThreadID = document.Thread.ID
	}
	document.Scope = scope
	document.MemoryPartition = coordination.StorageRole(scope, "continuity")
	document.CoreID = scope.CoreID
	document.OwnerID = scope.ResourceOwnerID
	document.ProviderEpoch = scope.ProviderEpoch
	document.ModeRevision = scope.ModeRevision
	document.PermissionRevision = scope.PermissionRevision
	document.RoleRevision = scope.RoleRevision
	if err := e.coordination.TrackContinuity(ctx, scope, document.Thread.ID, document.Thread.Revision); err != nil {
		return err
	}
	_, err = e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "continuity", ID: document.Thread.ID, RoleID: coordination.StorageRole(scope, "continuity"), Body: body(document)}}})
	return err
}

func (e *Engine) ApplyContinuityRuntimeEvent(ctx context.Context, request Request, event ContinuityRuntimeEvent) ([]string, error) {
	if request.RoleID == "" {
		roles, err := e.continuityRoleIDs(ctx, request)
		if err != nil {
			return nil, err
		}
		resolved := []string{}
		for _, role := range roles {
			scoped := request
			scoped.RoleID = role
			items, err := e.ApplyContinuityRuntimeEvent(ctx, scoped, event)
			resolved = append(resolved, items...)
			if err != nil {
				return resolved, err
			}
		}
		return resolved, nil
	}
	documents, scope, err := e.ListContinuity(ctx, request, false)
	if err != nil {
		return nil, err
	}
	resolved := []string{}
	for _, document := range documents {
		if document.ReadOnly || document.Thread.Status.IsTerminal() || document.Thread.Status == continuity.ThreadStatusPaused {
			continue
		}
		if len(event.Bindings) > 0 && !continuity.BindingLookupMatches(document.Bindings, event.Bindings...) {
			continue
		}
		if len(event.Bindings) == 0 {
			continue
		}
		for _, wait := range document.Waits {
			if event.Signal == nil || wait.Status != continuity.WaitStatusWaiting || !continuity.WaitMatchesSignal(wait, *event.Signal) {
				continue
			}
			resolved = append(resolved, wait.ID)
		}
		requestID := request.RequestID + "/" + document.Thread.ID
		if len(requestID) > 128 {
			digest := sha256.Sum256([]byte(requestID))
			requestID = "continuity-runtime/" + hex.EncodeToString(digest[:16])
		}
		mutationRequest := request
		mutationRequest.RoleID = document.Thread.CharacterID
		mutationRequest.RequestID = requestID
		mutationRequest.ExpectedScope = &scope
		_, _, err := e.Continuity(ctx, mutationRequest, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "runtime_event", RuntimeEvent: &event})
		if err != nil {
			return resolved, err
		}
	}
	return resolved, nil
}

func (e *Engine) continuityRoleIDs(ctx context.Context, request Request) ([]string, error) {
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, "", request.RequestID+"/roles")
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
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		result = append(result, role.ID)
	}
	return result, nil
}
