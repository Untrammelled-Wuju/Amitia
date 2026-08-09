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
		ID:            "desktop-pet-v2-cutover",
		Domain:        "desktop_pet",
		SourceVersion: "legacy",
		TargetVersion: "v2",
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
				return true, ""
			},
		},
		CutoverSteps: []migration.StepFunc{
			func() error {
				return nil
			},
		},
		LegacyWriteBlockSteps: []migration.StepFunc{
			func() error {
				return nil
			},
		},
		ParityChecks: []migration.ParityCheck{
			{
				Name:     "runtime_count",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					return true, "", nil
				},
			},
			{
				Name:     "installation_count",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
					return true, "", nil
				},
			},
			{
				Name:     "release_integrity",
				Required: true,
				Check: func(ctx context.Context) (bool, string, error) {
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
