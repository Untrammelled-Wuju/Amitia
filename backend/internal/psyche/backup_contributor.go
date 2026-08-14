// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package psyche

import (
	"bufio"
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const ComponentIDPsycheStates = "psyche.records"

type PsycheBackupContributor struct {
	DB *gorm.DB
}

func NewPsycheBackupContributor(db *gorm.DB) *PsycheBackupContributor {
	return &PsycheBackupContributor{DB: db}
}

func (p *PsycheBackupContributor) ID() string   { return "psyche" }
func (p *PsycheBackupContributor) Name() string { return "Psyche State" }

type psycheStateV1 struct {
	CharacterID  string  `json:"characterId"`
	Version      string  `json:"version"`
	StateVersion int     `json:"stateVersion"`
	Emotion      string  `json:"emotion"`
	Mood         string  `json:"mood"`
	Stress       float64 `json:"stress"`
	Energy       float64 `json:"energy"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

func (p *PsycheBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	if req.Scope == dataportability.ScopeCharacter && req.CharacterID != "" {
		p.DB.WithContext(ctx).Model(&psycheStateRecord{}).Where("character_id = ?", req.CharacterID).Count(&count)
	} else {
		p.DB.WithContext(ctx).Model(&psycheStateRecord{}).Count(&count)
	}

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDPsycheStates,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "psyche.records.v1",
			Required:      false,
			SourceOfTruth: true,
			ItemCount:     count,
			EstimatedSize: count * 512,
		},
	}, nil
}

func (p *PsycheBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDPsycheStates, "psyche.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := p.DB.WithContext(ctx).Table("psyche_states").Select(
		"character_id, version, state_version, emotion, mood, stress, energy, created_at, updated_at",
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
		var rec psycheStateV1
		if err := p.DB.ScanRows(rows, &rec); err != nil {
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

func (p *PsycheBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDPsycheStates)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDPsycheStates,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "psyche.records.v1",
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec psycheStateV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct{ CharacterID string }
		p.DB.WithContext(ctx).Table("psyche_states").Select("character_id").Where("character_id = ?", rec.CharacterID).Scan(&existing)
		if existing.CharacterID != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.CharacterID,
				TargetID:   existing.CharacterID,
				EntityType: "psyche_state",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (p *PsycheBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(ComponentIDPsycheStates)
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
		var rec psycheStateV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		var existing struct{ CharacterID string }
		p.DB.WithContext(ctx).Table("psyche_states").Select("character_id").Where("character_id = ?", rec.CharacterID).Scan(&existing)

		if existing.CharacterID != "" {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				p.DB.WithContext(ctx).Model(&psycheStateRecord{}).Where("character_id = ?", rec.CharacterID).Updates(map[string]interface{}{
					"version":       rec.Version,
					"state_version": rec.StateVersion,
					"emotion":       rec.Emotion,
					"mood":          rec.Mood,
					"stress":        rec.Stress,
					"energy":        rec.Energy,
					"created_at":    rec.CreatedAt,
					"updated_at":    rec.UpdatedAt,
				})
				continue
			default:
				continue
			}
		}

		createdAt, _ := time.Parse(time.RFC3339, rec.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, rec.UpdatedAt)
		record := psycheStateRecord{
			CharacterID:  rec.CharacterID,
			Version:      rec.Version,
			StateVersion: rec.StateVersion,
			Emotion:      rec.Emotion,
			Mood:         rec.Mood,
			Stress:       rec.Stress,
			Energy:       rec.Energy,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		}
		if err := p.DB.WithContext(ctx).Create(&record).Error; err != nil {
			continue
		}
	}

	return nil
}
