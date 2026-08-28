package migrationplans

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/migration"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB                    *gorm.DB
	ReadPathReady         func() error
	WritePathReady        func() error
	ReleasePublishedReady func(storageKey string) error
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
				required := []string{
					"desktop_pet_installations",
					"desktop_pet_runtime_desired_states",
					"desktop_pet_runtime_commands_v2",
					"desktop_pet_installation_runtime_projections",
					"desktop_pet_device_active_installation_bindings",
					"desktop_pet_package_releases",
					"kernel_device_runtime_sessions",
				}
				for _, table := range required {
					var count int64
					if err := deps.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&count).Error; err != nil {
						return false, "检查必需表失败: " + err.Error()
					}
					if count != 1 {
						return false, "缺少必需表: " + table
					}
				}
				return true, ""
			},
		},
		VerificationChecks: []migration.CheckFunc{
			func() (bool, string) {
				if deps.DB == nil {
					return false, "数据库未初始化"
				}
				var count int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM schema_migrations WHERE status = ?", "failed").Scan(&count).Error; err != nil {
					return false, "检查迁移状态失败: " + err.Error()
				}
				if count > 0 {
					return false, fmt.Sprintf("存在 %d 个失败的迁移", count)
				}
				return true, ""
			},
		},
		// Production installation read routes are RepositoryV2-only. These steps
		// validate that the V2 read model required by those routes is queryable and
		// internally consistent before the durable read-cutover record is written.
		ReadCutoverSteps: []migration.StepFunc{
			func() error {
				if deps.DB == nil {
					return fmt.Errorf("数据库未初始化")
				}
				if deps.ReadPathReady == nil {
					return fmt.Errorf("V2 production read path readiness check is not configured")
				}
				if err := deps.ReadPathReady(); err != nil {
					return fmt.Errorf("V2 production read path is not ready: %w", err)
				}
				var installationCount int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM desktop_pet_installations").Scan(&installationCount).Error; err != nil {
					return fmt.Errorf("V2 installation read probe failed: %w", err)
				}
				var desiredCount int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM desktop_pet_runtime_desired_states").Scan(&desiredCount).Error; err != nil {
					return fmt.Errorf("V2 desired-state read probe failed: %w", err)
				}
				var bindingCount int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM desktop_pet_device_active_installation_bindings").Scan(&bindingCount).Error; err != nil {
					return fmt.Errorf("V2 binding read probe failed: %w", err)
				}
				_ = installationCount
				_ = desiredCount
				_ = bindingCount
				return nil
			},
		},
		ReadCutoverChecks: []migration.CheckFunc{
			func() (bool, string) {
				var legacyTableCount int64
				if err := deps.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN (?, ?)", "desktop_pet_release_data", "desktop_pet_releases").Scan(&legacyTableCount).Error; err != nil {
					return false, "检查 legacy read schema 失败: " + err.Error()
				}
				if legacyTableCount != 0 {
					return false, "检测到已废弃的桌宠 Release 读表，拒绝读切转"
				}
				return true, ""
			},
		},
		// Production write routes are InstallationCoordinator + RepositoryV2.
		// Validate all durable write-side tables before disabling legacy writers.
		WriteCutoverSteps: []migration.StepFunc{
			func() error {
				if deps.DB == nil {
					return fmt.Errorf("数据库未初始化")
				}
				if deps.WritePathReady == nil {
					return fmt.Errorf("V2 production write path readiness check is not configured")
				}
				if err := deps.WritePathReady(); err != nil {
					return fmt.Errorf("V2 production write path is not ready: %w", err)
				}
				required := []string{
					"desktop_pet_installation_operations",
					"desktop_pet_runtime_desired_states",
					"desktop_pet_runtime_desired_state_outbox",
					"desktop_pet_runtime_commands_v2",
					"desktop_pet_installation_runtime_projections",
				}
				for _, table := range required {
					var count int64
					if err := deps.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&count).Error; err != nil {
						return err
					}
					if count != 1 {
						return fmt.Errorf("V2 write path table missing: %s", table)
					}
				}
				return nil
			},
		},
		WriteCutoverChecks: []migration.CheckFunc{
			func() (bool, string) {
				var broken int64
				if err := deps.DB.Raw(`SELECT COUNT(*)
FROM desktop_pet_installations i
LEFT JOIN desktop_pet_runtime_desired_states d
  ON d.user_id = i.user_id AND d.device_id = i.device_id AND d.installation_id = i.id
WHERE i.status IN ('active','installed') AND d.id IS NULL`).Scan(&broken).Error; err != nil {
					return false, "检查 V2 写路径一致性失败: " + err.Error()
				}
				if broken != 0 {
					return false, fmt.Sprintf("存在 %d 个活动 Installation 没有 DesiredState", broken)
				}
				return true, ""
			},
		},
		LegacyWriteBlockSteps: []migration.StepFunc{
			func() error {
				if deps.DB == nil {
					return fmt.Errorf("数据库未初始化")
				}
				for _, stepName := range []string{"installation", "editing"} {
					var verified int64
					if err := deps.DB.Raw(`SELECT COUNT(*) FROM desktop_pet_write_cutovers WHERE step_name = ? AND verified = 1`, stepName).Scan(&verified).Error; err != nil {
						return fmt.Errorf("verify %s write cutover: %w", stepName, err)
					}
					if verified == 0 {
						return fmt.Errorf("%s write cutover is not verified", stepName)
					}
				}
				// The durable authority for blocking legacy writes is the migration
				// operation stage itself. The runner advances to legacy_write_blocked
				// only after both verified cutovers above succeed, then refreshes the
				// in-process guards from that durable state.
				return nil
			},
		},
		ParityChecks: []migration.ParityCheck{
			{
				Name:     "runtime_session_projection",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					type sessionRow struct {
						ID        string
						UserID    string
						DeviceID  string
						RuntimeID string
						Status    string
					}
					var authority []sessionRow
					if err := deps.DB.WithContext(ctx).Raw(`SELECT runtime_session_id AS id, user_id, device_id, runtime_id, status
FROM kernel_device_runtime_sessions
WHERE status IN ('registering','syncing','ready','degraded')`).Scan(&authority).Error; err != nil {
						return false, "", err
					}
					for _, row := range authority {
						var projected int64
						if err := deps.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM desktop_pet_runtime_sessions
WHERE id = ? AND user_id = ? AND device_id = ? AND runtime_id = ? AND status = ?`, row.ID, row.UserID, row.DeviceID, row.RuntimeID, row.Status).Scan(&projected).Error; err != nil {
							return false, "", err
						}
						if projected != 1 {
							return false, fmt.Sprintf("runtime session projection mismatch: session=%s", row.ID), nil
						}
					}
					return true, "", nil
				},
			},
			{
				Name:     "installation_integrity",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					type installationRow struct {
						ID               string
						UserID           string
						DeviceID         string
						PetID            string
						CurrentReleaseID string
						Status           string
					}
					var installations []installationRow
					if err := deps.DB.WithContext(ctx).Raw("SELECT id, user_id, device_id, pet_id, current_release_id, status FROM desktop_pet_installations WHERE status IN (?, ?)", "active", "installed").Scan(&installations).Error; err != nil {
						return false, "", err
					}
					for _, inst := range installations {
						var desiredCount int64
						if err := deps.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM desktop_pet_runtime_desired_states
WHERE user_id = ? AND device_id = ? AND installation_id = ? AND pet_id = ? AND release_id = ?`, inst.UserID, inst.DeviceID, inst.ID, inst.PetID, inst.CurrentReleaseID).Scan(&desiredCount).Error; err != nil {
							return false, "", err
						}
						if desiredCount != 1 {
							return false, fmt.Sprintf("installation %s: desired state mismatch", inst.ID), nil
						}
						var bindingCount int64
						if err := deps.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM desktop_pet_device_active_installation_bindings
WHERE user_id = ? AND device_id = ? AND installation_id = ? AND pet_id = ? AND release_id = ?`, inst.UserID, inst.DeviceID, inst.ID, inst.PetID, inst.CurrentReleaseID).Scan(&bindingCount).Error; err != nil {
							return false, "", err
						}
						if bindingCount != 1 {
							return false, fmt.Sprintf("installation %s: active binding mismatch", inst.ID), nil
						}
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
						IntegrityStatus     string
						CompatibilityStatus string
						StorageKey          string
						ManifestHash        string
						ContentRootHash     string
					}
					var rows []releaseRow
					if err := deps.DB.WithContext(ctx).Raw("SELECT id, integrity_status, compatibility_status, storage_key, manifest_hash, content_root_hash FROM desktop_pet_package_releases WHERE lifecycle = ?", "ready").Scan(&rows).Error; err != nil {
						return false, "", err
					}
					for _, row := range rows {
						if row.IntegrityStatus != "verified" || row.CompatibilityStatus != "compatible" {
							return false, fmt.Sprintf("release %s: integrity/compatibility not verified", row.ID), nil
						}
						if row.StorageKey == "" || row.ManifestHash == "" || row.ContentRootHash == "" {
							return false, fmt.Sprintf("release %s: missing storage or hashes", row.ID), nil
						}
						if deps.ReleasePublishedReady == nil {
							return false, "release published-path readiness check is not configured", nil
						}
						if err := deps.ReleasePublishedReady(row.StorageKey); err != nil {
							return false, fmt.Sprintf("release %s: published storage invalid: %v", row.ID, err), nil
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
	plan.VerificationChecks = append(plan.VerificationChecks, func() (bool, string) {
		if deps.DB == nil {
			return false, "数据库未初始化"
		}
		return true, ""
	})
	return plan
}

func ValidatePlanID(id string) error {
	if id != "desktop-pet-v2-cutover" {
		return fmt.Errorf("未知的迁移计划: %s", id)
	}
	return nil
}
