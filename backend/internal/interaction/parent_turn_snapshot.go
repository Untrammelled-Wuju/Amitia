package interaction

import (
	"context"
)

func (t *SQLiteInteractionTracker) loadParentTurnConversationSnapshot(ctx context.Context, spaceID, conversationID string) (*ParentTurnConversationSnapshot, error) {
	if t == nil || t.db == nil || !t.db.Migrator().HasTable("conversations") {
		return nil, nil
	}
	var snapshot ParentTurnConversationSnapshot
	result := t.db.WithContext(ctx).Table("conversations").
		Select("model_config_id, reasoning_effort, reasoning_enabled, permission_mode, workspace_id, workspace_device_id").
		Where("id = ? AND space_id = ? AND deleted_at IS NULL", conversationID, spaceID).
		Limit(1).Find(&snapshot)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, nil
	}
	return &snapshot, nil
}
