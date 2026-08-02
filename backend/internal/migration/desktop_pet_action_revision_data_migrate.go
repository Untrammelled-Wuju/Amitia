package migration

import (
	"fmt"
	"time"
)

func DesktopPetActionRevisionDataMigrateMigration() Migration {
	return Migration{
		Version: "202607310020",
		Name:    "migrate_legacy_action_revision_data_to_stream",
		Up: func(s *Step) error {
			db := s.DB()
			if db == nil {
				return nil
			}

			type legacyRevision struct {
				ID               string `gorm:"column:id"`
				UserID           string `gorm:"column:user_id"`
				CharacterID      string `gorm:"column:character_id"`
				ActionKey        string `gorm:"column:action_key"`
				ProcessingTaskID string `gorm:"column:processing_task_id"`
				RevisionNumber   int    `gorm:"column:revision_number"`
			}

			var revs []legacyRevision
			if err := db.Table("desktop_pet_action_revisions").
				Where("action_stream_id = '' OR action_stream_id IS NULL").
				Find(&revs).Error; err != nil {
				return fmt.Errorf("查询无Stream的Revision失败: %w", err)
			}

			streamCache := make(map[string]string)
			now := time.Now().UTC().Format(time.RFC3339)

			for _, rev := range revs {
				streamKey := fmt.Sprintf("%s:%s:%s", rev.UserID, rev.CharacterID, rev.ActionKey)

				streamID, exists := streamCache[streamKey]
				if !exists {
					var existingID struct {
						ID string `gorm:"column:id"`
					}
					err := db.Table("desktop_pet_action_streams").
						Where("stream_key = ?", streamKey).
						Select("id").Scan(&existingID).Error
					if err == nil && existingID.ID != "" {
						streamID = existingID.ID
					} else {
						streamID = fmt.Sprintf("as-mig-%d", time.Now().UnixNano())
						if err := db.Exec(`INSERT INTO desktop_pet_action_streams 
							(id, user_id, character_id, action_key, root_processing_task_id, stream_key, next_revision_number, created_at, updated_at) 
							VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
							streamID, rev.UserID, rev.CharacterID, rev.ActionKey,
							rev.ProcessingTaskID, streamKey, now, now).Error; err != nil {
							var fallbackID struct {
								ID string `gorm:"column:id"`
							}
							_ = db.Table("desktop_pet_action_streams").
								Where("stream_key = ?", streamKey).
								Select("id").Scan(&fallbackID).Error
							if fallbackID.ID != "" {
								streamID = fallbackID.ID
							} else {
								continue
							}
						}
					}
					streamCache[streamKey] = streamID
				}

				if err := db.Exec("UPDATE desktop_pet_action_revisions SET action_stream_id = ?, root_action_revision_id = CASE WHEN root_action_revision_id = '' THEN id ELSE root_action_revision_id END, parent_action_revision_id = CASE WHEN parent_action_revision_id = '' THEN '' ELSE parent_action_revision_id END WHERE id = ?",
					streamID, rev.ID).Error; err != nil {
					return fmt.Errorf("更新Revision StreamID失败(rev=%s): %w", rev.ID, err)
				}

				if err := db.Exec(`INSERT INTO desktop_pet_legacy_revision_mappings (id, legacy_revision_id, new_action_revision_id, action_stream_id, legacy_revision_number, migrated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					fmt.Sprintf("lrm-%d", time.Now().UnixNano()), rev.ID, rev.ID, streamID, rev.RevisionNumber, now).Error; err != nil {
				}
			}

			type legacyBinding struct {
				ProcessingTaskID string `gorm:"column:processing_task_id"`
				ActionKey        string `gorm:"column:action_key"`
				RevisionID       string `gorm:"column:revision_id"`
				BindingVersion   int64  `gorm:"column:binding_version"`
				UserID           string `gorm:"column:user_id"`
				CharacterID      string `gorm:"column:character_id"`
			}

			var bindings []legacyBinding
			if err := db.Table("desktop_pet_action_active_revisions").
				Find(&bindings).Error; err != nil {
				return fmt.Errorf("查询旧Binding失败: %w", err)
			}

			for _, lb := range bindings {
				var streamID string
				if lb.UserID != "" && lb.CharacterID != "" {
					streamKey := fmt.Sprintf("%s:%s:%s", lb.UserID, lb.CharacterID, lb.ActionKey)
					var stream struct {
						ID string `gorm:"column:id"`
					}
					_ = db.Table("desktop_pet_action_streams").
						Where("stream_key = ?", streamKey).
						Select("id").Scan(&stream).Error
					streamID = stream.ID
				}

				if streamID == "" {
					var rev struct {
						ActionStreamID string `gorm:"column:action_stream_id"`
					}
					_ = db.Table("desktop_pet_action_revisions").
						Where("id = ?", lb.RevisionID).
						Select("action_stream_id").Scan(&rev).Error
					streamID = rev.ActionStreamID
				}

				if streamID == "" {
					continue
				}

				var count int64
				db.Table("desktop_pet_active_action_revision_bindings").
					Where("action_stream_id = ?", streamID).Count(&count)
				if count > 0 {
					continue
				}

				bindingID := fmt.Sprintf("ab-mig-%d", time.Now().UnixNano())
				if err := db.Exec(`INSERT INTO desktop_pet_active_action_revision_bindings 
					(id, action_stream_id, user_id, character_id, action_key, active_action_revision_id, binding_revision, bound_reason, bound_by, bound_at, created_at, updated_at) 
					VALUES (?, ?, ?, ?, ?, ?, ?, 'legacy_migration', 'system', ?, ?, ?)`,
					bindingID, streamID, lb.UserID, lb.CharacterID, lb.ActionKey,
					lb.RevisionID, lb.BindingVersion, now, now, now).Error; err != nil {
					continue
				}

				_ = db.Exec(`INSERT INTO desktop_pet_legacy_binding_mappings (id, legacy_processing_task_id, legacy_action_key, legacy_revision_id, new_binding_id, action_stream_id, migrated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
					fmt.Sprintf("lbm-%d", time.Now().UnixNano()), lb.ProcessingTaskID, lb.ActionKey, lb.RevisionID, bindingID, streamID, now)
			}

			return nil
		},
	}
}
