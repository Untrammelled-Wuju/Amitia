package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/workspace"
)

const (
	maxCharacterSummaryLength = 500
	maxMessageContentLength   = 200
	maxMemoryValueLength      = 300
)

type kernelCharacterReader struct {
	engine  *business.Engine
	catalog coordination.RoleCatalogPort
	coreID  string
}

func newKernelCharacterReader() *kernelCharacterReader { return &kernelCharacterReader{} }
func (r *kernelCharacterReader) roleCatalog(ctx context.Context, includeDisabled bool) ([]coordination.Role, error) {
	host := kernel.GetHostAPIScope(ctx)
	request, err := ownedHostChatRequest(ctx, r.engine, r.coreID, host.ConversationID, host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	if r.catalog == nil {
		return nil, fmt.Errorf("Owner role catalog is not configured")
	}
	var result []coordination.Role
	err = r.engine.WithExecutionScope(ctx, *request.ExpectedScope, func(current context.Context) error {
		var err error
		result, err = r.catalog.ListOwnedRoleCatalog(current, *request.ExpectedScope, includeDisabled)
		return err
	})
	return result, err
}
func (r *kernelCharacterReader) ReadCharacter(ctx context.Context, characterID string) (json.RawMessage, bool, error) {
	host := kernel.GetHostAPIScope(ctx)
	if characterID == "" {
		characterID = host.CharacterID
	}
	if characterID == "" || characterID != host.CharacterID {
		return nil, false, nil
	}
	roles, err := r.roleCatalog(ctx, false)
	if err != nil {
		return nil, false, err
	}
	for _, role := range roles {
		if role.ID == characterID {
			summary := role.Description
			if utf8.RuneCountInString(summary) > maxCharacterSummaryLength {
				summary = truncateRunes(summary, maxCharacterSummaryLength)
			}
			raw, err := json.Marshal(map[string]any{"id": role.ID, "displayName": role.Name, "avatarRef": role.Avatar, "summary": summary, "revision": role.Revision})
			return raw, err == nil, err
		}
	}
	return nil, false, nil
}
func (r *kernelCharacterReader) ListCharacters(ctx context.Context, spaceID string, includeDisabled bool) ([]json.RawMessage, error) {
	host := kernel.GetHostAPIScope(ctx)
	if spaceID != "" && spaceID != host.SpaceID {
		return nil, coordination.ErrWrongOwner
	}
	roles, err := r.roleCatalog(ctx, includeDisabled)
	if err != nil {
		return nil, err
	}
	items := make([]json.RawMessage, 0, len(roles))
	for _, role := range roles {
		raw, err := json.Marshal(map[string]any{"id": role.ID, "displayName": role.Name, "avatarRef": role.Avatar, "enabled": !role.Disabled})
		if err != nil {
			return nil, err
		}
		items = append(items, raw)
	}
	return items, nil
}

type kernelConversationReader struct {
	engine *business.Engine
	coreID string
}

func newKernelConversationReader() *kernelConversationReader { return &kernelConversationReader{} }
func ownedHostScopeRequest(ctx context.Context, engine *business.Engine, coreID, roleID, requestID string) (business.Request, error) {
	if engine == nil {
		return business.Request{}, fmt.Errorf("Owner chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if host.SpaceID == "" || host.CharacterID == "" || roleID != "" && roleID != host.CharacterID {
		return business.Request{}, coordination.ErrWrongOwner
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	request := business.Request{SpaceID: host.SpaceID, CoreID: coreID, RoleID: host.CharacterID, RequestID: requestID}
	if scope, ok := coordination.FromContext(ctx); ok {
		if scope.SpaceID != host.SpaceID || scope.RoleID != host.CharacterID {
			return business.Request{}, coordination.ErrWrongOwner
		}
		request.DeviceID, request.TargetDeviceID, request.CoreID = scope.InitiatorDeviceID, scope.TargetDeviceID, scope.CoreID
		scope.RequestID = requestID
		scope.TurnID, scope.ExecutionID = "", ""
		request.ExpectedScope = &scope
		return request, nil
	}
	actor, err := auth.RequireActor(ctx)
	if err != nil {
		return business.Request{}, err
	}
	if actor.SpaceID.String() != host.SpaceID || actor.DeviceID == "" {
		return business.Request{}, coordination.ErrWrongOwner
	}
	request.DeviceID, request.TargetDeviceID = actor.DeviceID.String(), actor.DeviceID.String()
	request.ResourceOwnerID, request.RoleOwnerID = request.TargetDeviceID, coreID
	return request, nil
}

func ownedHostChatRequest(ctx context.Context, engine *business.Engine, coreID, conversationID, roleID, requestID string) (business.Request, error) {
	host := kernel.GetHostAPIScope(ctx)
	if conversationID != "" && host.ConversationID != "" && host.ConversationID != conversationID {
		return business.Request{}, coordination.ErrWrongOwner
	}
	request, err := ownedHostScopeRequest(ctx, engine, coreID, roleID, requestID)
	if err != nil || request.ExpectedScope != nil {
		request.ConversationID = conversationID
		return request, err
	}
	request.ConversationID = conversationID
	view, err := engine.Query(ctx, request, coordination.DataQuery{ConversationID: conversationID, Management: true, ResourceKind: "conversation", Limit: 1})
	if err != nil {
		return business.Request{}, err
	}
	scope := view.Scope
	scope.RequestID = request.RequestID
	scope.TurnID, scope.ExecutionID = "", ""
	request.ExpectedScope = &scope
	return request, nil
}
func (r *kernelConversationReader) ReadConversation(ctx context.Context, conversationID string, limit, offset int) ([]json.RawMessage, bool, error) {
	host := kernel.GetHostAPIScope(ctx)
	if conversationID == "" {
		conversationID = host.ConversationID
	}
	if conversationID == "" {
		return []json.RawMessage{}, false, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 || offset > 1000000 {
		return nil, false, coordination.ErrPendingLimit
	}
	request, err := ownedHostChatRequest(ctx, r.engine, r.coreID, conversationID, host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, false, err
	}
	page := offset/limit + 1
	skip := offset % limit
	items := []json.RawMessage{}
	total := 0
	for len(items) < limit {
		result, err := r.engine.ManageChat(ctx, request, coordination.ChatManagementRequest{Operation: "search", ConversationOwnerID: request.ExpectedScope.ResourceOwnerID, ConversationID: conversationID, Page: page, PageSize: limit})
		if err != nil {
			return nil, false, err
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, false, err
		}
		var batch struct {
			Items []struct {
				ID             string                        `json:"id"`
				ConversationID string                        `json:"conversationId"`
				Role           string                        `json:"role"`
				RequestID      string                        `json:"requestId"`
				CreatedAt      string                        `json:"createdAt"`
				Content        string                        `json:"content"`
				Attachments    []conversationAttachmentInput `json:"attachments"`
			} `json:"items"`
			Total int `json:"total"`
		}
		if err = json.Unmarshal(raw, &batch); err != nil {
			return nil, false, err
		}
		total = batch.Total
		for i, msg := range batch.Items {
			if i < skip {
				continue
			}
			content := msg.Content
			if utf8.RuneCountInString(content) > maxMessageContentLength {
				content = truncateRunes(content, maxMessageContentLength)
			}
			attachments, err := materializeConversationAttachments(msg.Attachments)
			if err != nil {
				return nil, false, err
			}
			entry := map[string]any{"conversationId": msg.ConversationID, "role": msg.Role, "timestamp": msg.CreatedAt, "contentSummary": content}
			if msg.ID != "" {
				entry["id"] = msg.ID
			}
			if requestID := strings.TrimSpace(msg.RequestID); requestID != "" {
				entry["requestId"] = requestID
			}
			if len(attachments) > 0 {
				entry["attachments"] = attachments
			}
			encoded, err := json.Marshal(entry)
			if err != nil {
				return nil, false, err
			}
			items = append(items, encoded)
			if len(items) == limit {
				break
			}
		}
		if len(batch.Items) < limit || page*limit >= total {
			break
		}
		page++
		skip = 0
	}
	return items, offset+len(items) < total, nil
}

func (r *kernelConversationReader) ReadConversationTurns(ctx context.Context, conversationID string, limit int) ([]json.RawMessage, error) {
	host := kernel.GetHostAPIScope(ctx)
	if conversationID == "" {
		conversationID = host.ConversationID
	}
	if conversationID == "" {
		return []json.RawMessage{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	request, err := ownedHostChatRequest(ctx, r.engine, r.coreID, conversationID, host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	result, err := r.engine.Query(ctx, request, coordination.DataQuery{
		Management:     true,
		RuntimeTurns:   true,
		ConversationID: conversationID,
		ResourceKind:   "checkpoint",
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	type turnRecord struct {
		sequence int64
		encoded  json.RawMessage
	}
	records := make([]turnRecord, 0, len(result.Snapshot.Resources))
	for _, resource := range result.Snapshot.Resources {
		if resource.Deleted {
			continue
		}
		var saved struct {
			ConversationID string                              `json:"conversationId"`
			Status         string                              `json:"status"`
			Runtime        *conversationstream.ActiveTurnState `json:"runtime"`
		}
		if err := json.Unmarshal(resource.Body, &saved); err != nil {
			return nil, fmt.Errorf("decode conversation turn %s: %w", resource.ID, err)
		}
		if saved.Runtime == nil || saved.ConversationID != conversationID {
			continue
		}
		blocks := make([]map[string]any, 0, len(saved.Runtime.Blocks))
		for _, block := range saved.Runtime.Blocks {
			entry := map[string]any{
				"blockId":       block.BlockID,
				"blockSequence": block.BlockSequence,
				"type":          block.Type,
				"status":        block.Status,
			}
			if block.CallID != "" {
				entry["callId"] = block.CallID
			}
			if block.Revision > 0 {
				entry["revision"] = block.Revision
			}
			if block.Content != "" {
				entry["content"] = block.Content
			}
			if block.StartedAt != "" {
				entry["startedAt"] = block.StartedAt
			}
			if len(block.Payload) > 0 {
				entry["payload"] = block.Payload
			}
			blocks = append(blocks, entry)
		}
		turn := map[string]any{
			"turnId":       saved.Runtime.TurnID,
			"turnSequence": saved.Runtime.TurnSequence,
			"status":       saved.Runtime.Status,
			"blocks":       blocks,
		}
		if saved.Status != "" {
			turn["checkpointStatus"] = saved.Status
		}
		if saved.Runtime.RequestID != "" {
			turn["requestId"] = saved.Runtime.RequestID
		}
		if saved.Runtime.StartedAt != "" {
			turn["startedAt"] = saved.Runtime.StartedAt
		}
		if saved.Runtime.LastModifiedAt != "" {
			turn["lastModifiedAt"] = saved.Runtime.LastModifiedAt
		}
		encoded, err := json.Marshal(turn)
		if err != nil {
			return nil, err
		}
		records = append(records, turnRecord{sequence: saved.Runtime.TurnSequence, encoded: encoded})
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].sequence < records[j].sequence })
	turns := make([]json.RawMessage, 0, len(records))
	for _, record := range records {
		turns = append(turns, record.encoded)
	}
	return turns, nil
}

func (r *kernelConversationReader) SearchConversations(ctx context.Context, query string, limit int) ([]json.RawMessage, bool, error) {
	if r.engine == nil {
		return nil, false, fmt.Errorf("Owner chat service is not configured")
	}
	query, err := coordination.NormalizeConversationSearch(query)
	if err != nil {
		return nil, false, err
	}
	host := kernel.GetHostAPIScope(ctx)
	request, err := ownedHostChatRequest(ctx, r.engine, r.coreID, "", host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, false, err
	}
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	result, err := r.engine.Query(ctx, request, coordination.DataQuery{
		ListConversations: true,
		SearchQuery:       query,
		Limit:             limit,
	})
	if err != nil {
		return nil, false, err
	}
	items := make([]json.RawMessage, 0, len(result.Snapshot.Resources))
	for _, resource := range result.Snapshot.Resources {
		if resource.Kind != "conversation" || resource.Deleted {
			continue
		}
		var row struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Name  string `json:"name"`
		}
		if err := json.Unmarshal(resource.Body, &row); err != nil {
			return nil, false, fmt.Errorf("decode searched conversation %s: %w", resource.ID, err)
		}
		sessionID := strings.TrimSpace(row.ID)
		if sessionID == "" {
			sessionID = strings.TrimSpace(resource.ID)
		}
		if sessionID == "" {
			continue
		}
		snippet := strings.TrimSpace(row.Title)
		if snippet == "" {
			snippet = strings.TrimSpace(row.Name)
		}
		if snippet == "" {
			snippet = sessionID
		}
		raw, err := json.Marshal(map[string]any{
			"sessionId": sessionID,
			"snippet":   snippet,
		})
		if err != nil {
			return nil, false, err
		}
		items = append(items, raw)
	}
	return items, strings.TrimSpace(result.Snapshot.NextCursors["conversation"]) != "", nil
}

type conversationCreatorAdapter struct {
	engine           *business.Engine
	coreID           string
	workspaceService *workspace.Service
}

func newConversationCreatorAdapter() *conversationCreatorAdapter {
	return &conversationCreatorAdapter{}
}

func conversationCatalogTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func ownedChatDocumentString(document map[string]any, key string) string {
	value, _ := document[key].(string)
	return strings.TrimSpace(value)
}

func (a *conversationCreatorAdapter) ListConversations(ctx context.Context, limit int, offset int, archivedOnly bool) (any, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if host.SpaceID == "" || host.CharacterID == "" {
		return nil, coordination.ErrWrongOwner
	}
	request, err := ownedHostChatRequest(ctx, a.engine, a.coreID, "", host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	result, err := a.engine.ManageChat(ctx, request, coordination.ChatManagementRequest{
		Operation:           "conversations",
		ConversationOwnerID: request.ExpectedScope.ResourceOwnerID,
		Page:                1,
		PageSize:            128,
		ArchivedOnly:        archivedOnly,
	})
	if err != nil {
		return nil, err
	}
	rawItems, ok := result["items"].([]map[string]any)
	if !ok {
		rawItems = []map[string]any{}
	}
	if offset > len(rawItems) {
		offset = len(rawItems)
	}
	end := offset + limit
	if end > len(rawItems) {
		end = len(rawItems)
	}
	selected := rawItems[offset:end]

	localRoots := map[string]string{}
	if a.workspaceService != nil {
		mounts, listErr := a.workspaceService.ListMounts(ctx)
		if listErr != nil {
			return nil, listErr
		}
		for _, mount := range mounts {
			if mount.Kind == workspace.WorkspaceKindLocal && strings.TrimSpace(mount.LocalRoot) != "" {
				localRoots[string(mount.ID)] = mount.LocalRoot
			}
		}
	}

	type catalogWorkspace struct {
		WorkspaceID string   `json:"workspaceId"`
		ProjectID   string   `json:"projectId"`
		Path        string   `json:"path"`
		Title       string   `json:"title"`
		CreatedAt   string   `json:"createdAt"`
		UpdatedAt   string   `json:"updatedAt"`
		SessionIDs  []string `json:"sessionIds"`
	}
	type catalogSession struct {
		SessionID      string `json:"sessionId"`
		Title          string `json:"title"`
		DisplayTitle   string `json:"displayTitle"`
		UpdatedAt      int64  `json:"updatedAt"`
		Running        bool   `json:"running"`
		Blank          bool   `json:"blank"`
		ProjectID      string `json:"projectId"`
		WorkspaceID    string `json:"workspaceId"`
		RootURI        string `json:"rootUri"`
		CWD            string `json:"cwd,omitempty"`
		WorkspaceError string `json:"workspaceError,omitempty"`
		ArchivedAt     string `json:"archivedAt,omitempty"`
		PinnedAt       string `json:"pinnedAt,omitempty"`
		CreatedAt      string `json:"createdAt"`
		MessageCount   int    `json:"messageCount"`
		Order          int64  `json:"order"`
		FormatStatus   string `json:"formatStatus"`
		AgentAvailable bool   `json:"agentAvailable"`
		ParentSession  string `json:"parentSession,omitempty"`
	}
	sessions := make([]catalogSession, 0, len(selected))
	workspacesByID := map[string]*catalogWorkspace{}
	archived := []string{}
	pinned := []string{}
	for _, body := range selected {
		id := ownedChatDocumentString(body, "id")
		if id == "" {
			id = ownedChatDocumentString(body, "conversationId")
		}
		if id == "" {
			continue
		}
		resolved, resolveErr := a.ResolveConversationWorkspace(ctx, id)
		projectID := resolved.ProjectID
		workspaceID := resolved.WorkspaceID
		rootURI := resolved.RootURI
		workspaceError := ""
		if resolveErr != nil {
			workspaceError = resolveErr.Error()
			projectID = ownedChatDocumentString(body, "projectId")
			workspaceID = ownedChatDocumentString(body, "workspaceId")
		}
		localPath := ""
		if workspaceID != "" {
			localPath = localRoots[workspaceID]
		}
		workspaceKey := workspaceID
		if projectID != "" && workspaceID != "" {
			workspaceKey = projectID + ":" + workspaceID
		}
		title := ownedChatDocumentString(body, "title")
		if title == "" {
			title = "新对话"
		}
		createdAt := ownedChatDocumentString(body, "createdAt")
		updatedAtText := ownedChatDocumentString(body, "updatedAt")
		updatedAt := conversationCatalogTime(updatedAtText)
		if updatedAt.IsZero() {
			updatedAt = conversationCatalogTime(createdAt)
		}
		messageCount := 0
		if value, ok := body["messageCount"].(float64); ok {
			messageCount = int(value)
		}
		if messageCount == 0 {
			counted, countErr := a.countConversationMessages(ctx, id, ownedChatDocumentString(body, "characterId"))
			if countErr != nil {
				return nil, countErr
			}
			messageCount = counted
		}
		running := body["isGenerating"] == true
		archivedAt := ownedChatDocumentString(body, "archivedAt")
		pinnedAt := ownedChatDocumentString(body, "pinnedAt")
		orderValue, _ := body["order"].(float64)
		parentSession := ""
		if origin, ok := body["forkOrigin"].(map[string]any); ok {
			parentSession = ownedChatDocumentString(origin, "conversationId")
		}
		if archivedAt != "" {
			archived = append(archived, id)
		}
		if pinnedAt != "" {
			pinned = append(pinned, id)
		}
		session := catalogSession{
			SessionID:      id,
			Title:          title,
			DisplayTitle:   title,
			UpdatedAt:      updatedAt.UnixMilli(),
			Running:        running,
			Blank:          messageCount == 0,
			ProjectID:      projectID,
			WorkspaceID:    workspaceKey,
			RootURI:        rootURI,
			CWD:            localPath,
			WorkspaceError: workspaceError,
			ArchivedAt:     archivedAt,
			PinnedAt:       pinnedAt,
			CreatedAt:      createdAt,
			MessageCount:   messageCount,
			Order:          int64(orderValue),
			FormatStatus:   "current",
			AgentAvailable: running,
			ParentSession:  parentSession,
		}
		sessions = append(sessions, session)
		if workspaceKey != "" {
			view := workspacesByID[workspaceKey]
			if view == nil {
				pathValue := localPath
				if pathValue == "" {
					pathValue = rootURI
				}
				view = &catalogWorkspace{
					WorkspaceID: workspaceKey,
					ProjectID:   projectID,
					Path:        pathValue,
					Title:       title,
					CreatedAt:   createdAt,
					UpdatedAt:   updatedAtText,
					SessionIDs:  []string{},
				}
				workspacesByID[workspaceKey] = view
			}
			view.SessionIDs = append(view.SessionIDs, id)
		}
	}
	workspaces := make([]catalogWorkspace, 0, len(workspacesByID))
	for _, view := range workspacesByID {
		workspaces = append(workspaces, *view)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].WorkspaceID < workspaces[j].WorkspaceID
	})
	return map[string]any{
		"items":              sessions,
		"workspaces":         workspaces,
		"archivedSessionIds": archived,
		"pinnedSessionIds":   pinned,
		"hasMore":            end < len(rawItems),
		"total":              len(rawItems),
		"offset":             offset,
		"limit":              limit,
	}, nil
}

func (a *conversationCreatorAdapter) CreateConversation(ctx context.Context, request kernel.ConversationCreateRequest) (json.RawMessage, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if request.SpaceID != host.SpaceID || request.CharacterID != host.CharacterID {
		return nil, coordination.ErrWrongOwner
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, "", request.CharacterID, request.RequestID)
	if err != nil {
		return nil, err
	}
	result, err := a.engine.ManageChat(ctx, ownedRequest, coordination.ChatManagementRequest{
		Operation:           "conversation/create",
		ConversationOwnerID: ownedRequest.ExpectedScope.ResourceOwnerID,
		ConversationID:      request.ConversationID,
		RequestID:           request.RequestID,
		Title:               request.Title,
		ProjectID:           request.ProjectID,
		Channel:             request.Channel,
		PeerID:              request.PeerID,
		Draft:               request.Draft,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *conversationCreatorAdapter) ResolveConversationWorkspace(ctx context.Context, conversationID string) (kernel.ConversationWorkspace, error) {
	if a == nil || a.engine == nil {
		return kernel.ConversationWorkspace{}, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, "", host.CharacterID, uuid.NewString())
	if err != nil {
		return kernel.ConversationWorkspace{}, err
	}
	resolved, err := a.engine.ResolveConversationWorkspace(ctx, ownedRequest, conversationID)
	if err != nil {
		return kernel.ConversationWorkspace{}, err
	}
	return kernel.ConversationWorkspace{
		ProjectID:   resolved.ProjectID,
		WorkspaceID: resolved.WorkspaceID,
		RootURI:     resolved.RootURI,
		ReadOnly:    resolved.ReadOnly,
	}, nil
}

func (a *conversationCreatorAdapter) DeleteConversation(ctx context.Context, request kernel.ConversationDeleteRequest) (json.RawMessage, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if request.SpaceID != host.SpaceID || request.CharacterID != host.CharacterID || request.ConversationID == "" || request.ConversationID != host.ConversationID {
		return nil, coordination.ErrWrongOwner
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, request.CharacterID, request.RequestID)
	if err != nil {
		return nil, err
	}
	result, err := a.engine.ManageChat(ctx, ownedRequest, coordination.ChatManagementRequest{
		Operation:           "conversation/delete",
		ConversationID:      request.ConversationID,
		ConversationOwnerID: ownedRequest.ExpectedScope.ResourceOwnerID,
		RequestID:           request.RequestID,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *conversationCreatorAdapter) ArchiveConversation(ctx context.Context, request kernel.ConversationArchiveRequest) (json.RawMessage, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if request.SpaceID != host.SpaceID || request.CharacterID != host.CharacterID || request.ConversationID == "" || request.ConversationID != host.ConversationID {
		return nil, coordination.ErrWrongOwner
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, request.CharacterID, request.RequestID)
	if err != nil {
		return nil, err
	}
	archived := request.Archived
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	result, err := a.engine.ManageChat(ctx, ownedRequest, coordination.ChatManagementRequest{
		Operation:           "conversation/archive",
		ConversationID:      request.ConversationID,
		ConversationOwnerID: ownedRequest.ExpectedScope.ResourceOwnerID,
		RequestID:           requestID,
		Archived:            &archived,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *conversationCreatorAdapter) ReorderConversation(ctx context.Context, request kernel.ConversationReorderRequest) (json.RawMessage, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if request.ConversationID == "" || request.ConversationID != host.ConversationID {
		return nil, coordination.ErrWrongOwner
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, host.CharacterID, requestID)
	if err != nil {
		return nil, err
	}
	result, err := a.engine.ManageChat(ctx, ownedRequest, coordination.ChatManagementRequest{
		Operation:            "conversation/reorder",
		ConversationID:       request.ConversationID,
		BeforeConversationID: request.BeforeConversationID,
		ProjectID:            request.ProjectID,
		ConversationOwnerID:  ownedRequest.ExpectedScope.ResourceOwnerID,
		RequestID:            requestID,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *conversationCreatorAdapter) ForkConversation(ctx context.Context, request kernel.ConversationForkRequest) (json.RawMessage, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	scope := kernel.GetHostAPIScope(ctx)
	if request.ConversationID == "" {
		return nil, coordination.ErrWrongOwner
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, request.CharacterID, requestID)
	if err != nil {
		return nil, err
	}
	_ = scope
	result, err := a.engine.ManageChat(ctx, ownedRequest, coordination.ChatManagementRequest{
		Operation:           "conversation/fork",
		ConversationID:      request.ConversationID,
		ConversationOwnerID: ownedRequest.ExpectedScope.ResourceOwnerID,
		RequestID:           requestID,
		Title:               request.Title,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *conversationCreatorAdapter) PinConversation(ctx context.Context, request kernel.ConversationPinRequest) (json.RawMessage, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if request.SpaceID != host.SpaceID || request.CharacterID != host.CharacterID || request.ConversationID == "" || request.ConversationID != host.ConversationID {
		return nil, coordination.ErrWrongOwner
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, request.CharacterID, request.RequestID)
	if err != nil {
		return nil, err
	}
	pinned := request.Pinned
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	result, err := a.engine.ManageChat(ctx, ownedRequest, coordination.ChatManagementRequest{
		Operation:           "conversation/pin",
		ConversationID:      request.ConversationID,
		ConversationOwnerID: ownedRequest.ExpectedScope.ResourceOwnerID,
		RequestID:           requestID,
		Pinned:              &pinned,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

type conversationTurnControllerAdapter struct {
	engine *business.Engine
	coreID string
}

func newConversationTurnControllerAdapter() *conversationTurnControllerAdapter {
	return &conversationTurnControllerAdapter{}
}

func (a *conversationTurnControllerAdapter) resolveConversationTurnView(ctx context.Context, request kernel.ConversationTurnInterruptRequest) (business.QueryResult, error) {
	if a == nil || a.engine == nil {
		return business.QueryResult{}, fmt.Errorf("chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	spaceID := strings.TrimSpace(request.SpaceID)
	characterID := strings.TrimSpace(request.CharacterID)
	conversationID := strings.TrimSpace(request.ConversationID)
	if spaceID == "" {
		spaceID = host.SpaceID
	}
	if characterID == "" {
		characterID = host.CharacterID
	}
	if conversationID == "" {
		conversationID = host.ConversationID
	}
	if spaceID == "" || characterID == "" || conversationID == "" ||
		spaceID != host.SpaceID ||
		characterID != host.CharacterID ||
		host.ConversationID != "" && conversationID != host.ConversationID {
		return business.QueryResult{}, coordination.ErrWrongOwner
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, conversationID, characterID, uuid.NewString())
	if err != nil {
		return business.QueryResult{}, err
	}
	view, err := a.engine.Query(ctx, ownedRequest, coordination.DataQuery{ConversationID: conversationID, Management: true, RuntimeTurns: true, Limit: 1})
	if err != nil {
		return business.QueryResult{}, err
	}
	if view.ConversationID != conversationID {
		return business.QueryResult{}, coordination.ErrWrongOwner
	}
	return view, nil
}

func (a *conversationTurnControllerAdapter) InterruptConversationTurn(ctx context.Context, request kernel.ConversationTurnInterruptRequest) (kernel.ConversationTurnInterruptResult, error) {
	view, err := a.resolveConversationTurnView(ctx, request)
	if err != nil {
		return kernel.ConversationTurnInterruptResult{}, err
	}
	runtimeKey := business.RuntimeKey(view.Scope, view.ConversationID)
	manager := business.RuntimeManager()
	requestedTurnID := strings.TrimSpace(request.TurnID)
	var result kernel.ConversationTurnInterruptResult
	err = a.engine.WithConversationExecutionScope(ctx, view.Scope, view.ConversationID, func(current context.Context) error {
		var err error
		result, err = interruptOwnedRuntimeTurn(current, manager, runtimeKey, view.ConversationID, requestedTurnID)
		return err
	})
	if err != nil {
		return kernel.ConversationTurnInterruptResult{}, err
	}
	return result, nil
}

func (a *conversationTurnControllerAdapter) SteerConversationTurn(ctx context.Context, request kernel.ConversationTurnSteerRequest) (kernel.ConversationTurnSteerResult, error) {
	view, err := a.resolveConversationTurnView(ctx, kernel.ConversationTurnInterruptRequest{
		ConversationID: request.ConversationID,
		TurnID:         request.TurnID,
	})
	if err != nil {
		return kernel.ConversationTurnSteerResult{}, err
	}
	runtimeKey := business.RuntimeKey(view.Scope, view.ConversationID)
	manager := business.RuntimeManager()
	if manager == nil {
		return kernel.ConversationTurnSteerResult{}, fmt.Errorf("conversation runtime manager is not configured")
	}
	turnID := strings.TrimSpace(request.TurnID)
	if turnID == "" {
		snapshot := manager.RuntimeSnapshot(runtimeKey)
		if snapshot.ActiveTurn != nil {
			turnID = snapshot.ActiveTurn.TurnID
		}
	}
	if turnID == "" {
		return kernel.ConversationTurnSteerResult{}, fmt.Errorf("当前会话没有可转向的轮次")
	}
	if !manager.Steer(runtimeKey, turnID, request.Input) {
		return kernel.ConversationTurnSteerResult{}, fmt.Errorf("会话轮次 %s 已结束或不可转向", turnID)
	}
	return kernel.ConversationTurnSteerResult{Accepted: true, TurnID: turnID}, nil
}

func (a *conversationTurnControllerAdapter) AwaitConversationTurnIdle(ctx context.Context, request kernel.ConversationTurnInterruptRequest) (kernel.ConversationTurnAwaitResult, error) {
	view, err := a.resolveConversationTurnView(ctx, request)
	if err != nil {
		return kernel.ConversationTurnAwaitResult{}, err
	}
	runtimeKey := business.RuntimeKey(view.Scope, view.ConversationID)
	manager := business.RuntimeManager()
	var result kernel.ConversationTurnAwaitResult
	err = a.engine.WithConversationExecutionScope(ctx, view.Scope, view.ConversationID, func(current context.Context) error {
		var err error
		result, err = awaitOwnedRuntimeTurnIdle(current, manager, runtimeKey, view.ConversationID, request.TurnID)
		return err
	})
	if err != nil {
		return kernel.ConversationTurnAwaitResult{}, err
	}
	return result, nil
}

func interruptOwnedRuntimeTurn(ctx context.Context, manager *conversationstream.Manager, runtimeKey, conversationID, requestedTurnID string) (kernel.ConversationTurnInterruptResult, error) {
	if manager == nil {
		return kernel.ConversationTurnInterruptResult{}, fmt.Errorf("conversation runtime manager is not configured")
	}
	if strings.TrimSpace(runtimeKey) == "" {
		return kernel.ConversationTurnInterruptResult{}, coordination.ErrWrongOwner
	}
	snapshot, err := manager.RuntimeSnapshotCurrent(ctx, runtimeKey)
	if err != nil {
		return kernel.ConversationTurnInterruptResult{}, err
	}
	if snapshot.ActiveTurn == nil {
		return kernel.ConversationTurnInterruptResult{}, fmt.Errorf("%w: conversation %s has no active turn", kernel.ErrConversationTurnUnavailable, conversationID)
	}
	active := snapshot.ActiveTurn
	requestedTurnID = strings.TrimSpace(requestedTurnID)
	if requestedTurnID != "" && requestedTurnID != active.TurnID {
		return kernel.ConversationTurnInterruptResult{}, fmt.Errorf("%w: turn %s is not the active turn %s", kernel.ErrConversationTurnUnavailable, requestedTurnID, active.TurnID)
	}
	if !manager.Interrupt(runtimeKey, active.TurnID) {
		return kernel.ConversationTurnInterruptResult{}, fmt.Errorf("%w: turn %s is no longer interruptible", kernel.ErrConversationTurnUnavailable, active.TurnID)
	}
	return kernel.ConversationTurnInterruptResult{
		Interrupted: true,
		TurnID:      active.TurnID,
		RequestID:   active.RequestID,
		ExecutionID: active.ExecutionID,
	}, nil
}

func awaitOwnedRuntimeTurnIdle(ctx context.Context, manager *conversationstream.Manager, runtimeKey, conversationID, requestedTurnID string) (kernel.ConversationTurnAwaitResult, error) {
	if manager == nil {
		return kernel.ConversationTurnAwaitResult{}, fmt.Errorf("conversation runtime manager is not configured")
	}
	if strings.TrimSpace(runtimeKey) == "" {
		return kernel.ConversationTurnAwaitResult{}, coordination.ErrWrongOwner
	}
	requestedTurnID = strings.TrimSpace(requestedTurnID)
	for {
		snapshot, err := manager.RuntimeSnapshotCurrent(ctx, runtimeKey)
		if err != nil {
			return kernel.ConversationTurnAwaitResult{}, err
		}
		if snapshot.ActiveTurn == nil {
			return kernel.ConversationTurnAwaitResult{Idle: true}, nil
		}
		active := snapshot.ActiveTurn
		if requestedTurnID != "" && requestedTurnID != active.TurnID {
			return kernel.ConversationTurnAwaitResult{}, fmt.Errorf("%w: turn %s is not the active turn %s", kernel.ErrConversationTurnUnavailable, requestedTurnID, active.TurnID)
		}
		live, replay, _, unsubscribe, err := manager.SubscribeCurrent(ctx, runtimeKey, uuid.NewString(), snapshot.LastEventSequence)
		if err != nil {
			return kernel.ConversationTurnAwaitResult{}, err
		}
		if len(replay) > 0 {
			unsubscribe()
			continue
		}
		select {
		case <-ctx.Done():
			unsubscribe()
			return kernel.ConversationTurnAwaitResult{}, ctx.Err()
		case _, ok := <-live:
			unsubscribe()
			if !ok {
				continue
			}
		}
	}
}

type projectAdapter struct {
	engine     *business.Engine
	coreID     string
	workspaces *workspace.Service
}

func newProjectAdapter() *projectAdapter {
	return &projectAdapter{}
}

func projectWorkspacesInput(items []kernel.ProjectWorkspaceInput) []business.ProjectWorkspace {
	workspaces := make([]business.ProjectWorkspace, 0, len(items))
	for _, item := range items {
		workspaces = append(workspaces, business.ProjectWorkspace{ID: item.ID, DeviceID: item.DeviceID, Name: item.Name, Kind: item.Kind, RootURI: item.RootURI, ReadOnly: item.ReadOnly})
	}
	return workspaces
}

func (a *projectAdapter) ListProjects(ctx context.Context, limit int, cursor string) (any, error) {
	request, err := ownedHostScopeRequest(ctx, a.engine, a.coreID, "", "")
	if err != nil {
		return nil, err
	}
	result, err := a.engine.Query(ctx, request, coordination.DataQuery{Management: true, ResourceKind: "project", Cursor: cursor, Limit: limit})
	if err != nil {
		return nil, err
	}
	projects := make([]map[string]any, 0, len(result.Snapshot.Resources))
	orders := map[string]int64{}
	for _, row := range result.Snapshot.Resources {
		if row.Kind != "project" || row.Deleted {
			continue
		}
		var project business.Project
		if err := json.Unmarshal(row.Body, &project); err != nil {
			return nil, fmt.Errorf("项目数据格式无效: %w", err)
		}
		if project.ID != row.ID {
			return nil, fmt.Errorf("项目数据格式无效: project id %q does not match resource id %q", project.ID, row.ID)
		}
		if project.HiddenAt != nil && strings.TrimSpace(*project.HiddenAt) != "" {
			continue
		}
		projects = append(projects, map[string]any{"id": project.ID, "title": project.Title, "workspaces": project.Workspaces, "primaryWorkspaceId": project.PrimaryWorkspaceID, "hiddenAt": project.HiddenAt, "createdAt": project.CreatedAt, "updatedAt": project.UpdatedAt, "pinnedAt": project.PinnedAt, "order": project.Order, "revision": row.Revision, "ownerId": row.OwnerID, "roleId": row.RoleID})
		orders[project.ID] = business.ProjectSortOrder(project.Order, project.CreatedAt)
	}
	sort.SliceStable(projects, func(i, j int) bool {
		left, right := projects[i], projects[j]
		leftID, _ := left["id"].(string)
		rightID, _ := right["id"].(string)
		if orders[leftID] != orders[rightID] {
			return orders[leftID] < orders[rightID]
		}
		leftCreated, _ := left["createdAt"].(string)
		rightCreated, _ := right["createdAt"].(string)
		if leftCreated != rightCreated {
			return leftCreated < rightCreated
		}
		return leftID < rightID
	})
	return map[string]any{"projects": projects, "executionScope": result.Scope, "nextCursor": result.Snapshot.NextCursors["project"]}, nil
}

func (a *projectAdapter) ReorderProject(ctx context.Context, request kernel.ProjectReorderRequest) (json.RawMessage, error) {
	listing, err := a.ListProjects(ctx, 512, "")
	if err != nil {
		return nil, err
	}
	payload, ok := listing.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("项目列表格式无效")
	}
	rawProjects, _ := payload["projects"].([]map[string]any)
	entries := make([]orderPlanEntry, 0, len(rawProjects))
	for _, item := range rawProjects {
		id, _ := item["id"].(string)
		order, _ := item["order"].(int64)
		createdAt, _ := item["createdAt"].(string)
		entries = append(entries, orderPlanEntry{ID: id, Order: business.ProjectSortOrder(order, createdAt)})
	}
	plan, err := planSparseOrder(entries, request.ProjectID, request.BeforeProjectID, orderPlanStep)
	if err != nil {
		return nil, err
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	for index, change := range plan {
		order := change.Order
		if _, err := a.UpdateProject(ctx, kernel.ProjectUpdateRequest{ProjectID: change.ID, Order: &order, RequestID: fmt.Sprintf("%s#%d", requestID, index)}); err != nil {
			return nil, err
		}
	}
	orders := make(map[string]int64, len(entries))
	for _, entry := range entries {
		orders[entry.ID] = entry.Order
	}
	for _, change := range plan {
		orders[change.ID] = change.Order
	}
	ordered := append([]orderPlanEntry(nil), entries...)
	for index := range ordered {
		ordered[index].Order = orders[ordered[index].ID]
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Order != ordered[j].Order {
			return ordered[i].Order < ordered[j].Order
		}
		return ordered[i].ID < ordered[j].ID
	})
	ids := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		ids = append(ids, entry.ID)
	}
	return json.Marshal(map[string]any{"projectIds": ids})
}

func (a *projectAdapter) CreateProject(ctx context.Context, request kernel.ProjectCreateRequest) (json.RawMessage, error) {
	ownedRequest, err := ownedHostScopeRequest(ctx, a.engine, a.coreID, "", request.RequestID)
	if err != nil {
		return nil, err
	}
	if ownedRequest.ExpectedScope == nil {
		view, err := a.engine.Query(ctx, ownedRequest, coordination.DataQuery{Management: true, ResourceKind: "project", Limit: 1})
		if err != nil {
			return nil, err
		}
		scope := view.Scope
		scope.RequestID = ownedRequest.RequestID
		scope.TurnID, scope.ExecutionID = "", ""
		ownedRequest.ExpectedScope = &scope
	}
	workspaces := projectWorkspacesInput(request.Workspaces)
	primaryWorkspaceID := request.PrimaryWorkspaceID
	if request.DirectoryPath != "" {
		if a.workspaces == nil {
			return nil, fmt.Errorf("workspace service is not configured")
		}
		mount, err := a.workspaces.RegisterLocalMount(ctx, request.Title, request.DirectoryPath, false)
		if err != nil {
			return nil, err
		}
		workspaces = []business.ProjectWorkspace{{ID: string(mount.ID), DeviceID: ownedRequest.TargetDeviceID, Name: mount.Name, Kind: string(mount.Kind), RootURI: mount.RootURI}}
		primaryWorkspaceID = string(mount.ID)
	}
	result, err := a.engine.CreateProject(ctx, ownedRequest, business.CreateProjectInput{Title: request.Title, Workspaces: workspaces, PrimaryWorkspaceID: primaryWorkspaceID})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *projectAdapter) editProject(ctx context.Context, projectID, requestID string, changes map[string]json.RawMessage) (json.RawMessage, error) {
	ownedRequest, err := ownedHostScopeRequest(ctx, a.engine, a.coreID, "", requestID)
	if err != nil {
		return nil, err
	}
	resource, scope, err := a.engine.ReadResource(ctx, ownedRequest, "project", projectID)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted {
		return nil, coordination.ErrResourceVersion
	}
	acknowledgement, err := a.engine.Edit(ctx, ownedRequest, business.EditRequest{ExpectedScope: &scope, RoleID: ownedRequest.RoleID, RequestID: ownedRequest.RequestID, Kind: "project", ID: projectID, ExpectedRevision: resource.Revision, Changes: changes})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(map[string]any{"projectId": projectID, "revision": resource.Revision + 1, "acknowledgement": acknowledgement, "executionScope": scope})
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *kernelConversationReader) UpdateConversation(ctx context.Context, request kernel.ConversationUpdateRequest) (json.RawMessage, error) {
	if strings.TrimSpace(request.Title) == "" {
		return nil, fmt.Errorf("会话标题不能为空")
	}
	ownedRequest, err := ownedHostScopeRequest(ctx, a.engine, a.coreID, "", request.RequestID)
	if err != nil {
		return nil, err
	}
	resource, scope, err := a.engine.ReadResource(ctx, ownedRequest, "conversation", request.ConversationID)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted {
		return nil, coordination.ErrResourceVersion
	}
	encodedTitle, err := json.Marshal(request.Title)
	if err != nil {
		return nil, err
	}
	acknowledgement, err := a.engine.Edit(ctx, ownedRequest, business.EditRequest{
		ExpectedScope:    &scope,
		RoleID:           ownedRequest.RoleID,
		RequestID:        ownedRequest.RequestID,
		Kind:             "conversation",
		ID:               request.ConversationID,
		ExpectedRevision: resource.Revision,
		Changes:          map[string]json.RawMessage{"title": encodedTitle},
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"conversationId":  request.ConversationID,
		"title":           request.Title,
		"revision":        resource.Revision + 1,
		"acknowledgement": acknowledgement,
		"executionScope":  scope,
	})
}

func (a *projectAdapter) UpdateProject(ctx context.Context, request kernel.ProjectUpdateRequest) (json.RawMessage, error) {
	changes, err := projectUpdateChanges(request)
	if err != nil {
		return nil, err
	}
	if request.Workspaces != nil {
		encoded, err := json.Marshal(projectWorkspacesInput(request.Workspaces))
		if err != nil {
			return nil, err
		}
		changes["workspaces"] = encoded
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("没有需要修改的内容")
	}
	return a.editProject(ctx, request.ProjectID, request.RequestID, changes)
}

func projectUpdateChanges(request kernel.ProjectUpdateRequest) (map[string]json.RawMessage, error) {
	changes := map[string]json.RawMessage{}
	setChange := func(key string, value any) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		changes[key] = encoded
		return nil
	}
	for _, field := range []struct {
		key   string
		value any
		apply bool
	}{
		{"title", derefString(request.Title), request.Title != nil},
		{"pinned", derefBool(request.Pinned), request.Pinned != nil},
		{"hidden", derefBool(request.Hidden), request.Hidden != nil},
		{"primaryWorkspaceId", derefString(request.PrimaryWorkspaceID), request.PrimaryWorkspaceID != nil},
		{"order", derefInt64(request.Order), request.Order != nil},
	} {
		if !field.apply {
			continue
		}
		if err := setChange(field.key, field.value); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

func (a *projectAdapter) DeleteProject(ctx context.Context, request kernel.ProjectDeleteRequest) (json.RawMessage, error) {
	hidden, err := json.Marshal(true)
	if err != nil {
		return nil, err
	}
	return a.editProject(ctx, request.ProjectID, request.RequestID, map[string]json.RawMessage{"hidden": hidden})
}

type kernelMemoryQueryService struct {
	engine *business.Engine
}

type conversationMessageSenderAdapter struct {
	appender kernel.ConversationMessageAppender
}

func newConversationMessageSenderAdapter(appender kernel.ConversationMessageAppender) *conversationMessageSenderAdapter {
	return &conversationMessageSenderAdapter{appender: appender}
}

func (a *conversationMessageSenderAdapter) SendConversationMessage(ctx context.Context, request kernel.ConversationMessageRequest) (kernel.ConversationMessageResult, error) {
	if a == nil || a.appender == nil {
		return kernel.ConversationMessageResult{}, fmt.Errorf("conversation message appender is not configured")
	}
	_, err := a.appender.AppendConversationMessages(ctx, kernel.ConversationMessageAppendRequest{
		SpaceID:        request.SpaceID,
		CharacterID:    request.CharacterID,
		ConversationID: request.ConversationID,
		Channel:        request.Channel,
		Role:           "assistant",
		Source:         "extension",
		RequestID:      request.RequestID,
		Parts: []kernel.ConversationMessagePart{
			{Type: "text", Content: request.Content},
		},
	})
	if err != nil {
		return kernel.ConversationMessageResult{}, err
	}
	return kernel.ConversationMessageResult{
		Content:   request.Content,
		RequestID: request.RequestID,
	}, nil
}

type conversationMessageAppenderAdapter struct {
	engine          *business.Engine
	coreID          string
	resourceLinks   extensionResourceResolver
	channelDelivery channelDeliveryRecovery
}

type extensionResourceResolver interface {
	ResolveResourceLink(token string) (extensionID string, scope string, fullPath string, err error)
}

type channelDeliveryRecovery interface {
	ResumeChannelDeliveries(ctx context.Context) error
}

func newConversationMessageAppenderAdapter() *conversationMessageAppenderAdapter {
	return &conversationMessageAppenderAdapter{}
}

func (a *conversationMessageAppenderAdapter) AppendConversationMessages(ctx context.Context, request kernel.ConversationMessageAppendRequest) (kernel.ConversationMessageAppendResult, error) {
	if a == nil || a.engine == nil {
		return kernel.ConversationMessageAppendResult{}, fmt.Errorf("chat service is not configured")
	}
	parts := make([]business.AppendedMessagePart, len(request.Parts))
	for index, part := range request.Parts {
		parts[index] = business.AppendedMessagePart{
			Type:          part.Type,
			Content:       part.Content,
			ExtensionType: part.ExtensionType,
			MIMEType:      part.MIMEType,
			URL:           part.URL,
			FallbackURL:   part.FallbackURL,
			AltText:       part.AltText,
			Width:         part.Width,
			Height:        part.Height,
			IsAnimated:    part.IsAnimated,
			Metadata:      part.Metadata,
		}
	}
	if request.SpaceID != kernel.GetHostAPIScope(ctx).SpaceID {
		return kernel.ConversationMessageAppendResult{}, coordination.ErrWrongOwner
	}
	if request.RequestID == "" {
		request.RequestID = uuid.NewString()
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, request.CharacterID, request.RequestID)
	if err != nil {
		return kernel.ConversationMessageAppendResult{}, err
	}
	if ownedRequest.ExpectedScope != nil && strings.EqualFold(strings.TrimSpace(request.Role), "assistant") {
		if strings.TrimSpace(ownedRequest.ExpectedScope.TurnID) == "" {
			ownedRequest.ExpectedScope.TurnID = "manual-turn-" + request.RequestID
		}
		if strings.TrimSpace(ownedRequest.ExpectedScope.ExecutionID) == "" {
			ownedRequest.ExpectedScope.ExecutionID = "manual-execution-" + request.RequestID
		}
	}
	if strings.EqualFold(strings.TrimSpace(request.Role), "user") {
		for i, part := range parts {
			media := &interaction.UnifiedEntryRequest{}
			switch strings.ToLower(strings.TrimSpace(part.Type)) {
			case "image":
				media.ImageUrl, err = trustedExtensionMediaSource(part.URL, a.resourceLinks)
			case "audio":
				media.AudioUrl, err = trustedExtensionMediaSource(part.URL, a.resourceLinks)
			case "video":
				media.VideoUrl, err = trustedExtensionMediaSource(part.URL, a.resourceLinks)
			default:
				continue
			}
			if err != nil {
				return kernel.ConversationMessageAppendResult{}, err
			}
			parts[i].Attachments, err = ownedIngressAttachments(ctx, media)
			if err != nil {
				return kernel.ConversationMessageAppendResult{}, err
			}
		}
	}
	result, err := a.engine.AppendOwnedMessages(ctx, ownedRequest, business.AppendOwnedMessagesRequest{ConversationID: request.ConversationID, Role: request.Role, Source: request.Source, Channel: request.Channel, ReplyToMessageID: request.ReplyToMessageID, Parts: parts})
	if err != nil {
		return kernel.ConversationMessageAppendResult{}, err
	}
	if result.Status == "sending" && a.channelDelivery != nil {
		if err := a.channelDelivery.ResumeChannelDeliveries(ctx); err != nil {
			return kernel.ConversationMessageAppendResult{}, fmt.Errorf("resume channel delivery: %w", err)
		}
	}
	bus := system.GetMessageEventBus()
	direction := "outbound"
	if result.Role == "user" {
		direction = "inbound"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	for index, messageID := range result.MessageIDs {
		if index >= len(parts) {
			break
		}
		part := parts[index]
		sequence := result.LastSequence
		if index < len(result.Sequences) {
			sequence = result.Sequences[index]
		}
		bus.PublishMessageCreated(
			request.ConversationID,
			messageID,
			result.Channel,
			direction,
			result.Role,
			result.Status,
			part.Content,
			now,
			sequence,
			map[string]any{
				"type":          part.Type,
				"extensionType": part.ExtensionType,
				"url":           part.URL,
				"fallbackUrl":   part.FallbackURL,
				"altText":       part.AltText,
				"width":         part.Width,
				"height":        part.Height,
				"isAnimated":    part.IsAnimated,
			},
		)
	}
	return kernel.ConversationMessageAppendResult{
		MessageIDs:      result.MessageIDs,
		Sequences:       result.Sequences,
		DeliveryGroupID: result.DeliveryGroupID,
		LastSequence:    result.LastSequence,
	}, nil
}

func newKernelMemoryQueryService() *kernelMemoryQueryService {
	return &kernelMemoryQueryService{}
}

func (s *kernelMemoryQueryService) Query(ctx context.Context, extensionID string, query string, limit int) ([]json.RawMessage, error) {
	scope, owned := coordination.FromContext(ctx)
	host := kernel.GetHostAPIScope(ctx)
	if !owned || s.engine == nil || scope.SpaceID != host.SpaceID || scope.RoleID != host.CharacterID {
		return nil, fmt.Errorf("记忆查询缺少当前数据归属")
	}
	if strings.TrimSpace(query) == "" {
		return []json.RawMessage{}, nil
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	result, err := s.engine.SearchOwnedMemories(ctx, business.Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, TargetDeviceID: scope.TargetDeviceID, CoreID: scope.CoreID, RoleID: scope.RoleID, RequestID: scope.RequestID + "|host-memory"}, business.MemoryManagementQuery{Query: query, Mode: "keyword", Limit: limit, ContextUsable: true, IncludeDormant: true})
	if err != nil {
		return nil, err
	}
	results := []json.RawMessage{}
	for _, row := range result.Resources {
		if !coordination.ResourceUsable(row.Body, time.Now()) {
			continue
		}
		var document struct {
			Key     string `json:"key"`
			Content struct {
				Value       string `json:"value"`
				Text        string `json:"text"`
				MemoryType  string `json:"memoryType"`
				Importance  int    `json:"importance"`
				Sensitivity string `json:"sensitivityLevel"`
			} `json:"content"`
		}
		if json.Unmarshal(row.Body, &document) != nil {
			return nil, fmt.Errorf("记忆记录格式无效")
		}
		text := document.Content.Value
		if text == "" {
			text = document.Content.Text
		}
		if document.Content.Sensitivity == "confidential" || document.Content.Sensitivity == "restricted" {
			text = "[restricted]"
		} else if utf8.RuneCountInString(text) > maxMemoryValueLength {
			text = truncateRunes(text, maxMemoryValueLength)
		}
		entry, err := json.Marshal(map[string]any{"id": row.ID, "key": document.Key, "value": text, "memoryType": document.Content.MemoryType, "importance": document.Content.Importance})
		if err != nil {
			return nil, err
		}
		results = append(results, entry)
	}
	return results, nil
}

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

type modelGatewayAdapter struct {
	service chat.Service
}

func newModelGatewayAdapter(service chat.Service) *modelGatewayAdapter {
	return &modelGatewayAdapter{service: service}
}

type conversationForkAdapter struct {
	service chat.Service
}

func (a *conversationForkAdapter) ForkConversation(ctx context.Context, request kernel.ConversationForkRequest) (json.RawMessage, error) {
	if a == nil || a.service == nil {
		return nil, fmt.Errorf("chat service is unavailable")
	}
	forker, ok := a.service.(interface {
		ForkConversation(chat.ForkConversationRequest) (*chat.Conversation, error)
	})
	if !ok {
		return nil, fmt.Errorf("chat service does not implement ForkConversation")
	}
	fork, err := forker.ForkConversation(chat.ForkConversationRequest{
		SourceConversationID: request.ConversationID,
		BoundarySequence:     request.BoundarySequence,
		Title:                request.Title,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(map[string]any{
		"conversationId": fork.ID,
		"forkedFrom":     request.ConversationID,
		"title":          fork.Title,
		"messageCount":   fork.MessageCount,
	})
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *modelGatewayAdapter) ModelDefaults(ctx context.Context) (map[string]any, error) {
	if a == nil || a.service == nil {
		return nil, fmt.Errorf("chat model service is unavailable")
	}
	models, err := a.service.ListModels()
	if err != nil {
		return nil, err
	}
	var active *chat.ModelConfig
	for index := range models {
		if models[index].IsActive == 1 {
			active = &models[index]
			break
		}
	}
	if active == nil {
		return nil, fmt.Errorf("no active chat model configured")
	}
	return map[string]any{
		"provider":        active.APIType,
		"model":           active.ModelName,
		"temperature":     active.Temperature,
		"maxTokens":       active.MaxTokens,
		"maxOutputTokens": active.MaxOutputTokens,
		"contextWindow":   active.ContextWindow,
	}, nil
}

func (a *modelGatewayAdapter) InvokeModelGateway(ctx context.Context, request kernel.ModelInvokeRequest) (*kernel.ModelInvokeResult, error) {
	if a == nil || a.service == nil {
		return nil, fmt.Errorf("chat model service is unavailable")
	}
	messages := make([]chat.ModelGatewayMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		calls := make([]chat.ModelGatewayToolCall, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			calls = append(calls, chat.ModelGatewayToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
		}
		messages = append(messages, chat.ModelGatewayMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  calls,
			ToolCallID: message.ToolCallID,
		})
	}
	tools := make([]chat.ModelGatewayTool, 0, len(request.Tools))
	for _, candidate := range request.Tools {
		tools = append(tools, chat.ModelGatewayTool{
			Name:        candidate.Name,
			Description: candidate.Description,
			Parameters:  candidate.Parameters,
		})
	}
	result, err := a.service.InvokeModelGateway(ctx, chat.ModelGatewayRequest{
		Provider:        request.Provider,
		Model:           request.Model,
		System:          request.System,
		Messages:        messages,
		Tools:           tools,
		Temperature:     request.Temperature,
		MaxOutputTokens: request.MaxOutputTokens,
		ReasoningEffort: request.ReasoningEffort,
		SessionID:       request.SessionID,
		Purpose:         request.Purpose,
	})
	if err != nil {
		return nil, err
	}
	calls := make([]kernel.ModelInvokeToolCall, 0, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		calls = append(calls, kernel.ModelInvokeToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return &kernel.ModelInvokeResult{
		Provider:           result.Provider,
		Model:              result.Model,
		Protocol:           result.Protocol,
		ContextWindow:      result.ContextWindow,
		Text:               result.Text,
		Reasoning:          result.Reasoning,
		Refusal:            result.Refusal,
		ToolCalls:          calls,
		FinishReason:       result.FinishReason,
		ProviderResponseID: result.ProviderResponseID,
		Usage: kernel.ModelInvokeUsage{
			Reported:           result.Usage.Reported,
			InputTokens:        result.Usage.InputTokens,
			OutputTokens:       result.Usage.OutputTokens,
			ReasoningTokens:    result.Usage.ReasoningTokens,
			CachedInputTokens:  result.Usage.CachedInputTokens,
			CacheUsageReported: result.Usage.CacheUsageReported,
			TotalTokens:        result.Usage.TotalTokens,
		},
	}, nil
}

type kernelDirectoryAdapter struct{}

func newKernelDirectoryAdapter() *kernelDirectoryAdapter { return &kernelDirectoryAdapter{} }

func (a *kernelDirectoryAdapter) ListDirectory(ctx context.Context, request kernel.DirectoryListRequest) (kernel.DirectoryListResult, error) {
	if err := ctx.Err(); err != nil {
		return kernel.DirectoryListResult{}, err
	}
	entries, err := os.ReadDir(request.Path)
	if err != nil {
		return kernel.DirectoryListResult{}, err
	}
	result := kernel.DirectoryListResult{Path: request.Path, Entries: make([]kernel.DirectoryEntry, 0, len(entries))}
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil {
			return kernel.DirectoryListResult{}, statErr
		}
		kind := "file"
		if entry.IsDir() {
			kind = "directory"
		}
		result.Entries = append(result.Entries, kernel.DirectoryEntry{
			Name:       entry.Name(),
			Path:       filepath.Join(request.Path, entry.Name()),
			Kind:       kind,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Name < result.Entries[j].Name
	})
	if request.Limit > 0 && len(result.Entries) > request.Limit {
		result.Entries = result.Entries[:request.Limit]
	}
	return result, nil
}

func (a *kernelDirectoryAdapter) CreateDirectory(ctx context.Context, request kernel.DirectoryCreateRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	target := request.Path
	if request.Name != "" {
		target = filepath.Join(request.Path, request.Name)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", err
	}
	return target, nil
}

func derefString(value *string) any {
	if value == nil {
		return ""
	}
	return *value
}

func derefBool(value *bool) any {
	if value == nil {
		return false
	}
	return *value
}

func derefInt64(value *int64) any {
	if value == nil {
		return int64(0)
	}
	return *value
}

type conversationAttachmentInput struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	MIME string `json:"mimeType"`
	Data string `json:"data"`
	Hash string `json:"sha256"`
}

func materializeConversationAttachments(inputs []conversationAttachmentInput) ([]map[string]any, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	root := filepath.Join(os.TempDir(), "amitia-dsh-attachments")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("创建附件缓存目录失败: %w", err)
	}
	result := make([]map[string]any, 0, len(inputs))
	for _, input := range inputs {
		payload, err := base64.StdEncoding.DecodeString(input.Data)
		if err != nil {
			return nil, fmt.Errorf("附件 %q 解码失败: %w", input.Name, err)
		}
		if len(payload) == 0 {
			return nil, fmt.Errorf("附件 %q 内容为空", input.Name)
		}
		digest := sha256.Sum256(payload)
		hash := hex.EncodeToString(digest[:])
		if input.Hash != "" && input.Hash != hash {
			return nil, fmt.Errorf("附件 %q 完整性校验失败: 期望 %s 实际 %s", input.Name, input.Hash, hash)
		}
		fileName := sanitizeAttachmentFileName(input.Name, hash)
		absolutePath := filepath.Join(root, hash[:16]+"-"+fileName)
		if _, statErr := os.Stat(absolutePath); statErr != nil {
			if !os.IsNotExist(statErr) {
				return nil, fmt.Errorf("读取附件缓存失败: %w", statErr)
			}
			tempFile, createErr := os.CreateTemp(root, "attach-*.tmp")
			if createErr != nil {
				return nil, fmt.Errorf("创建附件副本失败: %w", createErr)
			}
			tempPath := tempFile.Name()
			if _, writeErr := tempFile.Write(payload); writeErr != nil {
				tempFile.Close()
				os.Remove(tempPath)
				return nil, fmt.Errorf("写入附件副本失败: %w", writeErr)
			}
			if closeErr := tempFile.Close(); closeErr != nil {
				os.Remove(tempPath)
				return nil, fmt.Errorf("关闭附件副本失败: %w", closeErr)
			}
			if chmodErr := os.Chmod(tempPath, 0o444); chmodErr != nil {
				os.Remove(tempPath)
				return nil, fmt.Errorf("设置附件副本只读失败: %w", chmodErr)
			}
			if renameErr := os.Rename(tempPath, absolutePath); renameErr != nil {
				os.Remove(tempPath)
				return nil, fmt.Errorf("发布附件副本失败: %w", renameErr)
			}
		}
		result = append(result, map[string]any{
			"id":           hash,
			"kind":         input.Kind,
			"filename":     input.Name,
			"mimeType":     input.MIME,
			"sizeBytes":    int64(len(payload)),
			"sha256":       hash,
			"absolutePath": absolutePath,
		})
	}
	return result, nil
}

func sanitizeAttachmentFileName(name, fallback string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "/", "_"), "\\", "_"))
	trimmed = strings.TrimLeft(trimmed, ".")
	if trimmed == "" {
		return fallback[:16]
	}
	if len(trimmed) > 120 {
		trimmed = trimmed[len(trimmed)-120:]
	}
	return trimmed
}

func (a *conversationCreatorAdapter) countConversationMessages(ctx context.Context, conversationID string, role string) (int, error) {
	request, err := ownedHostChatRequest(ctx, a.engine, a.coreID, conversationID, role, uuid.NewString())
	if err != nil {
		return 0, err
	}
	result, err := a.engine.ManageChat(ctx, request, coordination.ChatManagementRequest{
		Operation:           "search",
		ConversationOwnerID: request.ExpectedScope.ResourceOwnerID,
		ConversationID:      conversationID,
		Page:                1,
		PageSize:            1,
	})
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return 0, err
	}
	var batch struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(raw, &batch); err != nil {
		return 0, err
	}
	return batch.Total, nil
}
