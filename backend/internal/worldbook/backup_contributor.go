// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const ComponentIDWorldbookRecords = "worldbook.records"

type WorldbookBackupContributor struct {
	DB *gorm.DB
}

func NewWorldbookBackupContributor(db *gorm.DB) *WorldbookBackupContributor {
	return &WorldbookBackupContributor{DB: db}
}

func (w *WorldbookBackupContributor) ID() string   { return "worldbook" }
func (w *WorldbookBackupContributor) Name() string { return "Worldbook" }

func (w *WorldbookBackupContributor) Dependencies() []string {
	return []string{"character"}
}

type worldbookRecordV1 struct {
	ID            string `json:"id"`
	MatchType     string `json:"matchType"`
	MatchPattern  string `json:"matchPattern"`
	MatchScope    string `json:"matchScope"`
	InjectContent string `json:"injectContent"`
	Priority      int    `json:"priority"`
	HitCount      int    `json:"hitCount"`
	CharacterID   string `json:"characterId"`
	ConfigJSON    string `json:"configJson"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func (w *WorldbookBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	if req.Scope == dataportability.ScopeCharacter && req.CharacterID != "" {
		w.DB.WithContext(ctx).Model(&WorldBookEntry{}).Where("character_id = ?", req.CharacterID).Count(&count)
	} else {
		w.DB.WithContext(ctx).Model(&WorldBookEntry{}).Count(&count)
	}

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDWorldbookRecords,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "worldbook.records.v1",
			Required:      false,
			SourceOfTruth: false,
			ItemCount:     count,
			EstimatedSize: count * 1024,
		},
	}, nil
}

func (w *WorldbookBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDWorldbookRecords, "worldbook.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := w.DB.WithContext(ctx).Table("world_book").Select(
		"id, match_type, match_pattern, match_scope, inject_content, priority, hit_count, character_id, config_json, created_at, updated_at",
	)
	if req.Scope == dataportability.ScopeCharacter && req.CharacterID != "" {
		query = query.Where("character_id = ?", req.CharacterID)
	}

	rows, err := query.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	bw := bufio.NewWriter(compW)
	for rows.Next() {
		var rec worldbookRecordV1
		if err := w.DB.ScanRows(rows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		bw.Write(data)
		bw.WriteByte('\n')
	}
	bw.Flush()
	return rows.Err()
}

func (w *WorldbookBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDWorldbookRecords)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDWorldbookRecords,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "worldbook.records.v1",
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec worldbookRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct{ ID string }
		w.DB.WithContext(ctx).Table("world_book").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.ID,
				TargetID:   existing.ID,
				EntityType: "worldbook_entry",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (w *WorldbookBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(ComponentIDWorldbookRecords)
	if err != nil {
		return err
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec worldbookRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		newID := rec.ID
		var existing struct{ ID string }
		w.DB.WithContext(ctx).Table("world_book").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				newID = rec.ID
			default:
				newID = uuid.New().String()
			}
		}

		entry := WorldBookEntry{
			ID:            newID,
			MatchType:     rec.MatchType,
			MatchPattern:  rec.MatchPattern,
			MatchScope:    rec.MatchScope,
			InjectContent: rec.InjectContent,
			Priority:      rec.Priority,
			HitCount:      rec.HitCount,
			CharacterID:   rec.CharacterID,
			ConfigJSON:    rec.ConfigJSON,
			CreatedAt:     rec.CreatedAt,
			UpdatedAt:     rec.UpdatedAt,
		}

		if newID == rec.ID && existing.ID != "" {
			w.DB.WithContext(ctx).Model(&WorldBookEntry{}).Where("id = ?", rec.ID).Updates(map[string]interface{}{
				"match_type":     entry.MatchType,
				"match_pattern":  entry.MatchPattern,
				"match_scope":    entry.MatchScope,
				"inject_content": entry.InjectContent,
				"priority":       entry.Priority,
				"hit_count":      entry.HitCount,
				"character_id":   entry.CharacterID,
				"config_json":    entry.ConfigJSON,
				"created_at":     entry.CreatedAt,
				"updated_at":     entry.UpdatedAt,
			})
		} else {
			w.DB.WithContext(ctx).Create(&entry)
		}
	}

	return nil
}
