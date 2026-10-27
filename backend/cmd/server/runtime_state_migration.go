package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

type legacyRelationshipStateRow struct {
	CharacterID  string `gorm:"column:character_id"`
	RelationData string `gorm:"column:relation_data"`
	UpdatedAt    string `gorm:"column:updated_at"`
}

type legacyNeedStateRow struct {
	NeedKey      string  `gorm:"column:need_key"`
	CurrentValue float64 `gorm:"column:current_value"`
	Baseline     float64 `gorm:"column:baseline"`
	UpdatedAt    string  `gorm:"column:updated_at"`
}

func migrateLegacyRuntimeState(ctx context.Context, db *gorm.DB, kernelDB *sql.DB, spaceID, ownerID string) error {
	if db == nil || kernelDB == nil || spaceID == "" || ownerID == "" {
		return errors.New("角色运行状态迁移缺少数据库或归属信息")
	}
	store := coordination.NewOwnershipStore(kernelDB, ownerID)
	markerID := "runtime-state-migration/" + ownerID
	marker, err := store.Get(ctx, "checkpoint", markerID)
	if err != nil {
		return err
	}
	roleIDs, err := legacyRuntimeStateRoleIDs(db)
	if err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := migrateLegacyRuntimeStateRole(ctx, db, store, spaceID, ownerID, roleID); err != nil {
			return err
		}
	}
	if marker != nil && !marker.Deleted {
		return nil
	}
	encoded, err := json.Marshal(map[string]any{"ownerId": ownerID, "roles": len(roleIDs), "migratedAt": time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return err
	}
	scope := coordination.ExecutionScope{
		SpaceID:            spaceID,
		CoreID:             spaceID,
		AuthorizationRealm: spaceID,
		InitiatorDeviceID:  ownerID,
		TargetDeviceID:     ownerID,
		ResourceOwnerID:    ownerID,
		RoleOwnerID:        ownerID,
		RoleID:             ownerID,
		RequestID:          migrationRequestID("runtime-state-marker", ownerID),
	}
	if len(roleIDs) > 0 {
		scope.RoleID = roleIDs[0]
	}
	_, err = store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: markerID, RoleID: scope.RoleID, Body: encoded}}})
	return err
}

func legacyRuntimeStateRoleIDs(db *gorm.DB) ([]string, error) {
	rows, err := db.Raw(`
		SELECT character_id AS role_id FROM psyche_states WHERE character_id <> ''
		UNION
		SELECT character_id AS role_id FROM relationship_states WHERE character_id <> ''
		UNION
		SELECT character_id AS role_id FROM need_states WHERE character_id <> ''
		ORDER BY role_id`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		result = append(result, roleID)
	}
	return result, rows.Err()
}

func migrateLegacyRuntimeStateRole(ctx context.Context, db *gorm.DB, store *coordination.OwnershipStore, spaceID, ownerID, roleID string) error {
	mutations := make([]coordination.Mutation, 0, len(coordination.RuntimeStateKinds))
	psycheBody, psycheFound, err := legacyPsycheBody(db, roleID)
	if err != nil {
		return err
	}
	if psycheFound {
		id := coordination.RoleRuntimeStateID(coordination.RuntimeStateKindPsyche, roleID)
		existing, err := store.Get(ctx, coordination.RuntimeStateKindPsyche, id)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RoleID != roleID {
				return errors.New("角色心理状态资源归属冲突")
			}
		} else {
			body := psycheBody
			legacy, legacyErr := store.Get(ctx, coordination.RuntimeStateKindPsyche, coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindPsyche))
			if legacyErr != nil {
				return legacyErr
			}
			if legacy != nil && !legacy.Deleted && legacy.RoleID == roleID {
				body = legacy.Body
			}
			mutations = append(mutations, coordination.Mutation{Kind: coordination.RuntimeStateKindPsyche, ID: id, RoleID: roleID, Body: body})
		}
	}
	relationshipBody, relationshipFound, err := legacyRelationshipBody(db, roleID)
	if err != nil {
		return err
	}
	if relationshipFound {
		id := coordination.RoleRuntimeStateID(coordination.RuntimeStateKindRelationship, roleID)
		existing, err := store.Get(ctx, coordination.RuntimeStateKindRelationship, id)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RoleID != roleID {
				return errors.New("角色关系状态资源归属冲突")
			}
		} else {
			body := relationshipBody
			legacy, legacyErr := store.Get(ctx, coordination.RuntimeStateKindRelationship, coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindRelationship))
			if legacyErr != nil {
				return legacyErr
			}
			if legacy != nil && !legacy.Deleted && legacy.RoleID == roleID {
				body = legacy.Body
			}
			mutations = append(mutations, coordination.Mutation{Kind: coordination.RuntimeStateKindRelationship, ID: id, RoleID: roleID, Body: body})
		}
	}
	needBody, needFound, err := legacyNeedBody(db, roleID)
	if err != nil {
		return err
	}
	if needFound {
		id := coordination.RoleRuntimeStateID(coordination.RuntimeStateKindNeeds, roleID)
		existing, err := store.Get(ctx, coordination.RuntimeStateKindNeeds, id)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RoleID != roleID {
				return errors.New("角色需求状态资源归属冲突")
			}
		} else {
			body := needBody
			legacy, legacyErr := store.Get(ctx, coordination.RuntimeStateKindNeeds, coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindNeeds))
			if legacyErr != nil {
				return legacyErr
			}
			if legacy != nil && !legacy.Deleted && legacy.RoleID == roleID {
				body = legacy.Body
			}
			mutations = append(mutations, coordination.Mutation{Kind: coordination.RuntimeStateKindNeeds, ID: id, RoleID: roleID, Body: body})
		}
	}
	if len(mutations) == 0 {
		return nil
	}
	scope := coordination.ExecutionScope{
		SpaceID:            spaceID,
		CoreID:             spaceID,
		AuthorizationRealm: spaceID,
		InitiatorDeviceID:  ownerID,
		TargetDeviceID:     ownerID,
		ResourceOwnerID:    ownerID,
		RoleOwnerID:        ownerID,
		RoleID:             roleID,
		RequestID:          migrationRequestID("runtime-state-role", ownerID+"\x00"+roleID),
	}
	_, err = store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: mutations})
	return err
}

func legacyPsycheBody(db *gorm.DB, roleID string) (json.RawMessage, bool, error) {
	var row struct {
		Emotion   string  `gorm:"column:emotion"`
		Mood      string  `gorm:"column:mood"`
		Stress    float64 `gorm:"column:stress"`
		Energy    float64 `gorm:"column:energy"`
		UpdatedAt string  `gorm:"column:updated_at"`
	}
	err := db.Table("psyche_states").Select("emotion, mood, stress, energy, updated_at").Where("character_id = ?", roleID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var emotion struct {
		Valence   float64 `json:"valence"`
		Arousal   float64 `json:"arousal"`
		Dominance float64 `json:"dominance"`
	}
	var mood struct {
		MoodValence float64 `json:"moodValence"`
		MoodArousal float64 `json:"moodArousal"`
	}
	if row.Emotion != "" {
		if err := json.Unmarshal([]byte(row.Emotion), &emotion); err != nil {
			return nil, false, err
		}
	}
	if row.Mood != "" {
		if err := json.Unmarshal([]byte(row.Mood), &mood); err != nil {
			return nil, false, err
		}
	}
	updatedAt := time.Now().UTC()
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, parseErr := time.Parse(layout, row.UpdatedAt); parseErr == nil {
			updatedAt = parsed
			break
		}
	}
	body, err := json.Marshal(map[string]any{
		"valence":         emotion.Valence,
		"dominance":       emotion.Dominance,
		"arousal":         emotion.Arousal,
		"moodValence":     mood.MoodValence,
		"moodArousal":     mood.MoodArousal,
		"stress":          row.Stress,
		"fatigue":         1 - row.Energy,
		"moodPressure":    0.5 + (0.5-mood.MoodValence)*0.5 + row.Stress*0.25,
		"socialLoad":      0,
		"recoveryHours":   0,
		"needs":           map[string]float64{},
		"updatedAt":       updatedAt.Format(time.RFC3339Nano),
		"sourceRequestId": "legacy-runtime-state-migration",
	})
	return body, true, err
}

func legacyRelationshipBody(db *gorm.DB, roleID string) (json.RawMessage, bool, error) {
	var row legacyRelationshipStateRow
	err := db.Table("relationship_states").
		Select("character_id, relation_data, updated_at").
		Where("character_id = ? AND relation_type = ?", roleID, "user_character").
		Order("CASE WHEN channel = '*' THEN 0 ELSE 1 END, updated_at DESC").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	values := map[string]float64{}
	if row.RelationData != "" {
		if err := json.Unmarshal([]byte(row.RelationData), &values); err != nil {
			return nil, false, err
		}
	}
	body, err := json.Marshal(map[string]any{
		"trust":            values["trust"],
		"familiarity":      values["familiarity"],
		"security":         values["security"],
		"tension":          values["tension"],
		"repairConfidence": values["repairConfidence"],
		"boundary":         values["boundary"],
		"updatedAt":        row.UpdatedAt,
		"sourceRequestId":  "legacy-runtime-state-migration",
	})
	return body, true, err
}

func legacyNeedBody(db *gorm.DB, roleID string) (json.RawMessage, bool, error) {
	var rows []legacyNeedStateRow
	if err := db.Table("need_states").Select("need_key, current_value, baseline, updated_at").Where("character_id = ?", roleID).Order("need_key").Scan(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	needs := make([]map[string]any, 0, len(rows))
	latest := ""
	for _, row := range rows {
		needs = append(needs, map[string]any{"kind": row.NeedKey, "level": row.CurrentValue, "baseline": row.Baseline, "updatedAt": row.UpdatedAt})
		if row.UpdatedAt > latest {
			latest = row.UpdatedAt
		}
	}
	body, err := json.Marshal(map[string]any{"needs": needs, "count": len(needs), "updatedAt": latest, "sourceRequestId": "legacy-runtime-state-migration"})
	return body, true, err
}

func migrationRequestID(prefix, value string) string {
	digest := sha256.Sum256([]byte(value))
	return prefix + "/" + hex.EncodeToString(digest[:])
}
