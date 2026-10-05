package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type meshHistoricalPosition struct {
	ID        string `json:"id"`
	UpdatedAt string `json:"updatedAt"`
	Done      bool   `json:"done"`
}

type meshHistoricalListCursor struct {
	Fence      string                 `json:"fence"`
	Owner      string                 `json:"owner"`
	Core       string                 `json:"core"`
	Role       string                 `json:"role"`
	Epoch      int64                  `json:"epoch"`
	Mode       int64                  `json:"mode"`
	Permission int64                  `json:"permission"`
	SearchHash string                 `json:"searchHash,omitempty"`
	Owned      meshHistoricalPosition `json:"owned"`
	Legacy     meshHistoricalPosition `json:"legacy"`
}

func (p *meshLocalDataPort) HistoricalConversationPage(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.HistoricalConversationPage, error) {
	result := coordination.HistoricalConversationPage{Conversations: []json.RawMessage{}}
	search, err := coordination.NormalizeConversationSearch(query.SearchQuery)
	if err != nil {
		return result, err
	}
	if search != "" && query.ConversationID != "" {
		return result, coordination.ErrWrongOwner
	}
	if !scope.Coordinated || scope.TargetDeviceID != p.ownerID || scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID || !query.ListConversations {
		return result, coordination.ErrWrongOwner
	}
	stableScope := scope
	stableScope.RequestID, stableScope.TurnID, stableScope.ExecutionID = "", "", ""
	encodedScope, err := json.Marshal(stableScope)
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(encodedScope)
	cursor := meshHistoricalListCursor{Fence: hex.EncodeToString(digest[:]), Owner: p.ownerID, Core: scope.CoreID, Role: scope.RoleID, Epoch: scope.TargetProviderEpoch, Mode: scope.ModeRevision, Permission: scope.TargetPermissionRevision}
	cursor.SearchHash = coordination.ConversationSearchHash(search)
	if query.HistoricalListCursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(query.HistoricalListCursor)
		var previous meshHistoricalListCursor
		if err != nil || len(raw) > 4096 || json.Unmarshal(raw, &previous) != nil || previous.Fence != cursor.Fence || previous.Owner != cursor.Owner || previous.Core != cursor.Core || previous.Role != cursor.Role || previous.Epoch != cursor.Epoch || previous.Mode != cursor.Mode || previous.Permission != cursor.Permission || previous.SearchHash != cursor.SearchHash {
			return result, coordination.ErrWrongOwner
		}
		cursor = previous
	}
	limit := query.Limit
	if limit <= 0 || limit > 128 {
		limit = 128
	}
	appendRow := func(raw json.RawMessage) error {
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		if id, _ := value["id"].(string); id == "" {
			return coordination.ErrWrongOwner
		}
		value["ownerId"] = p.ownerID
		encoded, err := json.Marshal(value)
		if err == nil {
			result.Conversations = append(result.Conversations, encoded)
		}
		return err
	}
	if !cursor.Owned.Done {
		statement := `SELECT resource_id,updated_at,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND deleted=0`
		args := []any{p.ownerID}
		if search != "" {
			statement += coordination.OwnedConversationSearchPredicate
			args = append(args, search, search)
		}
		if cursor.Owned.ID != "" {
			statement += ` AND (updated_at<? OR (updated_at=? AND resource_id<?))`
			args = append(args, cursor.Owned.UpdatedAt, cursor.Owned.UpdatedAt, cursor.Owned.ID)
		}
		args = append(args, limit+1)
		rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, statement+` ORDER BY updated_at DESC,resource_id DESC LIMIT ?`, args...)
		if err != nil {
			return result, err
		}
		count := 0
		for rows.Next() {
			var id, updated string
			var raw json.RawMessage
			if err := rows.Scan(&id, &updated, &raw); err != nil {
				_ = rows.Close()
				return result, err
			}
			count++
			if count > limit {
				break
			}
			if err := appendRow(raw); err != nil {
				_ = rows.Close()
				return result, err
			}
			cursor.Owned.ID, cursor.Owned.UpdatedAt = id, updated
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return result, err
		}
		cursor.Owned.Done = count <= limit
	}
	if !cursor.Legacy.Done {
		db := p.services.DB.WithContext(ctx).Where("deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default')", p.legacySpaceID)
		if search != "" {
			db = db.Where("instr(lower(COALESCE(title,'')),?)>0 OR id IN (SELECT conversation_id FROM messages WHERE deleted_at IS NULL AND instr(lower(content),?)>0)", search, search)
		}
		if cursor.Legacy.ID != "" {
			db = db.Where("updated_at<? OR (updated_at=? AND id<?)", cursor.Legacy.UpdatedAt, cursor.Legacy.UpdatedAt, cursor.Legacy.ID)
		}
		var conversations []chat.Conversation
		if err := db.Order("updated_at DESC,id DESC").Limit(limit + 1).Find(&conversations).Error; err != nil {
			return result, err
		}
		cursor.Legacy.Done = len(conversations) <= limit
		if len(conversations) > limit {
			conversations = conversations[:limit]
		}
		for _, row := range conversations {
			raw, err := json.Marshal(row)
			if err != nil {
				return result, err
			}
			if err := appendRow(raw); err != nil {
				return result, err
			}
			cursor.Legacy.ID, cursor.Legacy.UpdatedAt = row.ID, row.UpdatedAt
		}
	}
	if !cursor.Owned.Done || !cursor.Legacy.Done {
		raw, err := json.Marshal(cursor)
		if err != nil {
			return result, err
		}
		result.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, ctx.Err()
}
