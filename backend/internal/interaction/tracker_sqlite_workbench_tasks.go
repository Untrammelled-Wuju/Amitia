package interaction

import "context"

func (t *SQLiteInteractionTracker) ListRecentWorkbenchTaskRecords(ctx context.Context, scope InteractionScope, limit int) ([]*InteractionRecord, error) {
	scope = scope.Normalize()
	if scope.SpaceID == "" || scope.ConversationID == "" {
		return []*InteractionRecord{}, nil
	}
	if limit <= 0 || limit > 128 {
		limit = 128
	}
	var rows []InteractionRecordModel
	query := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("space_id = ? AND conversation_id = ? AND source <> ? AND recovery_descriptor_json <> ?", scope.SpaceID, scope.ConversationID, "multi_agent", "")
	if scope.CharacterID != "" {
		query = query.Where("character_id = ?", scope.CharacterID)
	}
	result := query.
		Order("updated_at DESC, id DESC").Limit(limit).Find(&rows)
	if result.Error != nil {
		return nil, result.Error
	}
	records := make([]*InteractionRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, modelToInteractionRecord(row))
	}
	return records, nil
}
