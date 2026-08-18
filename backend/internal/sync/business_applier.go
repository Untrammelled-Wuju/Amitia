// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"encoding/json"
	"errors"
	"time"

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
	Name           string                 `json:"name,omitempty"`
	Content        string                 `json:"content"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	ConversationID string                 `json:"conversationId,omitempty"`
	CharacterID    string                 `json:"characterId,omitempty"`
	Channel        string                 `json:"channel,omitempty"`
	Source         string                 `json:"source,omitempty"`
	PeerID         string                 `json:"peerId,omitempty"`
	Role           string                 `json:"role,omitempty"`
	Sequence       int64                  `json:"sequence,omitempty"`
	MsgType        string                 `json:"msgType,omitempty"`
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
			"id":           string(mutation.EntityID),
			"character_id": payload.CharacterID,
			"title":        payload.Title,
			"channel":      payload.Channel,
			"source":       payload.Source,
			"peer_id":      payload.PeerID,
			"created_at":   a.now(),
			"updated_at":   a.now(),
			"revision":     1,
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
		if payload.CharacterID != "" {
			updates["character_id"] = payload.CharacterID
		}
		if payload.Channel != "" {
			updates["channel"] = payload.Channel
		}
		if payload.Source != "" {
			updates["source"] = payload.Source
		}
		if payload.PeerID != "" {
			updates["peer_id"] = payload.PeerID
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
		name := payload.Name
		if name == "" {
			name = payload.Title
		}
		record := map[string]interface{}{
			"id":         string(mutation.EntityID),
			"name":       name,
			"created_at": a.now(),
			"updated_at": a.now(),
			"revision":   1,
		}
		for key, value := range sanitizeCharacterMeta(payload.Meta) {
			record[key] = value
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
		if payload.Name != "" {
			updates["name"] = payload.Name
		} else if payload.Title != "" {
			updates["name"] = payload.Title
		}
		for key, value := range sanitizeCharacterMeta(payload.Meta) {
			updates[key] = value
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

func sanitizeCharacterMeta(meta map[string]interface{}) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	allowed := map[string]struct{}{
		"name": {}, "identity": {}, "personality": {}, "speaking_style": {},
		"relationship_style": {}, "character_base": {}, "boundary_rules": {},
		"description": {}, "status": {}, "is_active": {}, "sort_order": {},
		"gender": {}, "pronoun": {}, "self_reference": {}, "gender_expression": {},
		"life_identity": {}, "voice_config_id": {}, "voice_type": {}, "voice_speed": {},
		"voice_pitch": {}, "voice_volume": {}, "custom_voice_id": {}, "voice_mode": {},
		"emotion": {}, "emotion_scale": {}, "silence_duration": {}, "personality_config": {},
		"is_default": {}, "chat_style_config": {}, "scene_rules": {}, "avatar": {},
		"conversation_id": {}, "gender_label": {}, "user_addressing_style": {},
		"base_prompt": {}, "generated_prompt": {}, "personality_sliders": {}, "card_data_json": {},
	}
	out := make(map[string]interface{}, len(meta))
	for key, value := range meta {
		if _, ok := allowed[key]; ok {
			out[key] = value
		}
	}
	return out
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
	type settingRevision struct {
		Revision  int64
		DeletedAt *time.Time
	}
	readCurrent := func() (settingRevision, bool, error) {
		var current settingRevision
		err := tx.Table("app_settings").Where("key = ?", settingsKey).Select("revision", "deleted_at").Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return settingRevision{}, false, nil
		}
		if err != nil {
			return settingRevision{}, false, err
		}
		return current, true, nil
	}
	conflict := func(current int64, message string) (int64, error) {
		return 0, &ApplierError{Code: "conflict", Message: message, ServerRevision: current}
	}
	switch mutation.Operation {
	case OpCreate:
		current, exists, err := readCurrent()
		if err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read settings: " + err.Error()}
		}
		if !exists {
			if mutation.BaseRevision != 0 {
				return conflict(0, "settings base revision mismatch")
			}
			if err := tx.Table("app_settings").Create(map[string]interface{}{
				"key":        settingsKey,
				"value":      payload.Value,
				"revision":   1,
				"deleted_at": nil,
				"updated_at": a.now(),
			}).Error; err != nil {
				return 0, &ApplierError{Code: "apply_failed", Message: "create settings: " + err.Error()}
			}
			return 1, nil
		}
		if current.DeletedAt == nil || current.Revision != mutation.BaseRevision {
			return conflict(current.Revision, "settings already exists or base revision mismatch")
		}
		result := tx.Table("app_settings").Where("key = ? AND revision = ? AND deleted_at IS NOT NULL", settingsKey, mutation.BaseRevision).Updates(map[string]interface{}{
			"value":      payload.Value,
			"revision":   gorm.Expr("revision + 1"),
			"deleted_at": nil,
			"updated_at": a.now(),
		})
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "restore settings: " + result.Error.Error()}
		}
		if result.RowsAffected != 1 {
			latest, _, latestErr := readCurrent()
			if latestErr != nil {
				return 0, &ApplierError{Code: "apply_failed", Message: "read settings after restore conflict: " + latestErr.Error()}
			}
			return conflict(latest.Revision, "settings restore revision mismatch")
		}
		return mutation.BaseRevision + 1, nil
	case OpUpdate:
		current, exists, err := readCurrent()
		if err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read settings: " + err.Error()}
		}
		if !exists {
			return conflict(0, "settings does not exist")
		}
		if current.DeletedAt != nil || current.Revision != mutation.BaseRevision {
			return conflict(current.Revision, "settings revision mismatch")
		}
		result := tx.Table("app_settings").Where("key = ? AND revision = ? AND deleted_at IS NULL", settingsKey, mutation.BaseRevision).Updates(map[string]interface{}{
			"value":      payload.Value,
			"revision":   gorm.Expr("revision + 1"),
			"updated_at": a.now(),
		})
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "update settings: " + result.Error.Error()}
		}
		if result.RowsAffected != 1 {
			latest, _, latestErr := readCurrent()
			if latestErr != nil {
				return 0, &ApplierError{Code: "apply_failed", Message: "read settings after conflict: " + latestErr.Error()}
			}
			return conflict(latest.Revision, "settings revision mismatch")
		}
		return mutation.BaseRevision + 1, nil
	case OpDelete:
		current, exists, err := readCurrent()
		if err != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "read settings: " + err.Error()}
		}
		if !exists {
			return conflict(0, "settings does not exist")
		}
		if current.DeletedAt != nil || current.Revision != mutation.BaseRevision {
			return conflict(current.Revision, "settings revision mismatch")
		}
		now := a.now()
		result := tx.Table("app_settings").Where("key = ? AND revision = ? AND deleted_at IS NULL", settingsKey, mutation.BaseRevision).Updates(map[string]interface{}{
			"value":      "",
			"revision":   gorm.Expr("revision + 1"),
			"deleted_at": now,
			"updated_at": now,
		})
		if result.Error != nil {
			return 0, &ApplierError{Code: "apply_failed", Message: "delete settings: " + result.Error.Error()}
		}
		if result.RowsAffected != 1 {
			latest, _, latestErr := readCurrent()
			if latestErr != nil {
				return 0, &ApplierError{Code: "apply_failed", Message: "read settings after delete conflict: " + latestErr.Error()}
			}
			return conflict(latest.Revision, "settings revision mismatch")
		}
		return mutation.BaseRevision + 1, nil
	}
	return 0, &ApplierError{Code: "unsupported_operation", Message: "unsupported operation: " + string(mutation.Operation)}
}

func (a *businessApplier) now() string {
	return a.db.NowFunc().Format("2006-01-02 15:04:05")
}
