package chat

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	syncapi "github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

type ManagedHistorySelection struct {
	EraseContent          bool
	KeepConversations     bool
	ConversationRevisions map[string]int64
	MessageRevisions      map[string]int64
}

func DeleteOwnedManagedHistory(ctx context.Context, db *gorm.DB, configured Service, legacySpaceID, roleID string, selection ManagedHistorySelection) error {
	s := &service{db: db}
	if configured != nil {
		var ok bool
		s, ok = configured.(*service)
		if !ok {
			return errors.New("Owner 历史存储服务未接通")
		}
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Format("2006-01-02 15:04:05")
		for id, revision := range selection.MessageRevisions {
			var message Message
			if err := tx.Where("id=? AND character_id=? AND revision=? AND deleted_at IS NULL", id, roleID, revision).First(&message).Error; err != nil {
				return coordination.ErrResourceVersion
			}
			if _, ok := selection.ConversationRevisions[message.ConversationID]; !ok {
				return coordination.ErrWrongOwner
			}
			var attachments []MessageAttachment
			if err := tx.Where("message_id=?", id).Find(&attachments).Error; err != nil {
				return err
			}
			if err := s.removeAttachmentReferences(tx, attachments); err != nil {
				return err
			}
			if selection.EraseContent {
				message.Content, message.ReasoningContent, message.AltText, message.AudioUrl, message.ImageUrl, message.VideoUrl, message.OriginalAsset, message.FallbackAsset, message.CitationSnapshotJSON = "", "", "", "", "", "", "", "", "[]"
				message.ReplyToExcerpt = nil
				if err := tx.Model(&Message{}).Where("id=? AND revision=? AND character_id=?", id, revision, roleID).Updates(map[string]any{"content": "", "reasoning_content": "", "alt_text": "", "audio_url": "", "image_url": "", "video_url": "", "original_asset_reference": "", "fallback_asset_reference": "", "citation_snapshot_json": "[]", "reply_to_excerpt": nil}).Error; err != nil {
					return err
				}
			}
			result := tx.Model(&Message{}).Where("id=? AND revision=? AND character_id=? AND deleted_at IS NULL", id, revision, roleID).Updates(map[string]any{"deleted_at": now, "updated_at": now, "revision": revision + 1})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return coordination.ErrResourceVersion
			}
			if err := s.recordMessageChangeTx(tx, &message, syncapi.OpDelete, revision+1, legacySpaceID); err != nil {
				return err
			}
			if err := tx.Where("message_id=?", id).Delete(&MessageAttachment{}).Error; err != nil {
				return err
			}
		}
		for id, revision := range selection.ConversationRevisions {
			var conversation Conversation
			if err := tx.Where("id=? AND revision=? AND deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default')", id, revision, legacySpaceID).First(&conversation).Error; err != nil {
				return coordination.ErrResourceVersion
			}
			var remaining int64
			if err := tx.Model(&Message{}).Where("conversation_id=? AND deleted_at IS NULL", id).Count(&remaining).Error; err != nil {
				return err
			}
			changes := map[string]any{"message_count": remaining, "updated_at": now, "revision": revision + 1}
			operation := syncapi.OpUpdate
			if remaining == 0 && !selection.KeepConversations {
				if selection.EraseContent {
					changes["title"] = ""
					changes["peer_id"] = ""
				}
				changes["deleted_at"] = now
				operation = syncapi.OpDelete
				if err := tx.Where("conversation_id=?", id).Delete(&ConversationSummary{}).Error; err != nil {
					return err
				}
			}
			result := tx.Model(&Conversation{}).Where("id=? AND revision=? AND deleted_at IS NULL", id, revision).Updates(changes)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return coordination.ErrResourceVersion
			}
			if err := s.recordConversationChangeTx(tx, &conversation, operation, revision+1, legacySpaceID); err != nil {
				return err
			}
			if selection.EraseContent {
				if err := tx.Where("conversation_id=?", id).Delete(&ConversationSummary{}).Error; err != nil {
					return err
				}
			}
			if err := pipelinecheckpoint.New(tx).ResetConversation(id); err != nil {
				return err
			}
		}
		return coordination.ValidateCurrent(ctx)
	})
	return err
}
