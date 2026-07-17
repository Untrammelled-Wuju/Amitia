package extension

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ownedResourceRecord struct {
	ID               string `gorm:"column:id;primaryKey"`
	ExtensionID      string `gorm:"column:extension_id"`
	ExtensionVersion string `gorm:"column:extension_version"`
	ResourceType     string `gorm:"column:resource_type"`
	ResourceID       string `gorm:"column:resource_id"`
	OwnerScopeType   string `gorm:"column:owner_scope_type"`
	OwnerScopeID     string `gorm:"column:owner_scope_id"`
	SourceRunID      string `gorm:"column:source_run_id"`
	Status           string `gorm:"column:status"`
	CleanupAttempts  int    `gorm:"column:cleanup_attempts"`
	LastError        string `gorm:"column:last_error"`
	CreatedAt        string `gorm:"column:created_at"`
	UpdatedAt        string `gorm:"column:updated_at"`
	CleanedAt        string `gorm:"column:cleaned_at"`
}

func (ownedResourceRecord) TableName() string { return "extension_owned_resources" }

func (r *Repository) RegisterOwnedSideEffects(ctx context.Context, scope ExecutionScope, effects []SideEffectRecord) error {
	if strings.TrimSpace(scope.ExtensionID) == "" || !r.db.Migrator().HasTable(&ownedResourceRecord{}) {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	owner := PermissionScope{Type: ScopeGlobal}
	if scope.CharacterID != "" {
		owner = PermissionScope{Type: ScopeCharacter, ID: scope.CharacterID}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, effect := range effects {
			if !effect.Confirmed || strings.TrimSpace(effect.TargetID) == "" {
				continue
			}
			record := ownedResourceRecord{ID: uuid.NewString(), ExtensionID: scope.ExtensionID, ExtensionVersion: scope.ExtensionVersion, ResourceType: effect.Type, ResourceID: effect.TargetID, OwnerScopeType: string(owner.Type), OwnerScopeID: owner.ID, SourceRunID: scope.RunID, Status: "active", CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "extension_id"}, {Name: "resource_type"}, {Name: "resource_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"extension_version": scope.ExtensionVersion, "owner_scope_type": owner.Type, "owner_scope_id": owner.ID, "source_run_id": scope.RunID, "status": "active", "last_error": "", "updated_at": now}),
			}).Create(&record).Error; err != nil {
				return err
			}
			if effect.Type == "schedule_create" && tx.Migrator().HasTable("schedules") && tx.Migrator().HasColumn("schedules", "source_extension_id") {
				updates := map[string]interface{}{"source_extension_id": scope.ExtensionID, "source_extension_version": scope.ExtensionVersion, "source_run_id": scope.RunID, "owner_scope_type": owner.Type, "owner_scope_id": owner.ID}
				if tx.Migrator().HasColumn("schedules", "source_type") {
					updates["source_type"] = "extension"
				}
				if err := tx.Table("schedules").Where("id = ?", effect.TargetID).Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Repository) CountOwnedResources(ctx context.Context, extensionID, scopeType, scopeID string) (int64, error) {
	if !r.db.Migrator().HasTable(&ownedResourceRecord{}) {
		return 0, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&ownedResourceRecord{}).Where("extension_id = ? AND owner_scope_type = ? AND owner_scope_id = ? AND resource_type = ? AND status IN ?", extensionID, scopeType, scopeID, "schedule_create", []string{"active", "cleanup_failed"}).Count(&count).Error
	return count, err
}

func (r *Repository) CleanupOwnedResources(ctx context.Context, extensionID, scopeType, scopeID string) error {
	if !r.db.Migrator().HasTable(&ownedResourceRecord{}) {
		return nil
	}
	var records []ownedResourceRecord
	if err := r.db.WithContext(ctx).Where("extension_id = ? AND owner_scope_type = ? AND owner_scope_id = ? AND status IN ?", extensionID, scopeType, scopeID, []string{"active", "cleanup_failed"}).Find(&records).Error; err != nil {
		return err
	}
	failures := []string{}
	for _, record := range records {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if record.ResourceType == "schedule_create" && tx.Migrator().HasTable("schedules") {
				if err := tx.Table("schedules").Where("id = ?", record.ResourceID).Delete(nil).Error; err != nil {
					return err
				}
			}
			status := "retained"
			if record.ResourceType == "schedule_create" {
				status = "cleaned"
			}
			return tx.Model(&ownedResourceRecord{}).Where("id = ?", record.ID).Updates(map[string]interface{}{"status": status, "cleanup_attempts": gorm.Expr("cleanup_attempts + 1"), "last_error": "", "updated_at": now, "cleaned_at": now}).Error
		})
		if err != nil {
			failures = append(failures, record.ResourceType+":"+record.ResourceID)
			_ = r.db.WithContext(ctx).Model(&ownedResourceRecord{}).Where("id = ?", record.ID).Updates(map[string]interface{}{"status": "cleanup_failed", "cleanup_attempts": gorm.Expr("cleanup_attempts + 1"), "last_error": err.Error(), "updated_at": now}).Error
		}
	}
	if len(failures) > 0 {
		return NewExtensionError(ErrPackageUninstallFailed, "扩展自有资源清理失败", strings.Join(failures, ", "), true, errors.New(fmt.Sprint(failures)))
	}
	return nil
}

func (r *Repository) RetryOwnedResourceCleanup(ctx context.Context) {
	if !r.db.Migrator().HasTable(&ownedResourceRecord{}) {
		return
	}
	var records []ownedResourceRecord
	if r.db.WithContext(ctx).Where("status = ? AND cleanup_attempts < ?", "cleanup_failed", 3).Find(&records).Error != nil {
		return
	}
	seen := map[string]bool{}
	for _, record := range records {
		key := record.ExtensionID + "\x00" + record.OwnerScopeType + "\x00" + record.OwnerScopeID
		if seen[key] {
			continue
		}
		seen[key] = true
		_ = r.CleanupOwnedResources(ctx, record.ExtensionID, record.OwnerScopeType, record.OwnerScopeID)
	}
}

func (r *Repository) CompensateUnownedSideEffects(ctx context.Context, scope ExecutionScope, effects []SideEffectRecord) {
	for _, effect := range effects {
		if !effect.Confirmed || effect.Type != "schedule_create" || strings.TrimSpace(effect.TargetID) == "" {
			continue
		}
		if r.db.WithContext(ctx).Table("schedules").Where("id = ?", effect.TargetID).Delete(nil).Error == nil {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		owner := PermissionScope{Type: ScopeGlobal}
		if scope.CharacterID != "" {
			owner = PermissionScope{Type: ScopeCharacter, ID: scope.CharacterID}
		}
		record := ownedResourceRecord{ID: uuid.NewString(), ExtensionID: scope.ExtensionID, ExtensionVersion: scope.ExtensionVersion, ResourceType: effect.Type, ResourceID: effect.TargetID, OwnerScopeType: string(owner.Type), OwnerScopeID: owner.ID, SourceRunID: scope.RunID, Status: "cleanup_failed", CleanupAttempts: 1, LastError: ErrPackageUninstallFailed, CreatedAt: now, UpdatedAt: now}
		_ = r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "extension_id"}, {Name: "resource_type"}, {Name: "resource_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"status": "cleanup_failed", "cleanup_attempts": gorm.Expr("cleanup_attempts + 1"), "last_error": ErrPackageUninstallFailed, "updated_at": now})}).Create(&record).Error
	}
}
