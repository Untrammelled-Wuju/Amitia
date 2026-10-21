package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

func migrateLegacyWorldbook(ctx context.Context, businessDB *gorm.DB, kernelDB *sql.DB, spaceID, ownerID string) error {
	spaceID, ownerID = strings.TrimSpace(spaceID), strings.TrimSpace(ownerID)
	if businessDB == nil || kernelDB == nil || spaceID == "" || ownerID == "" {
		return errors.New("世界书迁移参数无效")
	}
	const migrationID = "worldbook-migration"
	var existing int
	if err := kernelDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND resource_id=? AND deleted=0`, ownerID, migrationID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	var rows []struct {
		ID            string
		MatchType     string
		MatchPattern  string
		MatchScope    string
		InjectContent string
		Priority      int
		CharacterID   string
		CreatedAt     string
		UpdatedAt     string
	}
	if err := businessDB.WithContext(ctx).Table("world_book").
		Select("id,match_type,match_pattern,match_scope,inject_content,priority,character_id,created_at,updated_at").
		Where("space_id=? OR space_id='' OR space_id='default'", spaceID).
		Order("created_at,id").
		Scan(&rows).Error; err != nil {
		return err
	}
	tx, err := kernelDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	migrated := 0
	for _, row := range rows {
		roleID := strings.TrimSpace(row.CharacterID)
		scope := "role"
		if roleID == "" {
			roleID, scope = coordination.GlobalWorldbookRole, "global"
		}
		createdAt, updatedAt := strings.TrimSpace(row.CreatedAt), strings.TrimSpace(row.UpdatedAt)
		if createdAt == "" {
			createdAt = now
		}
		if updatedAt == "" {
			updatedAt = createdAt
		}
		identity := sha256.Sum256([]byte(ownerID + "\x00" + spaceID + "\x00" + row.ID))
		resourceID := "worldbook-" + hex.EncodeToString(identity[:16])
		rule := business.WorldbookRule{
			ID: resourceID, Scope: scope, CharacterID: row.CharacterID,
			MatchType: row.MatchType, MatchPattern: row.MatchPattern,
			MatchScope: row.MatchScope, InjectContent: row.InjectContent,
			Priority: row.Priority, Enabled: true, CreatedAt: createdAt, UpdatedAt: updatedAt,
		}
		encoded, err := json.Marshal(rule)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,'worldbook',?,?,'',1,0,?,?) ON CONFLICT(owner_id,kind,resource_id) DO NOTHING`, ownerID, resourceID, roleID, encoded, updatedAt)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil {
			return err
		} else {
			migrated += int(affected)
		}
	}
	marker, _ := json.Marshal(map[string]any{"spaceId": spaceID, "ownerId": ownerID, "legacyRecords": len(rows), "migratedRecords": migrated, "migratedAt": now})
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,'checkpoint',?,?,?,1,0,?,?) ON CONFLICT(owner_id,kind,resource_id) DO NOTHING`, ownerID, migrationID, "@worldbook-migration", "", marker, now); err != nil {
		return err
	}
	return tx.Commit()
}
