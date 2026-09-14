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

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

type extensionRecord struct {
	ID                     string `gorm:"column:id;primaryKey"`
	ExtensionID            string `gorm:"column:extension_id;uniqueIndex"`
	Kind                   string `gorm:"column:kind"`
	Name                   string `gorm:"column:name"`
	CurrentVersion         string `gorm:"column:current_version"`
	Source                 string `gorm:"column:source"`
	Enabled                int    `gorm:"column:enabled"`
	ManifestJSON           string `gorm:"column:manifest_json"`
	NormalizedManifestJSON string `gorm:"column:normalized_manifest_json"`
	CreatedAt              string `gorm:"column:created_at"`
	UpdatedAt              string `gorm:"column:updated_at"`
	ArchivedAt             string `gorm:"column:archived_at"`
}

func (extensionRecord) TableName() string { return "extensions" }

type extensionVersionRecord struct {
	ID           string `gorm:"column:id;primaryKey"`
	ExtensionID  string `gorm:"column:extension_id"`
	Version      string `gorm:"column:version"`
	ManifestJSON string `gorm:"column:manifest_json"`
	Checksum     string `gorm:"column:checksum"`
	CreatedAt    string `gorm:"column:created_at"`
}

func (extensionVersionRecord) TableName() string { return "extension_versions" }

type scopeBindingRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	ExtensionID string `gorm:"column:extension_id"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
	Enabled     int    `gorm:"column:enabled"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (scopeBindingRecord) TableName() string { return "extension_scope_bindings" }

func (r *Repository) hasScopeBindings() bool {
	return r.db != nil && r.db.Migrator().HasTable(&scopeBindingRecord{})
}

func (r *Repository) ResolveScopeEnabled(ctx context.Context, extensionID string, scope ExecutionScope, fallback bool) (bool, PermissionScope, error) {
	if !r.hasScopeBindings() {
		return fallback, PermissionScope{Type: ScopeGlobal}, nil
	}
	var global scopeBindingRecord
	err := r.db.WithContext(ctx).Where("extension_id = ? AND scope_type = ? AND scope_id = ''", extensionID, ScopeGlobal).First(&global).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, PermissionScope{}, nil
	}
	if err != nil {
		return false, PermissionScope{}, err
	}
	return global.Enabled == 1, PermissionScope{Type: ScopeGlobal}, nil
}

func (r *Repository) SetScopeEnabled(ctx context.Context, extensionID string, scope PermissionScope, enabled bool) error {
	if scope.Type != ScopeGlobal {
		return fmt.Errorf("unsupported extension binding scope: %s", scope.Type)
	}
	if !r.hasScopeBindings() {
		return r.db.WithContext(ctx).Model(&extensionRecord{}).Where("extension_id = ?", extensionID).Update("enabled", boolNumber(enabled)).Error
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := scopeBindingRecord{ID: uuid.NewString(), ExtensionID: extensionID, ScopeType: string(scope.Type), ScopeID: scope.ID, Enabled: boolNumber(enabled), CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&extensionRecord{}).Where("extension_id = ?", extensionID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return NewExtensionError(ErrSkillNotFound, "Skill not found", extensionID, false, nil)
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "extension_id"}, {Name: "scope_type"}, {Name: "scope_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"enabled": boolNumber(enabled), "updated_at": now}),
		}).Create(&record).Error; err != nil {
			return err
		}
		return tx.Model(&extensionRecord{}).Where("extension_id = ?", extensionID).Updates(map[string]interface{}{"enabled": boolNumber(enabled), "updated_at": now}).Error
	})
}

func (r *Repository) ValidateConversationScope(ctx context.Context, scope ExecutionScope) error {
	if strings.TrimSpace(scope.ConversationID) == "" {
		return nil
	}
	var conversation struct {
		CharacterID string `gorm:"column:character_id"`
		Channel     string `gorm:"column:channel"`
	}
	err := r.db.WithContext(ctx).Table("conversations").Select("character_id", "channel").Where("id = ?", scope.ConversationID).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewExtensionError(ErrSkillPermissionDenied, "Conversation scope is unavailable", scope.ConversationID, false, nil)
	}
	if err != nil {
		return fmt.Errorf("validate conversation scope: %w", err)
	}
	if conversation.CharacterID != scope.CharacterID || (scope.Channel != "" && conversation.Channel != "" && !strings.EqualFold(conversation.Channel, scope.Channel)) {
		return NewExtensionError(ErrSkillPermissionDenied, "Conversation scope mismatch", scope.ConversationID, false, nil)
	}
	return nil
}

func boolNumber(value bool) int {
	if value {
		return 1
	}
	return 0
}
