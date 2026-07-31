package generation

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const (
	ErrCodeTaskPlanNotFound   = "GEN_TASK_PLAN_NOT_FOUND"
	ErrCodeActionPlanNotFound = "GEN_ACTION_PLAN_NOT_FOUND"
)

var (
	ErrTaskPlanNotFound   = NewGenerationError(ErrCodeTaskPlanNotFound, "task plan not found", nil)
	ErrActionPlanNotFound = NewGenerationError(ErrCodeActionPlanNotFound, "action plan not found", nil)
)

type TaskPlan struct {
	ID                          string `gorm:"column:id;primaryKey;type:text" json:"id"`
	TaskID                      string `gorm:"column:task_id;type:text" json:"taskId"`
	SchemaVersion               int    `gorm:"column:schema_version;type:integer" json:"schemaVersion"`
	PlanHash                    string `gorm:"column:plan_hash;type:text" json:"planHash"`
	Provider                    string `gorm:"column:provider;type:text" json:"provider"`
	Model                       string `gorm:"column:model;type:text" json:"model"`
	ConfigID                    int    `gorm:"column:config_id;type:integer" json:"configId"`
	ConfigRevision              string `gorm:"column:config_revision;type:text" json:"configRevision"`
	CapabilitySnapshotJSON      string `gorm:"column:capability_snapshot_json;type:text" json:"capabilitySnapshotJson"`
	CapabilitySnapshotHash      string `gorm:"column:capability_snapshot_hash;type:text" json:"capabilitySnapshotHash"`
	ReferenceAssetID            string `gorm:"column:reference_asset_id;type:text" json:"referenceAssetId"`
	CostEstimateJSON            string `gorm:"column:cost_estimate_json;type:text" json:"costEstimateJson"`
	PlannedPrimaryRequestCount  int    `gorm:"column:planned_primary_request_count;type:integer" json:"plannedPrimaryRequestCount"`
	PlannedMaxProviderCallCount int    `gorm:"column:planned_max_provider_call_count;type:integer" json:"plannedMaxProviderCallCount"`
	PlanJSON                    string `gorm:"column:plan_json;type:text" json:"planJson"`
	FrozenAt                    string `gorm:"column:frozen_at;type:text" json:"frozenAt"`
	CreatedAt                   string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt                   string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (TaskPlan) TableName() string { return "desktop_pet_generation_task_plans" }

type ActionPlan struct {
	ID                          string `gorm:"column:id;primaryKey;type:text" json:"id"`
	TaskPlanID                  string `gorm:"column:task_plan_id;type:text" json:"taskPlanId"`
	TaskID                      string `gorm:"column:task_id;type:text" json:"taskId"`
	TaskActionID                string `gorm:"column:task_action_id;type:text" json:"taskActionId"`
	ActionKey                   string `gorm:"column:action_key;type:text" json:"actionKey"`
	SchemaVersion               int    `gorm:"column:schema_version;type:integer" json:"schemaVersion"`
	PlanHash                    string `gorm:"column:plan_hash;type:text" json:"planHash"`
	Mode                        string `gorm:"column:mode;type:text" json:"mode"`
	Provider                    string `gorm:"column:provider;type:text" json:"provider"`
	Model                       string `gorm:"column:model;type:text" json:"model"`
	ConfigID                    int    `gorm:"column:config_id;type:integer" json:"configId"`
	ConfigRevision              string `gorm:"column:config_revision;type:text" json:"configRevision"`
	CapabilityHash              string `gorm:"column:capability_hash;type:text" json:"capabilityHash"`
	ReferenceAssetID            string `gorm:"column:reference_asset_id;type:text" json:"referenceAssetId"`
	LayoutJSON                  string `gorm:"column:layout_json;type:text" json:"layoutJson"`
	LayoutHash                  string `gorm:"column:layout_hash;type:text" json:"layoutHash"`
	PromptSnapshot              string `gorm:"column:prompt_snapshot;type:text" json:"promptSnapshot"`
	PromptHash                  string `gorm:"column:prompt_hash;type:text" json:"promptHash"`
	NegativePromptSnapshot      string `gorm:"column:negative_prompt_snapshot;type:text" json:"negativePromptSnapshot"`
	NegativePromptHash          string `gorm:"column:negative_prompt_hash;type:text" json:"negativePromptHash"`
	SeedPolicy                  string `gorm:"column:seed_policy;type:text" json:"seedPolicy"`
	SeedValue                   *int64 `gorm:"column:seed_value;type:integer" json:"seedValue"`
	OutputCount                 int    `gorm:"column:output_count;type:integer" json:"outputCount"`
	TargetFrameCount            int    `gorm:"column:target_frame_count;type:integer" json:"targetFrameCount"`
	PlannedSegmentCount         int    `gorm:"column:planned_segment_count;type:integer" json:"plannedSegmentCount"`
	PlannedPrimaryRequestCount  int    `gorm:"column:planned_primary_request_count;type:integer" json:"plannedPrimaryRequestCount"`
	PlannedMaxProviderCallCount int    `gorm:"column:planned_max_provider_call_count;type:integer" json:"plannedMaxProviderCallCount"`
	PlannedCallCount            int    `gorm:"column:planned_call_count;type:integer" json:"plannedCallCount"`
	SheetWidth                  int    `gorm:"column:sheet_width;type:integer" json:"sheetWidth"`
	SheetHeight                 int    `gorm:"column:sheet_height;type:integer" json:"sheetHeight"`
	CellWidth                   int    `gorm:"column:cell_width;type:integer" json:"cellWidth"`
	CellHeight                  int    `gorm:"column:cell_height;type:integer" json:"cellHeight"`
	FallbackMode                string `gorm:"column:fallback_mode;type:text" json:"fallbackMode"`
	ActionSpecVersion           string `gorm:"column:action_spec_version;type:text" json:"actionSpecVersion"`
	ActionCatalogHash           string `gorm:"column:action_catalog_hash;type:text" json:"actionCatalogHash"`
	ProviderConfigHash          string `gorm:"column:provider_config_hash;type:text" json:"providerConfigHash"`
	SafetyPolicyVersion         string `gorm:"column:safety_policy_version;type:text" json:"safetyPolicyVersion"`
	OutputFormat                string `gorm:"column:output_format;type:text" json:"outputFormat"`
	PlanJSON                    string `gorm:"column:plan_json;type:text" json:"planJson"`
	FrozenAt                    string `gorm:"column:frozen_at;type:text" json:"frozenAt"`
	CreatedAt                   string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt                   string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ActionPlan) TableName() string { return "desktop_pet_generation_action_plans" }

type TaskPlanRepository interface {
	CreateTaskPlanTx(tx *gorm.DB, plan *TaskPlan) error
	GetTaskPlanByTaskID(taskID string) (*TaskPlan, error)
	GetTaskPlanByTaskIDTx(tx *gorm.DB, taskID string) (*TaskPlan, error)
}

type ActionPlanRepository interface {
	CreateActionPlanTx(tx *gorm.DB, plan *ActionPlan) error
	ListActionPlansByTaskID(taskID string) ([]ActionPlan, error)
	GetActionPlanByActionID(taskActionID string) (*ActionPlan, error)
	GetActionPlanByActionIDTx(tx *gorm.DB, taskActionID string) (*ActionPlan, error)
}

type taskPlanRepository struct {
	db *gorm.DB
}

func NewTaskPlanRepository(db *gorm.DB) TaskPlanRepository {
	return &taskPlanRepository{db: db}
}

func (r *taskPlanRepository) CreateTaskPlanTx(tx *gorm.DB, plan *TaskPlan) error {
	if tx == nil {
		tx = r.db
	}
	if plan.ID == "" {
		plan.ID = generateUUID()
	}
	now := nowRFC3339()
	if plan.CreatedAt == "" {
		plan.CreatedAt = now
	}
	if plan.UpdatedAt == "" {
		plan.UpdatedAt = now
	}
	if err := tx.Create(plan).Error; err != nil {
		return fmt.Errorf("create task plan: %w", err)
	}
	return nil
}

func (r *taskPlanRepository) GetTaskPlanByTaskID(taskID string) (*TaskPlan, error) {
	return r.GetTaskPlanByTaskIDTx(r.db, taskID)
}

func (r *taskPlanRepository) GetTaskPlanByTaskIDTx(tx *gorm.DB, taskID string) (*TaskPlan, error) {
	if tx == nil {
		tx = r.db
	}
	var plan TaskPlan
	err := tx.Where("task_id = ?", taskID).First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

type actionPlanRepository struct {
	db *gorm.DB
}

func NewActionPlanRepository(db *gorm.DB) ActionPlanRepository {
	return &actionPlanRepository{db: db}
}

func (r *actionPlanRepository) CreateActionPlanTx(tx *gorm.DB, plan *ActionPlan) error {
	if tx == nil {
		tx = r.db
	}
	if plan.ID == "" {
		plan.ID = generateUUID()
	}
	now := nowRFC3339()
	if plan.CreatedAt == "" {
		plan.CreatedAt = now
	}
	if plan.UpdatedAt == "" {
		plan.UpdatedAt = now
	}
	if err := tx.Create(plan).Error; err != nil {
		return fmt.Errorf("create action plan: %w", err)
	}
	return nil
}

func (r *actionPlanRepository) ListActionPlansByTaskID(taskID string) ([]ActionPlan, error) {
	var plans []ActionPlan
	err := r.db.Where("task_id = ?", taskID).Order("action_key ASC").Find(&plans).Error
	if err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *actionPlanRepository) GetActionPlanByActionID(taskActionID string) (*ActionPlan, error) {
	return r.GetActionPlanByActionIDTx(r.db, taskActionID)
}

func (r *actionPlanRepository) GetActionPlanByActionIDTx(tx *gorm.DB, taskActionID string) (*ActionPlan, error) {
	if tx == nil {
		tx = r.db
	}
	var plan ActionPlan
	err := tx.Where("task_action_id = ?", taskActionID).First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActionPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}
