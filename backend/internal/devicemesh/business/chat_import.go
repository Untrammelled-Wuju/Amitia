package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) commitImportedChat(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatManagementRequest, archive map[string]any) (map[string]any, error) {
	if request.RequestID == "" || len(request.RequestID) > 256 {
		return nil, errors.New("导入请求编号无效")
	}
	raw, err := json.Marshal(archive)
	if err != nil {
		return nil, err
	}
	var source struct {
		Conversation map[string]any   `json:"conversation"`
		Messages     []map[string]any `json:"messages"`
		OwnerID      string           `json:"sourceOwnerId"`
		RoleID       string           `json:"sourceRoleId"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	batchID := request.ImportBatchID
	if batchID == "" {
		batchID = request.ConversationID
	}
	if source.OwnerID != request.ConversationOwnerID || source.Conversation == nil || source.Conversation["id"] != batchID {
		return nil, coordination.ErrWrongOwner
	}
	identity := sha256.Sum256([]byte(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00" + scope.RoleID + "\x00" + source.OwnerID + "\x00" + batchID))
	id := "import-" + hex.EncodeToString(identity[:16])
	mutations := []coordination.Mutation{}
	for index, message := range source.Messages {
		original, ok := message["id"].(string)
		if !ok || original == "" || message["conversationId"] != batchID || message["characterId"] != source.RoleID {
			return nil, coordination.ErrWrongOwner
		}
		messageID := fmt.Sprintf("%s-message-%06d", id, index)
		message["id"], message["conversationId"], message["characterId"], message["ownerId"] = messageID, id, scope.RoleID, scope.ResourceOwnerID
		message["importOrigin"] = map[string]any{"ownerId": source.OwnerID, "conversationId": batchID, "messageId": original, "roleId": source.RoleID}
		mutations = append(mutations, coordination.Mutation{Kind: "message", ID: messageID, RoleID: scope.RoleID, Body: body(message)})
	}
	for start := 0; start < len(mutations); start += 128 {
		end := min(start+128, len(mutations))
		batchScope := scope
		batchScope.RequestID = fmt.Sprintf("%s/batch/%d", id, start/128)
		if _, err := e.commit(ctx, coordination.Commit{Scope: batchScope, Mutations: mutations[start:end]}); err != nil {
			return nil, err
		}
	}
	conversation := source.Conversation
	conversation["id"], conversation["characterId"], conversation["ownerId"] = id, scope.RoleID, scope.ResourceOwnerID
	conversation["source"], conversation["channel"] = "import", "web"
	conversation["messageCount"] = len(source.Messages)
	conversation["importOrigin"] = map[string]any{"ownerId": source.OwnerID, "conversationId": batchID, "roleId": source.RoleID}
	batchScope := scope
	batchScope.RequestID = id + "/conversation"
	if _, err := e.commit(ctx, coordination.Commit{Scope: batchScope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: id, RoleID: scope.RoleID, Body: body(conversation)}}}); err != nil {
		return nil, err
	}
	if manager, ok := e.data.(coordination.ChatManagementPort); ok {
		promotion := request
		promotion.Operation = "import-promoted"
		promotion.ImportBatchID = batchID
		promotion.ConversationID = id
		promotion.ConversationOwnerID = scope.ResourceOwnerID
		promotion.RequestID = id + "/promoted"
		if _, err := manager.ManageChat(ctx, scope, promotion); err != nil {
			return nil, err
		}
	}
	return map[string]any{"conversationId": id, "imported": true, "executionScope": scope}, nil
}
