package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func migrateLegacyMessageFeedback(ctx context.Context, db *gorm.DB, spaceID, ownerID, coreID string) error {
	if db == nil || spaceID == "" || ownerID == "" || coreID == "" || !db.Migrator().HasTable("message_feedback") {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			ID        int
			MessageID string
			CreatedAt string
		}
		if err := tx.Table("message_feedback").Select("id,message_id,created_at").Where("owner_id='' OR owner_id IS NULL").Order("id").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			requestID := fmt.Sprintf("legacy-feedback/%d", row.ID)
			updatedAt := strings.TrimSpace(row.CreatedAt)
			if updatedAt == "" {
				updatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			}
			var message struct {
				RoleID         string
				ConversationID string
				SpaceID        string
			}
			err := tx.Table("messages AS m").
				Joins("JOIN conversations AS c ON c.id=m.conversation_id").
				Select("m.character_id AS role_id,m.conversation_id,c.space_id").
				Where("m.id=? AND m.deleted_at IS NULL AND c.deleted_at IS NULL", row.MessageID).
				Take(&message).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if err == nil && message.RoleID != "" && message.ConversationID != "" {
				if err := tx.Table("message_feedback").Where("id=? AND (owner_id='' OR owner_id IS NULL)", row.ID).Updates(map[string]any{
					"space_id":        spaceID,
					"owner_id":        ownerID,
					"core_id":         coreID,
					"role_id":         message.RoleID,
					"conversation_id": message.ConversationID,
					"request_id":      requestID,
					"revision":        1,
					"updated_at":      updatedAt,
				}).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Table("message_feedback").Where("id=? AND (owner_id='' OR owner_id IS NULL)", row.ID).Updates(map[string]any{
				"space_id":   "legacy-unscoped",
				"owner_id":   "legacy-unscoped",
				"core_id":    "legacy-unscoped",
				"request_id": requestID,
				"revision":   1,
				"updated_at": updatedAt,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
