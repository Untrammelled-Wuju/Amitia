package migration

import (
	"fmt"
	"time"
)

func DesktopPetActionRevisionDataMigrateMigration() Migration {
	return Migration{
		Version: "202607310020",
		Name:    "migrate_legacy_action_revision_data_to_stream",
		// Up performs direct data migration through Step.DB() and records no
		// schema operations. Re-running Up merely to calculate a checksum would
		// mutate an already-migrated database outside the migration transaction.
		// The applied checksum for this migration is therefore intentionally the
		// checksum of an empty operation list.
		ChecksumUp: func(_ *Step) error { return nil },
		Up: func(s *Step) error {
			exists, err := s.TableExists("desktop_pet_action_revisions")
			if err != nil {
				return err
			}
			if !exists {
				return nil
			}

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

				streamID, cached := streamCache[streamKey]
				if !cached {
					var existingID struct {
						ID string `gorm:"column:id"`
					}
					if err := db.Table("desktop_pet_action_streams").
						Where("stream_key = ?", streamKey).
						Select("id").Scan(&existingID).Error; err != nil {
						return fmt.Errorf("查询Action Stream失败(stream=%s): %w", streamKey, err)
					}
					if existingID.ID != "" {
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
							if lookupErr := db.Table("desktop_pet_action_streams").
								Where("stream_key = ?", streamKey).
								Select("id").Scan(&fallbackID).Error; lookupErr != nil {
								return fmt.Errorf("创建Action Stream失败且回查失败(stream=%s): create=%v lookup=%w", streamKey, err, lookupErr)
							}
							if fallbackID.ID == "" {
								return fmt.Errorf("创建Action Stream失败且未找到并发创建结果(stream=%s): %w", streamKey, err)
							}
							streamID = fallbackID.ID
						}
					}
					streamCache[streamKey] = streamID
				}

				if err := db.Exec("UPDATE desktop_pet_action_revisions SET action_stream_id = ?, root_action_revision_id = CASE WHEN root_action_revision_id = '' THEN id ELSE root_action_revision_id END, parent_action_revision_id = CASE WHEN parent_action_revision_id = '' THEN '' ELSE parent_action_revision_id END WHERE id = ?",
					streamID, rev.ID).Error; err != nil {
					return fmt.Errorf("更新Revision StreamID失败(rev=%s): %w", rev.ID, err)
				}

				if err := db.Exec(`INSERT OR IGNORE INTO desktop_pet_legacy_revision_mappings
					(id, legacy_revision_id, new_action_revision_id, action_stream_id, legacy_revision_number, migrated_at)
					VALUES (?, ?, ?, ?, ?, ?)`,
					fmt.Sprintf("lrm-%d", time.Now().UnixNano()), rev.ID, rev.ID, streamID, rev.RevisionNumber, now).Error; err != nil {
					return fmt.Errorf("写入Revision迁移映射失败(rev=%s): %w", rev.ID, err)
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
			if err := db.Table("desktop_pet_action_active_revisions").Find(&bindings).Error; err != nil {
				return fmt.Errorf("查询旧Binding失败: %w", err)
			}

			for _, lb := range bindings {
				var streamID string
				if lb.UserID != "" && lb.CharacterID != "" {
					streamKey := fmt.Sprintf("%s:%s:%s", lb.UserID, lb.CharacterID, lb.ActionKey)
					var stream struct {
						ID string `gorm:"column:id"`
					}
					if err := db.Table("desktop_pet_action_streams").
						Where("stream_key = ?", streamKey).
						Select("id").Scan(&stream).Error; err != nil {
						return fmt.Errorf("查询Binding对应Stream失败(stream=%s): %w", streamKey, err)
					}
					streamID = stream.ID
				}

				if streamID == "" {
					var rev struct {
						ActionStreamID string `gorm:"column:action_stream_id"`
					}
					if err := db.Table("desktop_pet_action_revisions").
						Where("id = ?", lb.RevisionID).
						Select("action_stream_id").Scan(&rev).Error; err != nil {
						return fmt.Errorf("按Revision查询Binding Stream失败(rev=%s): %w", lb.RevisionID, err)
					}
					streamID = rev.ActionStreamID
				}

				if streamID == "" {
					return fmt.Errorf("旧Binding无法解析Action Stream(task=%s action=%s rev=%s)", lb.ProcessingTaskID, lb.ActionKey, lb.RevisionID)
				}

				var count int64
				if err := db.Table("desktop_pet_active_action_revision_bindings").
					Where("action_stream_id = ?", streamID).Count(&count).Error; err != nil {
					return fmt.Errorf("查询现有Binding失败(stream=%s): %w", streamID, err)
				}
				if count > 0 {
					continue
				}

				bindingID := fmt.Sprintf("ab-mig-%d", time.Now().UnixNano())
				if err := db.Exec(`INSERT INTO desktop_pet_active_action_revision_bindings
					(id, action_stream_id, user_id, character_id, action_key, active_action_revision_id, binding_revision, bound_reason, bound_by, bound_at, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, 'legacy_migration', 'system', ?, ?, ?)`,
					bindingID, streamID, lb.UserID, lb.CharacterID, lb.ActionKey,
					lb.RevisionID, lb.BindingVersion, now, now, now).Error; err != nil {
					return fmt.Errorf("迁移Active Binding失败(stream=%s): %w", streamID, err)
				}

				if err := db.Exec(`INSERT OR IGNORE INTO desktop_pet_legacy_binding_mappings
					(id, legacy_processing_task_id, legacy_action_key, legacy_revision_id, new_binding_id, action_stream_id, migrated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?)`,
					fmt.Sprintf("lbm-%d", time.Now().UnixNano()), lb.ProcessingTaskID, lb.ActionKey, lb.RevisionID, bindingID, streamID, now).Error; err != nil {
					return fmt.Errorf("写入Binding迁移映射失败(task=%s action=%s): %w", lb.ProcessingTaskID, lb.ActionKey, err)
				}
			}

			return nil
		},
	}
}
