package safety

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

type PolicyScope struct {
	SpaceID string `json:"spaceId"`
	OwnerID string `json:"ownerId"`
	CoreID  string `json:"coreId"`
}

type RuntimePolicy struct {
	Revision int64        `json:"revision"`
	Config   SafetyConfig `json:"config"`
	BDI      BdiConfig    `json:"bdi"`
}

type RuntimePolicyReader interface {
	RuntimePolicy(context.Context, string, string) (RuntimePolicy, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithDB(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func DefaultPolicy() RuntimePolicy {
	return RuntimePolicy{
		Revision: 1,
		Config:   *defaultSafetyConfig(),
		BDI:      DefaultBdiConfig(),
	}
}

func DefaultBdiConfig() BdiConfig {
	return BdiConfig{
		HardConstraints: []HardConstraint{},
		SoftPreferences: []SoftPreference{},
		CopingStrategy: &CopingStrategy{
			Selected:     "active",
			Alternatives: []string{},
		},
		EmotionExpression: &EmotionExpression{
			DisplayMode:       "show",
			InternalIntensity: 5,
			DisplayIntensity:  5,
		},
	}
}

func ScopeFromContext(ctx context.Context) (PolicyScope, error) {
	return ScopeFromContextWithCoordinator(ctx, nil)
}

func ScopeFromContextWithCoordinator(ctx context.Context, coordinator *coordination.Service) (PolicyScope, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || strings.TrimSpace(string(actor.SpaceID)) == "" {
		return PolicyScope{}, errors.New("安全治理缺少可信空间身份")
	}
	scope := PolicyScope{SpaceID: strings.TrimSpace(string(actor.SpaceID)), CoreID: strings.TrimSpace(string(actor.SpaceID)), OwnerID: strings.TrimSpace(actor.DeviceID.String())}
	if execution, found := coordination.FromContext(ctx); found {
		scope.SpaceID = strings.TrimSpace(execution.SpaceID)
		scope.CoreID = strings.TrimSpace(execution.CoreID)
		scope.OwnerID = strings.TrimSpace(execution.ResourceOwnerID)
	} else if coordinator != nil && actor.DeviceID != "" {
		policy, err := coordinator.Get(ctx, actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			return PolicyScope{}, err
		}
		if policy.Coordinated {
			scope.OwnerID = actor.SpaceID.String()
		}
	}
	if scope.SpaceID == "" || scope.CoreID == "" || scope.OwnerID == "" {
		return PolicyScope{}, errors.New("安全治理数据归属不完整")
	}
	return scope, nil
}

func (r *Repository) RuntimePolicy(ctx context.Context, spaceID, ownerID string) (RuntimePolicy, error) {
	scope := PolicyScope{SpaceID: strings.TrimSpace(spaceID), OwnerID: strings.TrimSpace(ownerID), CoreID: strings.TrimSpace(spaceID)}
	if scope.SpaceID == "" || scope.OwnerID == "" || scope.CoreID == "" {
		return RuntimePolicy{}, errors.New("安全策略范围无效")
	}
	return r.ensurePolicy(ctx, scope)
}

func (r *Repository) Policy(ctx context.Context, scope PolicyScope) (RuntimePolicy, error) {
	if err := validatePolicyScope(scope); err != nil {
		return RuntimePolicy{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return RuntimePolicy{}, err
	}
	return r.ensurePolicy(ctx, scope)
}

func (r *Repository) SaveConfig(ctx context.Context, scope PolicyScope, config SafetyConfig) (RuntimePolicy, error) {
	if err := validatePolicyScope(scope); err != nil {
		return RuntimePolicy{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return RuntimePolicy{}, err
	}
	expected := config.ExpectedRevision
	config.Revision = 0
	config.ExpectedRevision = 0
	raw, err := json.Marshal(config)
	if err != nil {
		return RuntimePolicy{}, err
	}
	return r.savePolicy(ctx, scope, expected, "config_json", raw)
}

func (r *Repository) SaveBDI(ctx context.Context, scope PolicyScope, bdi BdiConfig) (RuntimePolicy, error) {
	if err := validatePolicyScope(scope); err != nil {
		return RuntimePolicy{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return RuntimePolicy{}, err
	}
	expected := bdi.ExpectedRevision
	if err := ValidateBdiConfig(bdi); err != nil {
		return RuntimePolicy{}, err
	}
	bdi = NormalizeBdiConfig(bdi)
	bdi.Revision = 0
	bdi.ExpectedRevision = 0
	raw, err := json.Marshal(bdi)
	if err != nil {
		return RuntimePolicy{}, err
	}
	return r.savePolicy(ctx, scope, expected, "bdi_json", raw)
}

func (r *Repository) ImportLegacy(ctx context.Context, scopes []PolicyScope) error {
	if r == nil || r.db == nil {
		return errors.New("安全策略存储不可用")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var values []struct {
			Key   string `gorm:"column:key"`
			Value string `gorm:"column:value"`
		}
		if err := tx.Table("app_settings").Select("key,value").Where("key IN ? AND deleted_at IS NULL", []string{"safety_config", "safety_bdi_config"}).Scan(&values).Error; err != nil {
			return err
		}
		if len(values) == 0 {
			return nil
		}
		config := *defaultSafetyConfig()
		bdi := DefaultBdiConfig()
		for _, value := range values {
			switch value.Key {
			case "safety_config":
				if json.Unmarshal([]byte(value.Value), &config) != nil {
					return errors.New("旧安全配置无法解析")
				}
			case "safety_bdi_config":
				if json.Unmarshal([]byte(value.Value), &bdi) != nil {
					return errors.New("旧 BDI 安全配置无法解析")
				}
			}
		}
		config.Revision, config.ExpectedRevision = 0, 0
		bdi.Revision, bdi.ExpectedRevision = 0, 0
		configRaw, err := json.Marshal(config)
		if err != nil {
			return err
		}
		bdiRaw, err := json.Marshal(bdi)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, scope := range scopes {
			if validatePolicyScope(scope) != nil {
				continue
			}
			if err := tx.Exec(`INSERT INTO safety_policies(space_id,owner_id,core_id,revision,config_json,bdi_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(space_id,owner_id) DO NOTHING`, scope.SpaceID, scope.OwnerID, scope.CoreID, 1, string(configRaw), string(bdiRaw), now, now).Error; err != nil {
				return err
			}
		}
		return tx.Exec("DELETE FROM app_settings WHERE key IN ?", []string{"safety_config", "safety_bdi_config"}).Error
	})
}

func (r *Repository) ensurePolicy(ctx context.Context, scope PolicyScope) (RuntimePolicy, error) {
	var row struct {
		CoreID     string `gorm:"column:core_id"`
		Revision   int64  `gorm:"column:revision"`
		ConfigJSON string `gorm:"column:config_json"`
		BDIJSON    string `gorm:"column:bdi_json"`
	}
	err := r.db.WithContext(ctx).Table("safety_policies").Select("core_id,revision,config_json,bdi_json").Where("space_id=? AND owner_id=?", scope.SpaceID, scope.OwnerID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		policy := DefaultPolicy()
		configRaw, _ := json.Marshal(policy.Config)
		bdiRaw, _ := json.Marshal(policy.BDI)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if createErr := r.db.WithContext(ctx).Exec(`INSERT INTO safety_policies(space_id,owner_id,core_id,revision,config_json,bdi_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(space_id,owner_id) DO NOTHING`, scope.SpaceID, scope.OwnerID, scope.CoreID, 1, string(configRaw), string(bdiRaw), now, now).Error; createErr != nil {
			return RuntimePolicy{}, createErr
		}
		return r.ensurePolicy(ctx, scope)
	}
	if err != nil {
		return RuntimePolicy{}, err
	}
	policy := DefaultPolicy()
	policy.Revision = row.Revision
	if json.Unmarshal([]byte(row.ConfigJSON), &policy.Config) != nil {
		return RuntimePolicy{}, errors.New("安全策略配置已损坏")
	}
	if json.Unmarshal([]byte(row.BDIJSON), &policy.BDI) != nil {
		return RuntimePolicy{}, errors.New("BDI 安全策略已损坏")
	}
	policy.Config.Revision = row.Revision
	policy.Config.ExpectedRevision = 0
	policy.BDI.Revision = row.Revision
	policy.BDI.ExpectedRevision = 0
	return policy, nil
}

func (r *Repository) savePolicy(ctx context.Context, scope PolicyScope, expected int64, field string, raw json.RawMessage) (RuntimePolicy, error) {
	if expected < 1 {
		policy, err := r.ensurePolicy(ctx, scope)
		if err != nil {
			return RuntimePolicy{}, err
		}
		expected = policy.Revision
	}
	result := r.db.WithContext(ctx).Exec("UPDATE safety_policies SET "+field+"=?,revision=revision+1,updated_at=? WHERE space_id=? AND owner_id=? AND revision=?", string(raw), time.Now().UTC().Format(time.RFC3339Nano), scope.SpaceID, scope.OwnerID, expected)
	if result.Error != nil {
		return RuntimePolicy{}, result.Error
	}
	if result.RowsAffected != 1 {
		return RuntimePolicy{}, gorm.ErrRecordNotFound
	}
	return r.ensurePolicy(ctx, scope)
}

func validatePolicyScope(scope PolicyScope) error {
	if strings.TrimSpace(scope.SpaceID) == "" || strings.TrimSpace(scope.OwnerID) == "" || strings.TrimSpace(scope.CoreID) == "" {
		return errors.New("安全治理数据归属不完整")
	}
	return nil
}
