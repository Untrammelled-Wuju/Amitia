package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (r *ownedChatIngress) reviewReplyReference(ctx context.Context, request business.Request, id string) (*business.QuoteReference, error) {
	if request.ConversationID == "" {
		return nil, errors.New("引用消息缺少所属对话")
	}
	view, err := r.services.OwnedBusiness.Query(ctx, request, coordination.DataQuery{ConversationID: request.ConversationID, ResourceKind: "message", ResourceID: id, Limit: 128})
	if err != nil {
		return nil, err
	}
	create := func(raw json.RawMessage, revision int64) (*business.QuoteReference, error) {
		var document struct {
			ID             string `json:"id"`
			ConversationID string `json:"conversationId"`
			Content        string `json:"content"`
		}
		if err := json.Unmarshal(raw, &document); err != nil {
			return nil, err
		}
		if document.ID != id || document.ConversationID != request.ConversationID {
			return nil, coordination.ErrWrongOwner
		}
		digest := sha256.Sum256([]byte(document.Content))
		return &business.QuoteReference{OwnerID: view.Scope.ResourceOwnerID, RoleID: view.Scope.RoleID, ConversationID: request.ConversationID, MessageID: id, ExpectedRevision: revision, ContentHash: hex.EncodeToString(digest[:]), ExpectedScope: &view.Scope}, nil
	}
	for _, row := range view.Snapshot.Resources {
		if row.Kind == "message" && row.ID == id {
			return create(row.Body, row.Revision)
		}
	}
	for _, raw := range view.Snapshot.LegacyMessages {
		var document struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &document) == nil && document.ID == id {
			return create(raw, 0)
		}
	}
	return nil, errors.New("引用消息不存在或不属于当前授权对话")
}
