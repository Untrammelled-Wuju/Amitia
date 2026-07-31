package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/u-ai/backend/internal/desktoppet/generationlayout"
	"github.com/u-ai/backend/internal/desktoppet/generationprompt"
	"github.com/u-ai/backend/internal/imageprovider"
)

type PlanFreezeInput struct {
	Tx                     *gorm.DB
	TaskID                 string
	UserID                 string
	CharacterID            string
	Provider               string
	Model                  string
	ConfigID               int
	ConfigRevision         string
	CapabilitySnapshotJSON string
	CapabilitySnapshotHash string
	ReferenceAssetID       string
	TaskActions            []TaskActionForPlanning
}

type TaskActionForPlanning struct {
	TaskActionID   string
	ActionKey      string
	FrameCount     int
	ActionSpecJSON string
	ActionSpecHash string
	Mode             string
	Capabilities     imageprovider.ProviderCapabilities
	CapabilityHash   string
	LayoutResult     *generationlayout.LayoutResult
	PromptSnapshot   *generationprompt.PromptSnapshot
	SeedPolicy       string
	SeedValue        *int64
	OutputCount      int
	Budget           *Budget
}

type PlanFreezeResult struct {
	TaskPlan    *TaskPlan
	ActionPlans []ActionPlan
}

type PlanService interface {
	FreezePlan(input PlanFreezeInput) (*PlanFreezeResult, error)
	GetFrozenPlan(taskID string) (*TaskPlan, []ActionPlan, error)
	GetActionPlan(taskActionID string) (*ActionPlan, error)
}

type planService struct {
	taskPlanRepo   TaskPlanRepository
	actionPlanRepo ActionPlanRepository
}

func NewPlanService(taskPlanRepo TaskPlanRepository, actionPlanRepo ActionPlanRepository) PlanService {
	return &planService{
		taskPlanRepo:   taskPlanRepo,
		actionPlanRepo: actionPlanRepo,
	}
}

func (s *planService) FreezePlan(input PlanFreezeInput) (*PlanFreezeResult, error) {
	tx := input.Tx
	if tx == nil {
		return nil, NewGenerationError(ErrCodePlanInvalid, "transaction is required for freeze plan", nil)
	}

	existing, err := s.taskPlanRepo.GetTaskPlanByTaskIDTx(tx, input.TaskID)
	if err != nil {
		if !errors.Is(err, ErrTaskPlanNotFound) {
			return nil, err
		}
	}
	if existing != nil {
		actionPlans, err := s.actionPlanRepo.ListActionPlansByTaskID(input.TaskID)
		if err != nil {
			return nil, err
		}
		return &PlanFreezeResult{
			TaskPlan:    existing,
			ActionPlans: actionPlans,
		}, nil
	}

	frozenAt := nowRFC3339()
	actionPlans := make([]ActionPlan, 0, len(input.TaskActions))
	snapshots := make(map[string]GenerationPlanSnapshot, len(input.TaskActions))

	for _, ta := range input.TaskActions {
		planner := NewActionGenerationPlanner(ta.ActionKey, ta.FrameCount).
			WithMode(ta.Mode).
			WithProvider(input.Provider).
			WithModel(input.Model).
			WithCapabilities(ta.Capabilities).
			WithCapabilityHash(ta.CapabilityHash).
			WithLayoutResult(ta.LayoutResult).
			WithPromptSnapshot(ta.PromptSnapshot).
			WithSeedPolicy(ta.SeedPolicy).
			WithSeedValue(ta.SeedValue).
			WithOutputCount(ta.OutputCount).
			WithBudget(ta.Budget).
			WithConfig(input.ConfigID, input.ConfigRevision).
			WithReferenceAsset(input.ReferenceAssetID)

		snapshot, err := planner.Plan()
		if err != nil {
			return nil, err
		}

		snapshots[ta.ActionKey] = *snapshot

		planJSON, err := json.Marshal(snapshot)
		if err != nil {
			return nil, NewGenerationError(ErrCodePlanInvalid,
				fmt.Sprintf("failed to marshal action plan snapshot for action %s", ta.ActionKey), err)
		}

		actionPlan := ActionPlan{
			TaskID:                      input.TaskID,
			TaskActionID:                ta.TaskActionID,
			ActionKey:                   snapshot.ActionKey,
			SchemaVersion:               snapshot.SchemaVersion,
			PlanHash:                    snapshot.Hash,
			Mode:                        snapshot.Mode,
			Provider:                    snapshot.Provider,
			Model:                       snapshot.Model,
			ConfigID:                    snapshot.ConfigID,
			ConfigRevision:              snapshot.ConfigRevision,
			CapabilityHash:              snapshot.CapabilityHash,
			ReferenceAssetID:            snapshot.ReferenceAssetID,
			LayoutJSON:                  snapshot.LayoutJSON,
			LayoutHash:                  snapshot.LayoutHash,
			PromptSnapshot:              snapshot.PromptSnapshot,
			PromptHash:                  snapshot.PromptHash,
			NegativePromptSnapshot:      snapshot.NegativePromptSnapshot,
			NegativePromptHash:          snapshot.NegativePromptHash,
			SeedPolicy:                  snapshot.SeedPolicy,
			SeedValue:                   snapshot.SeedValue,
			OutputCount:                 snapshot.OutputCount,
			TargetFrameCount:            snapshot.TargetFrameCount,
			PlannedSegmentCount:         snapshot.PlannedSegmentCount,
			PlannedPrimaryRequestCount:  snapshot.PlannedPrimaryRequestCount,
			PlannedMaxProviderCallCount: snapshot.PlannedMaxProviderCallCount,
			PlannedCallCount:            snapshot.PlannedCallCount,
			SheetWidth:                  snapshot.SheetWidth,
			SheetHeight:                 snapshot.SheetHeight,
			CellWidth:                   snapshot.CellWidth,
			CellHeight:                  snapshot.CellHeight,
			FallbackMode:                snapshot.FallbackMode,
			ActionSpecVersion:           snapshot.ActionSpecVersion,
			ActionCatalogHash:           snapshot.ActionCatalogHash,
			ProviderConfigHash:          snapshot.ProviderConfigHash,
			SafetyPolicyVersion:         snapshot.SafetyPolicyVersion,
			OutputFormat:                snapshot.OutputFormat,
			PlanJSON:                    string(planJSON),
			FrozenAt:                    frozenAt,
		}

		actionPlans = append(actionPlans, actionPlan)
	}

	totalPrimaryRequests := 0
	totalMaxProviderCalls := 0
	for _, ap := range actionPlans {
		totalPrimaryRequests += ap.PlannedPrimaryRequestCount
		totalMaxProviderCalls += ap.PlannedMaxProviderCallCount
	}

	taskPlanHash := computeTaskPlanHash(actionPlans)

	taskPlanSnapshot := TaskPlanSnapshot{
		SchemaVersion:               TaskPlanSchemaVersion,
		TaskID:                      input.TaskID,
		Provider:                    input.Provider,
		Model:                       input.Model,
		ConfigID:                    input.ConfigID,
		ConfigRevision:              input.ConfigRevision,
		CapabilitySnapshotJSON:      input.CapabilitySnapshotJSON,
		CapabilitySnapshotHash:      input.CapabilitySnapshotHash,
		ReferenceAssetID:            input.ReferenceAssetID,
		PlannedPrimaryRequestCount:  totalPrimaryRequests,
		PlannedMaxProviderCallCount: totalMaxProviderCalls,
		ActionPlans:                 snapshots,
		Hash:                        taskPlanHash,
	}

	taskPlanJSON, err := json.Marshal(taskPlanSnapshot)
	if err != nil {
		return nil, NewGenerationError(ErrCodePlanInvalid, "failed to marshal task plan snapshot", err)
	}

	taskPlan := &TaskPlan{
		TaskID:                      input.TaskID,
		SchemaVersion:               TaskPlanSchemaVersion,
		PlanHash:                    taskPlanHash,
		Provider:                    input.Provider,
		Model:                       input.Model,
		ConfigID:                    input.ConfigID,
		ConfigRevision:              input.ConfigRevision,
		CapabilitySnapshotJSON:      input.CapabilitySnapshotJSON,
		CapabilitySnapshotHash:      input.CapabilitySnapshotHash,
		ReferenceAssetID:            input.ReferenceAssetID,
		PlannedPrimaryRequestCount:  totalPrimaryRequests,
		PlannedMaxProviderCallCount: totalMaxProviderCalls,
		PlanJSON:                    string(taskPlanJSON),
		FrozenAt:                    frozenAt,
	}

	if err := s.taskPlanRepo.CreateTaskPlanTx(tx, taskPlan); err != nil {
		return nil, err
	}

	for i := range actionPlans {
		actionPlans[i].TaskPlanID = taskPlan.ID
		if err := s.actionPlanRepo.CreateActionPlanTx(tx, &actionPlans[i]); err != nil {
			return nil, err
		}
	}

	return &PlanFreezeResult{
		TaskPlan:    taskPlan,
		ActionPlans: actionPlans,
	}, nil
}

func (s *planService) GetFrozenPlan(taskID string) (*TaskPlan, []ActionPlan, error) {
	taskPlan, err := s.taskPlanRepo.GetTaskPlanByTaskID(taskID)
	if err != nil {
		return nil, nil, err
	}
	actionPlans, err := s.actionPlanRepo.ListActionPlansByTaskID(taskID)
	if err != nil {
		return nil, nil, err
	}
	return taskPlan, actionPlans, nil
}

func (s *planService) GetActionPlan(taskActionID string) (*ActionPlan, error) {
	return s.actionPlanRepo.GetActionPlanByActionID(taskActionID)
}

func computeTaskPlanHash(plans []ActionPlan) string {
	hashes := make([]string, 0, len(plans))
	for _, p := range plans {
		hashes = append(hashes, p.PlanHash)
	}
	sort.Strings(hashes)
	return computeSHA256Hex(strings.Join(hashes, "|"))
}
