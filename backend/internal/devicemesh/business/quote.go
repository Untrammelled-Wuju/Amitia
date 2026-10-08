package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type QuoteReference struct {
	OwnerID          string                       `json:"ownerId"`
	RoleID           string                       `json:"characterId"`
	ConversationID   string                       `json:"conversationId"`
	MessageID        string                       `json:"messageId"`
	ExpectedRevision int64                        `json:"expectedRevision"`
	ContentHash      string                       `json:"contentHash"`
	ExpectedScope    *coordination.ExecutionScope `json:"expectedExecutionScope"`
}

type ReviewedQuote struct {
	OwnerID        string `json:"ownerId"`
	RoleID         string `json:"characterId"`
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
	Revision       int64  `json:"revision"`
	ContentHash    string `json:"contentHash"`
	Role           string `json:"role"`
	Content        string `json:"content"`
}

func (e *Engine) reviewQuote(ctx context.Context, scope coordination.ExecutionScope, request Request, reference *QuoteReference) (*ReviewedQuote, []coordination.ResourceVersion, error) {
	if reference == nil {
		return nil, nil, nil
	}
	if reference.ExpectedScope == nil || summaryAuthority(*reference.ExpectedScope) != summaryAuthority(scope) {
		return nil, nil, coordination.ErrScopeExpired
	}
	for _, id := range []string{reference.OwnerID, reference.RoleID, reference.ConversationID, reference.MessageID} {
		if id == "" || len(id) > 512 || strings.ContainsRune(id, '\x00') {
			return nil, nil, coordination.ErrWrongOwner
		}
	}
	if reference.ExpectedRevision < 0 || len(reference.ContentHash) != 64 {
		return nil, nil, errors.New("引用消息版本或完整性标识无效")
	}
	route, err := e.resolveRunConversationRoute(ctx, scope, request)
	if err != nil {
		return nil, nil, err
	}
	if reference.OwnerID == scope.ResourceOwnerID {
		if reference.RoleID != scope.RoleID || reference.ConversationID != route.CurrentID {
			return nil, nil, coordination.ErrWrongOwner
		}
	} else {
		if !scope.Coordinated || request.ConversationOrigin == nil || reference.OwnerID != scope.TargetDeviceID || reference.OwnerID != request.ConversationOrigin.OwnerID || reference.ConversationID != request.ConversationOrigin.ID || route.HistoricalID != reference.ConversationID {
			return nil, nil, coordination.ErrWrongOwner
		}
		if request.HistoricalRoleID != "" && request.HistoricalRoleID != reference.RoleID {
			return nil, nil, coordination.ErrWrongOwner
		}
		request.HistoricalRoleID = reference.RoleID
	}
	messages, _, err := e.summaryHistory(ctx, request, scope)
	if err != nil {
		return nil, nil, err
	}
	for _, message := range messages {
		if message.OwnerID != reference.OwnerID || message.ID != reference.MessageID {
			continue
		}
		digest := sha256.Sum256([]byte(message.Content))
		if message.Revision != reference.ExpectedRevision || hex.EncodeToString(digest[:]) != reference.ContentHash {
			return nil, nil, coordination.ErrResourceVersion
		}
		quote := &ReviewedQuote{OwnerID: reference.OwnerID, RoleID: reference.RoleID, ConversationID: reference.ConversationID, MessageID: reference.MessageID, Revision: message.Revision, ContentHash: reference.ContentHash, Role: message.Role, Content: message.Content}
		var dependencies []coordination.ResourceVersion
		if message.OwnerID == scope.ResourceOwnerID && message.Revision > 0 {
			dependencies = append(dependencies, coordination.ResourceVersion{Kind: "message", ID: message.ID, Revision: message.Revision})
		}
		return quote, dependencies, nil
	}
	return nil, nil, errors.New("引用消息不存在、已删除或不属于当前授权会话")
}
