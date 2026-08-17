// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"encoding/json"

	"gorm.io/gorm"
)

const (
	EntityTypeConversation EntityType = "conversation"
	EntityTypeMessage      EntityType = "message"
	EntityTypeCharacter    EntityType = "character"
	EntityTypeSettings     EntityType = "settings"
)

type mutationPayload struct {
	Title          string                 `json:"title"`
	Content        string                 `json:"content"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	ConversationID string                 `json:"conversationId,omitempty"`
	Role           string                 `json:"role,omitempty"`
	Sequence       int64                  `json:"sequence,omitempty"`
	MsgType        string                 `json:"msgType,omitempty"`
	Source         string                 `json:"source,omitempty"`
	Key            string                 `json:"key,omitempty"`
	Value          string                 `json:"value,omitempty"`
}

type businessApplier struct {
	db *gorm.DB
}

func NewBusinessApplier(db *gorm.DB) EntityMutationApplier {
	return &businessApplier{db: db}
}

func (a *businessApplier) Supports(entityType EntityType) bool {
	switch entityType {
	case EntityTypeConversation, EntityTypeMessage, EntityTypeCharacter, EntityTypeSettings:
		return true
	}
	return false
}

func (a *businessApplier) Apply(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	switch mutation.EntityType {
	case EntityTypeConversation:
		return a.applyConversation(tx, mutation)
	case EntityTypeMessage:
		return a.applyMessage(tx, mutation)
	case EntityTypeCharacter:
		return a.applyCharacter(tx, mutation)
	case EntityTypeSettings:
		return a.applySettings(tx, mutation)
	}
	return 0, &ApplierError{
		Code:    "unsupported_entity_type",
		Message: "unsupported entity type: " + string(mutation.EntityType),
	}
}

func (a *businessApplier) applyConversation(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	var payload mutationPayload
	if len(mutation.Payload) > 0 {
		if err := json.Unmarshal(mutation.Payload, &payload); err != nil {
			return 0, &ApplierError{Code: "invalid_payload", Message: "unmarshal conversation payload: " + err.Error()}
		}
	}
	switch mutation.Operation {
	case OpCreate:
		record := map[string]interface{}{
			"id":         string(mutation.EntityID),
			"title":      payload.Title,
			"created_at": a.now(),
			"updated_at": a.now(),
			"revision":   1,
		}
		if err := tx.Table("conversations").Create(record).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "create conversation: " + err.Error()}
		}
		return 1, nil
	case OpUpdate:
		updates := map[string]interface{}{
			"updated_at": a.now(),
			"revision":   gorm.Expr("COALESCE(revision, 0) + 1"),
		}
		if payload.Title != "" {
			updates["title"] = payload.Title
		}
		result := tx.Table("conversations").Where("id = ? AND revision = ?", mutation.EntityID, mutation.BaseRevision).Updates(updates)
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update conversation: " + result.Error.Error()}
		}
		if result.RowsAffected == 0 {
			var currentRev int64
			tx.Table("conversations").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&currentRev)
			return 0, &ApplierError{Code: "conflict", Message: "conversation revision mismatch", ServerRevision: currentRev}
		}
		var rev int64
		if err := tx.Table("conversations").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		result := tx.Table("conversations").Where("id = ? AND revision = ?", mutation.EntityID, mutation.BaseRevision).Updates(map[string]interface{}{
			"deleted_at": a.now(),
			"updated_at": a.now(),
			"revision":   gorm.Expr("COALESCE(revision, 0) + 1"),
		})
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete conversation: " + result.Error.Error()}
		}
		if result.RowsAffected == 0 {
			var currentRev int64
			tx.Table("conversations").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&currentRev)
			return 0, &ApplierError{Code: "conflict", Message: "conversation revision mismatch", ServerRevision: currentRev}
		}
		var rev int64
		if err := tx.Table("conversations").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read revision: " + err.Error()}
		}
		return rev, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) applyMessage(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	var payload mutationPayload
	if len(mutation.Payload) > 0 {
		if err := json.Unmarshal(mutation.Payload, &payload); err != nil {
			return 0, &ApplierError{Code: "invalid_payload", Message: "unmarshal message payload: " + err.Error()}
		}
	}
	switch mutation.Operation {
	case OpCreate:
		if payload.ConversationID == "" {
			return 0, &ApplierError{Code: "missing_required_field", Message: "message conversationId is required"}
		}
		if payload.Role == "" {
			return 0, &ApplierError{Code: "missing_required_field", Message: "message role is required"}
		}
		record := map[string]interface{}{
			"id":              string(mutation.EntityID),
			"conversation_id": payload.ConversationID,
			"role":            payload.Role,
			"content":         payload.Content,
			"sequence":        payload.Sequence,
			"msg_type":        payload.MsgType,
			"source":          payload.Source,
			"created_at":      a.now(),
			"updated_at":      a.now(),
			"revision":        1,
		}
		if err := tx.Table("messages").Create(record).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "create message: " + err.Error()}
		}
		return 1, nil
	case OpUpdate:
		updates := map[string]interface{}{
			"updated_at": a.now(),
			"revision":   gorm.Expr("COALESCE(revision, 0) + 1"),
		}
		if payload.Content != "" {
			updates["content"] = payload.Content
		}
		result := tx.Table("messages").Where("id = ? AND revision = ?", mutation.EntityID, mutation.BaseRevision).Updates(updates)
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update message: " + result.Error.Error()}
		}
		if result.RowsAffected == 0 {
			var currentRev int64
			tx.Table("messages").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&currentRev)
			return 0, &ApplierError{Code: "conflict", Message: "message revision mismatch", ServerRevision: currentRev}
		}
		var rev int64
		if err := tx.Table("messages").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		result := tx.Table("messages").Where("id = ? AND revision = ?", mutation.EntityID, mutation.BaseRevision).Updates(map[string]interface{}{
			"deleted_at": a.now(),
			"updated_at": a.now(),
			"revision":   gorm.Expr("COALESCE(revision, 0) + 1"),
		})
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete message: " + result.Error.Error()}
		}
		if result.RowsAffected == 0 {
			var currentRev int64
			tx.Table("messages").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&currentRev)
			return 0, &ApplierError{Code: "conflict", Message: "message revision mismatch", ServerRevision: currentRev}
		}
		var rev int64
		if err := tx.Table("messages").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read revision: " + err.Error()}
		}
		return rev, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) applyCharacter(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	var payload mutationPayload
	if len(mutation.Payload) > 0 {
		if err := json.Unmarshal(mutation.Payload, &payload); err != nil {
			return 0, &ApplierError{Code: "invalid_payload", Message: "unmarshal character payload: " + err.Error()}
		}
	}
	switch mutation.Operation {
	case OpCreate:
		record := map[string]interface{}{
			"id":         string(mutation.EntityID),
			"title":      payload.Title,
			"created_at": a.now(),
			"updated_at": a.now(),
			"revision":   1,
		}
		if err := tx.Table("characters").Create(record).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "create character: " + err.Error()}
		}
		return 1, nil
	case OpUpdate:
		updates := map[string]interface{}{
			"updated_at": a.now(),
			"revision":   gorm.Expr("COALESCE(revision, 0) + 1"),
		}
		if payload.Title != "" {
			updates["title"] = payload.Title
		}
		result := tx.Table("characters").Where("id = ? AND revision = ?", mutation.EntityID, mutation.BaseRevision).Updates(updates)
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update character: " + result.Error.Error()}
		}
		if result.RowsAffected == 0 {
			var currentRev int64
			tx.Table("characters").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&currentRev)
			return 0, &ApplierError{Code: "conflict", Message: "character revision mismatch", ServerRevision: currentRev}
		}
		var rev int64
		if err := tx.Table("characters").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		result := tx.Table("characters").Where("id = ? AND revision = ?", mutation.EntityID, mutation.BaseRevision).Updates(map[string]interface{}{
			"deleted_at": a.now(),
			"updated_at": a.now(),
			"revision":   gorm.Expr("COALESCE(revision, 0) + 1"),
		})
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete character: " + result.Error.Error()}
		}
		if result.RowsAffected == 0 {
			var currentRev int64
			tx.Table("characters").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&currentRev)
			return 0, &ApplierError{Code: "conflict", Message: "character revision mismatch", ServerRevision: currentRev}
		}
		var rev int64
		if err := tx.Table("characters").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0)").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read revision: " + err.Error()}
		}
		return rev, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) applySettings(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	var payload mutationPayload
	if len(mutation.Payload) > 0 {
		if err := json.Unmarshal(mutation.Payload, &payload); err != nil {
			return 0, &ApplierError{Code: "invalid_payload", Message: "unmarshal settings payload: " + err.Error()}
		}
	}
	settingsKey := string(mutation.EntityID)
	if payload.Key != "" {
		settingsKey = payload.Key
	}
	switch mutation.Operation {
	case OpCreate, OpUpdate:
		var existing struct {
			Key string
		}
		tx.Table("app_settings").Where("key = ?", settingsKey).Select("key").Scan(&existing)
		if existing.Key == "" {
			if err := tx.Table("app_settings").Create(map[string]interface{}{
				"key":        settingsKey,
				"value":      payload.Value,
				"updated_at": a.now(),
			}).Error; err != nil {
				return 0, &ApplierError{Code: "apply_failed", Message: "create settings: " + err.Error()}
			}
		} else {
			if err := tx.Table("app_settings").Where("key = ?", settingsKey).Updates(map[string]interface{}{
				"value":      payload.Value,
				"updated_at": a.now(),
			}).Error; err != nil {
				return 0, &ApplierError{Code: "apply_failed", Message: "update settings: " + err.Error()}
			}
		}
		return 0, nil
	case OpDelete:
		if err := tx.Table("app_settings").Where("key = ?", settingsKey).Delete(nil).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete settings: " + err.Error()}
		}
		return 0, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) now() string {
	return a.db.NowFunc().Format("2006-01-02 15:04:05")
}
