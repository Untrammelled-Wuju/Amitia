// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package relationship

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const ComponentIDRelationshipRecords = "relationship.records"

type RelationshipBackupContributor struct {
	DB *gorm.DB
}

func NewRelationshipBackupContributor(db *gorm.DB) *RelationshipBackupContributor {
	return &RelationshipBackupContributor{DB: db}
}

func (r *RelationshipBackupContributor) ID() string   { return "relationship" }
func (r *RelationshipBackupContributor) Name() string { return "Relationship" }

type relationshipRecordV1 struct {
	ID               string  `json:"id"`
	CharacterID      string  `json:"characterId"`
	UserID           string  `json:"userId"`
	Channel          string  `json:"channel"`
	RelationType     string  `json:"relationType"`
	Trust            float64 `json:"trust"`
	Familiarity      float64 `json:"familiarity"`
	Security         float64 `json:"security"`
	Tension          float64 `json:"tension"`
	RepairConfidence float64 `json:"repairConfidence"`
	Boundary         float64 `json:"boundary"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

type relationshipStateDB struct {
	ID           string `gorm:"column:id"`
	CharacterID  string `gorm:"column:character_id"`
	UserID       string `gorm:"column:user_id"`
	Channel      string `gorm:"column:channel"`
	RelationType string `gorm:"column:relation_type"`
	RelationData string `gorm:"column:relation_data"`
	CreatedAt    string `gorm:"column:created_at"`
	UpdatedAt    string `gorm:"column:updated_at"`
}

func (r *RelationshipBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	if req.Scope == dataportability.ScopeCharacter && req.CharacterID != "" {
		r.DB.WithContext(ctx).Table("relationship_states").Where("character_id = ?", req.CharacterID).Count(&count)
	} else {
		r.DB.WithContext(ctx).Table("relationship_states").Count(&count)
	}

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDRelationshipRecords,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "relationship.records.v1",
			Required:      false,
			SourceOfTruth: false,
			ItemCount:     count,
			EstimatedSize: count * 512,
		},
	}, nil
}

func (r *RelationshipBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDRelationshipRecords, "relationship.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := r.DB.WithContext(ctx).Table("relationship_states").Select(
		"id, character_id, user_id, channel, relation_type, relation_data, created_at, updated_at",
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
		var dbRec relationshipStateDB
		if err := r.DB.ScanRows(rows, &dbRec); err != nil {
			continue
		}

		state := RelationshipState{}
		if dbRec.RelationData != "" {
			json.Unmarshal([]byte(dbRec.RelationData), &state)
		}

		rec := relationshipRecordV1{
			ID:               dbRec.ID,
			CharacterID:      dbRec.CharacterID,
			UserID:           dbRec.UserID,
			Channel:          dbRec.Channel,
			RelationType:     dbRec.RelationType,
			Trust:            state.Trust,
			Familiarity:      state.Familiarity,
			Security:         state.Security,
			Tension:          state.Tension,
			RepairConfidence: state.RepairConfidence,
			Boundary:         state.Boundary,
			CreatedAt:        dbRec.CreatedAt,
			UpdatedAt:        dbRec.UpdatedAt,
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

func (r *RelationshipBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDRelationshipRecords)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDRelationshipRecords,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "relationship.records.v1",
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec relationshipRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct{ ID string }
		r.DB.WithContext(ctx).Table("relationship_states").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.ID,
				TargetID:   existing.ID,
				EntityType: "relationship_state",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (r *RelationshipBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(ComponentIDRelationshipRecords)
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
		var rec relationshipRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		newID := rec.ID
		var existing struct{ ID string }
		r.DB.WithContext(ctx).Table("relationship_states").Select("id").Where("id = ?", rec.ID).Scan(&existing)
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

		state := RelationshipState{
			Trust:            rec.Trust,
			Familiarity:      rec.Familiarity,
			Security:         rec.Security,
			Tension:          rec.Tension,
			RepairConfidence: rec.RepairConfidence,
			Boundary:         rec.Boundary,
		}
		relDataBytes, _ := json.Marshal(state)

		if newID == rec.ID && existing.ID != "" {
			r.DB.WithContext(ctx).Table("relationship_states").Where("id = ?", rec.ID).Updates(map[string]interface{}{
				"character_id":  rec.CharacterID,
				"user_id":       rec.UserID,
				"channel":       rec.Channel,
				"relation_type": rec.RelationType,
				"relation_data": string(relDataBytes),
				"created_at":    rec.CreatedAt,
				"updated_at":    rec.UpdatedAt,
			})
		} else {
			r.DB.WithContext(ctx).Table("relationship_states").Create(map[string]interface{}{
				"id":            newID,
				"character_id":  rec.CharacterID,
				"user_id":       rec.UserID,
				"channel":       rec.Channel,
				"relation_type": rec.RelationType,
				"relation_data": string(relDataBytes),
				"created_at":    rec.CreatedAt,
				"updated_at":    rec.UpdatedAt,
			})
		}
	}

	return nil
}

var _ = fmt.Sprintf
var _ = io.EOF
