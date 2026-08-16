// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

const (
	EntityTypeConversation EntityType = "conversation"
	EntityTypeMessage      EntityType = "message"
	EntityTypeCharacter    EntityType = "character"
	EntityTypeSettings     EntityType = "settings"
)

type mutationPayload struct {
	Title   string                 `json:"title"`
	Content string                 `json:"content"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
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

func (a *businessApplier) Apply(mutation ClientMutation) (int64, error) {
	switch mutation.EntityType {
	case EntityTypeConversation:
		return a.applyConversation(mutation)
	case EntityTypeMessage:
		return a.applyMessage(mutation)
	case EntityTypeCharacter:
		return a.applyCharacter(mutation)
	case EntityTypeSettings:
		return a.applySettings(mutation)
	}
	return 0, &ApplierError{
		Code:    "unsupported_entity_type",
		Message: "unsupported entity type: " + string(mutation.EntityType),
	}
}

func (a *businessApplier) applyConversation(mutation ClientMutation) (int64, error) {
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
		if err := a.db.Table("conversations").Create(record).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "create conversation: " + err.Error()}
		}
		return 1, nil
	case OpUpdate:
		updates := map[string]interface{}{
			"updated_at": a.now(),
		}
		if payload.Title != "" {
			updates["title"] = payload.Title
		}
		result := a.db.Table("conversations").Where("id = ?", mutation.EntityID).Updates(updates)
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update conversation: " + result.Error.Error()}
		}
		var rev int64
		if err := a.db.Table("conversations").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0) + 1").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "bump revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		if err := a.db.Table("conversations").Where("id = ?", mutation.EntityID).Delete(nil).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete conversation: " + err.Error()}
		}
		return mutation.BaseRevision + 1, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) applyMessage(mutation ClientMutation) (int64, error) {
	var payload mutationPayload
	if len(mutation.Payload) > 0 {
		if err := json.Unmarshal(mutation.Payload, &payload); err != nil {
			return 0, &ApplierError{Code: "invalid_payload", Message: "unmarshal message payload: " + err.Error()}
		}
	}
	switch mutation.Operation {
	case OpCreate:
		record := map[string]interface{}{
			"id":         string(mutation.EntityID),
			"content":    payload.Content,
			"created_at": a.now(),
			"updated_at": a.now(),
			"revision":   1,
		}
		if err := a.db.Table("messages").Create(record).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "create message: " + err.Error()}
		}
		return 1, nil
	case OpUpdate:
		updates := map[string]interface{}{
			"updated_at": a.now(),
		}
		if payload.Content != "" {
			updates["content"] = payload.Content
		}
		result := a.db.Table("messages").Where("id = ?", mutation.EntityID).Updates(updates)
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update message: " + result.Error.Error()}
		}
		var rev int64
		if err := a.db.Table("messages").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0) + 1").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "bump revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		if err := a.db.Table("messages").Where("id = ?", mutation.EntityID).Delete(nil).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete message: " + err.Error()}
		}
		return mutation.BaseRevision + 1, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) applyCharacter(mutation ClientMutation) (int64, error) {
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
		if err := a.db.Table("characters").Create(record).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "create character: " + err.Error()}
		}
		return 1, nil
	case OpUpdate:
		updates := map[string]interface{}{
			"updated_at": a.now(),
		}
		if payload.Title != "" {
			updates["title"] = payload.Title
		}
		result := a.db.Table("characters").Where("id = ?", mutation.EntityID).Updates(updates)
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update character: " + result.Error.Error()}
		}
		var rev int64
		if err := a.db.Table("characters").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0) + 1").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "bump revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		if err := a.db.Table("characters").Where("id = ?", mutation.EntityID).Delete(nil).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete character: " + err.Error()}
		}
		return mutation.BaseRevision + 1, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) applySettings(mutation ClientMutation) (int64, error) {
	var payload mutationPayload
	if len(mutation.Payload) > 0 {
		if err := json.Unmarshal(mutation.Payload, &payload); err != nil {
			return 0, &ApplierError{Code: "invalid_payload", Message: "unmarshal settings payload: " + err.Error()}
		}
	}
	switch mutation.Operation {
	case OpCreate, OpUpdate:
		upsert := map[string]interface{}{
			"id":         string(mutation.EntityID),
			"meta":       payload.Meta,
			"updated_at": a.now(),
		}
		if err := a.db.Table("settings").Save(upsert).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "upsert settings: " + err.Error()}
		}
		var rev int64
		if err := a.db.Table("settings").Where("id = ?", mutation.EntityID).Select("COALESCE(revision, 0) + 1").Scan(&rev).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "bump revision: " + err.Error()}
		}
		return rev, nil
	case OpDelete:
		if err := a.db.Table("settings").Where("id = ?", mutation.EntityID).Delete(nil).Error; err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete settings: " + err.Error()}
		}
		return mutation.BaseRevision + 1, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) now() string {
	return fmt.Sprintf("%d", a.db.NowFunc().Unix())
}
