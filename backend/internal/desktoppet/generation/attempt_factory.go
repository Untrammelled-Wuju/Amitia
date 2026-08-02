package generation

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

type AttemptFactory struct {
	attemptRepo AttemptRepository
}

func NewAttemptFactory(attemptRepo AttemptRepository) *AttemptFactory {
	return &AttemptFactory{attemptRepo: attemptRepo}
}

type CreateAttemptInput struct {
	Tx              *gorm.DB
	TaskID          string
	TaskActionID    string
	Plan            *GenerationPlanSnapshot
	ParentAttemptID string
	Reason          string
	ExecutionID     string
	WorkerID        string
}

func (f *AttemptFactory) Create(input CreateAttemptInput) (*ActionGenerationAttempt, error) {
	if input.Plan == nil {
		return nil, NewGenerationError(ErrCodePlanInvalid, "plan snapshot is nil", nil)
	}
	if input.TaskActionID == "" {
		return nil, NewGenerationError(ErrCodeAttemptNotFound, "task action id is empty", nil)
	}

	planJSON, err := json.Marshal(input.Plan)
	if err != nil {
		return nil, NewGenerationError(ErrCodePlanInvalid, "failed to marshal plan snapshot", err)
	}

	attempt := &ActionGenerationAttempt{
		TaskID:                 input.TaskID,
		Mode:                   input.Plan.Mode,
		Reason:                 input.Reason,
		Status:                 string(AttemptStatusPending),
		Provider:               input.Plan.Provider,
		Model:                  input.Plan.Model,
		ConfigID:               input.Plan.ConfigID,
		ConfigRevision:         input.Plan.ConfigRevision,
		CapabilityHash:         input.Plan.CapabilityHash,
		ReferenceAssetID:       input.Plan.ReferenceAssetID,
		PlanJSON:               string(planJSON),
		PromptDocumentJSON:     input.Plan.PromptDocumentJSON,
		PromptSnapshot:         input.Plan.PromptSnapshot,
		PromptHash:             input.Plan.PromptHash,
		NegativePromptSnapshot: input.Plan.NegativePromptSnapshot,
		SeedPolicy:             input.Plan.SeedPolicy,
		SeedValue:              input.Plan.SeedValue,
		OutputCount:            input.Plan.OutputCount,
		ParentAttemptID:        input.ParentAttemptID,
		ExecutionID:            input.ExecutionID,
		WorkerID:               input.WorkerID,
	}

	created, err := f.attemptRepo.AtomicallyCreateAttempt(input.Tx, input.TaskActionID, attempt)
	if err != nil {
		return nil, fmt.Errorf("atomically create attempt: %w", err)
	}

	return created, nil
}

func (f *AttemptFactory) CreateRetry(tx *gorm.DB, taskID, taskActionID, parentAttemptID, reason, executionID, workerID string, plan *GenerationPlanSnapshot) (*ActionGenerationAttempt, error) {
	return f.Create(CreateAttemptInput{
		Tx:              tx,
		TaskID:          taskID,
		TaskActionID:    taskActionID,
		Plan:            plan,
		ParentAttemptID: parentAttemptID,
		Reason:          reason,
		ExecutionID:     executionID,
		WorkerID:        workerID,
	})
}

func (f *AttemptFactory) CreateInitial(tx *gorm.DB, taskID, taskActionID, executionID, workerID string, plan *GenerationPlanSnapshot) (*ActionGenerationAttempt, error) {
	return f.Create(CreateAttemptInput{
		Tx:           tx,
		TaskID:       taskID,
		TaskActionID: taskActionID,
		Plan:         plan,
		Reason:       "initial",
		ExecutionID:  executionID,
		WorkerID:     workerID,
	})
}
