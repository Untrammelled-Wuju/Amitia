package business

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type QueryResult struct {
	ConversationID           string                         `json:"conversationId,omitempty"`
	ConversationOrigin       *ConversationOrigin            `json:"conversationOrigin,omitempty"`
	DeliveryFailures         []coordination.DeliveryFailure `json:"deliveryFailures,omitempty"`
	NextHistoricalListCursor string                         `json:"nextHistoricalListCursor,omitempty"`
	HistoricalConversations  []json.RawMessage              `json:"historicalConversations,omitempty"`
	Scope                    coordination.ExecutionScope    `json:"executionScope"`
	Snapshot                 coordination.DataSnapshot      `json:"snapshot"`
	HistoricalSnapshot       *coordination.DataSnapshot     `json:"historicalSnapshot,omitempty"`
}

func (e *Engine) Query(ctx context.Context, request Request, query coordination.DataQuery) (QueryResult, error) {
	search, err := coordination.NormalizeConversationSearch(query.SearchQuery)
	if err != nil {
		return QueryResult{}, err
	}
	if search != "" {
		if !query.ListConversations || query.ConversationID != "" || query.ResourceKind != "" && query.ResourceKind != "conversation" {
			return QueryResult{}, coordination.ErrWrongOwner
		}
		query.SearchQuery, query.ResourceKind = search, "conversation"
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, uuid.NewString())
	if err != nil {
		return QueryResult{}, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return QueryResult{}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return QueryResult{}, err
	}
	requestedRole := request.RoleID
	if requestedRole == "" {
		policy, err := e.coordination.Get(ctx, scope.SpaceID, scope.TargetDeviceID)
		if err != nil {
			return QueryResult{}, err
		}
		requestedRole = policy.SelectedRole
	}
	role, err := coordination.ResolveRole(requestedRole, roles)
	if err != nil {
		return QueryResult{}, err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	ctx = coordination.WithScope(ctx, scope)
	conversationRoute, err := e.resolveStoredConversationRoute(ctx, scope, query.ConversationID, request.ConversationOrigin)
	if err != nil {
		return QueryResult{}, err
	}
	query.ConversationID = conversationRoute.CurrentID
	snapshot, err := e.data.Snapshot(ctx, scope, query)
	if err != nil {
		return QueryResult{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return QueryResult{}, err
	}
	result := QueryResult{ConversationID: conversationRoute.CurrentID, Scope: scope, Snapshot: snapshot, ConversationOrigin: conversationRoute.Origin}
	if port, ok := e.data.(coordination.HistoricalConversationPagePort); ok && scope.Coordinated && query.ListConversations {
		page, pageErr := port.HistoricalConversationPage(ctx, scope, query)
		if pageErr != nil {
			return QueryResult{}, pageErr
		}
		result.HistoricalConversations, result.NextHistoricalListCursor = page.Conversations, page.NextCursor
	} else if port, ok := e.data.(coordination.HistoricalConversationPort); ok && scope.Coordinated && query.ListConversations {
		result.HistoricalConversations, err = port.HistoricalConversations(ctx, scope)
		if err != nil {
			return QueryResult{}, err
		}
	}
	if port, ok := e.data.(coordination.HistoricalDataPort); ok && scope.Coordinated && (conversationRoute.HistoricalID != "" || query.Management && query.HistoricalRoleID != "") && ((query.Cursor == "" && query.LegacyCursor == "") || query.HistoricalCursor != "" || query.HistoricalLegacyCursor != "") {
		historicalQuery := query
		historicalQuery.ConversationID = conversationRoute.HistoricalID
		historicalQuery.Cursor = query.HistoricalCursor
		historicalQuery.HistoricalCursor = ""
		historicalQuery.LegacyCursor = query.HistoricalLegacyCursor
		historicalQuery.HistoricalLegacyCursor = ""
		result.HistoricalSnapshot, err = port.HistoricalSnapshot(ctx, scope, historicalQuery)
		if err != nil {
			return QueryResult{}, err
		}
	}
	if err := rejectAmbiguousConversation(conversationRoute, result.Snapshot, result.HistoricalSnapshot); err != nil {
		return QueryResult{}, err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return QueryResult{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return QueryResult{}, err
	}
	result.DeliveryFailures, err = e.coordination.DeliveryFailures(ctx, scope, query.ConversationID)
	if err != nil {
		return QueryResult{}, err
	}
	return result, nil
}
