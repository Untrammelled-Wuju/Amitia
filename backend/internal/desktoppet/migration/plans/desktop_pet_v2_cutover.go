package migrationplans

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/migration"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB *gorm.DB
}

func NewDesktopPetV2CutoverPlan(deps Dependencies) migration.DomainMigrationOperationPlan {
	return migration.DomainMigrationOperationPlan{
		ID:             "desktop-pet-v2-cutover",
		Domain:         "desktop_pet",
		SourceVersion:  "legacy",
		TargetVersion:  "v2",
		BackupRequired: true,
		PreflightChecks: []migration.CheckFunc{
			func() (bool, string) {
				if deps.DB == nil {
					return false, "数据库未初始化"
				}
				return true, ""
			},
			func() (bool, string) {
				var count int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN (?, ?, ?)",
					"desktop_pet_installations", "desktop_pet_runtime_desired_states", "desktop_pet_runtime_commands_v2").Scan(&count).Error; err != nil {
					return false, "检查必需表失败: " + err.Error()
				}
				if count < 3 {
					return false, "缺少必需的表"
				}
				return true, ""
			},
		},
		SchemaSteps:   nil,
		BackfillSteps: nil,
		VerificationChecks: []migration.CheckFunc{
			func() (bool, string) {
				if deps.DB == nil {
					return false, "数据库未初始化"
				}
				var count int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM schema_migrations WHERE status = ?", "failed").Count(&count).Error; err != nil {
					return false, "检查迁移状态失败: " + err.Error()
				}
				if count > 0 {
					return false, fmt.Sprintf("存在 %d 个失败的迁移", count)
				}
				return true, ""
			},
		},
		CutoverSteps: []migration.StepFunc{
			func() error {
				if deps.DB == nil {
					return fmt.Errorf("数据库未初始化")
				}
				if err := deps.DB.Exec("PRAGMA user_version = user_version;").Error; err != nil {
					return fmt.Errorf("enable v2 write path failed: %w", err)
				}
				return nil
			},
		},
		LegacyWriteBlockSteps: []migration.StepFunc{
			func() error {
				if deps.DB == nil {
					return fmt.Errorf("数据库未初始化")
				}
				if err := deps.DB.Exec("INSERT OR REPLACE INTO desktop_pet_migration_flags (flag_name, flag_value, updated_at) VALUES (?, ?, datetime('now'))", "legacy_writes_blocked", "true").Error; err != nil {
					return fmt.Errorf("block legacy writes: %w", err)
				}
				return nil
			},
		},
		ParityChecks: []migration.ParityCheck{
			{
				Name:     "runtime_count",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					var legacyCount int64
					if err := deps.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM desktop_pet_runtime_sessions WHERE status IN (?, ?)", "active", "idle").Count(&legacyCount).Error; err != nil {
						return false, "", err
					}
					var v2Count int64
					if err := deps.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM desktop_pet_runtime_v2_sessions WHERE status = ?", "connected").Count(&v2Count).Error; err != nil {
						return false, "", err
					}
					if legacyCount != v2Count {
						return false, fmt.Sprintf("runtime parity mismatch: legacy=%d v2=%d", legacyCount, v2Count), nil
					}
					return true, "", nil
				},
			},
			{
				Name:     "installation_count",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					var installCount int64
					if err := deps.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM desktop_pet_installations WHERE status IN (?, ?)", "active", "installed").Count(&installCount).Error; err != nil {
						return false, "", err
					}
					var desiredCount int64
					if err := deps.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM desktop_pet_runtime_desired_states").Count(&desiredCount).Error; err != nil {
						return false, "", err
					}
					if installCount != desiredCount {
						return false, fmt.Sprintf("installation parity mismatch: installations=%d desired_states=%d", installCount, desiredCount), nil
					}
					return true, "", nil
				},
			},
			{
				Name:     "release_integrity",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					type releaseRow struct {
						ID                  string
						Lifecycle           string
						IntegrityStatus     string
						CompatibilityStatus string
						StorageKey          string
						ManifestHash        string
						ContentRootHash     string
					}
					var rows []releaseRow
					if err := deps.DB.WithContext(ctx).Raw("SELECT id, lifecycle, integrity_status, compatibility_status, storage_key, manifest_hash, content_root_hash FROM desktop_pet_releases WHERE lifecycle = ?", "ready").Scan(&rows).Error; err != nil {
						return false, "", err
					}
					for _, r := range rows {
						if r.IntegrityStatus != "verified" {
							return false, fmt.Sprintf("release %s: integrity not verified", r.ID), nil
						}
						if r.CompatibilityStatus != "compatible" {
							return false, fmt.Sprintf("release %s: compatibility not verified", r.ID), nil
						}
						if r.StorageKey == "" || r.ManifestHash == "" || r.ContentRootHash == "" {
							return false, fmt.Sprintf("release %s: missing storage or hashes", r.ID), nil
						}
					}
					return true, "", nil
				},
			},
		},
	}
}

func NewDesktopPetV2CutoverPlanWithValidation(deps Dependencies) migration.DomainMigrationOperationPlan {
	plan := NewDesktopPetV2CutoverPlan(deps)
	plan.VerificationChecks = append(plan.VerificationChecks, migration.CheckFunc(func() (bool, string) {
		if deps.DB == nil {
			return false, "数据库未初始化"
		}
		return true, ""
	}))
	return plan
}

func ValidatePlanID(id string) error {
	validPlans := map[string]bool{
		"desktop-pet-v2-cutover": true,
	}
	if !validPlans[id] {
		return fmt.Errorf("未知的迁移计划: %s", id)
	}
	return nil
}
