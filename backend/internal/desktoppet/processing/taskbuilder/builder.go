package taskbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing/source"
	"gorm.io/gorm"
)

type ProcessingRepo interface {
	CreateProcessingTask(tx *gorm.DB, task *processing.ProcessingTask) error
	CreateProcessingAction(tx *gorm.DB, action *processing.ProcessingAction) error
	DB() *gorm.DB
}

type TaskBuilder struct {
	repo            ProcessingRepo
	manifestBuilder *source.ManifestBuilder
	manifestStore   source.ManifestStore
	now             func() string
}

type CreateTaskRequest struct {
	GenerationTaskID   string
	UserID             string
	CharacterID        string
	OutputWidth        int
	OutputHeight       int
	TargetHeightRatio  float64
	AnchorMode         string
	BackgroundMode     string
	BackgroundProvider string
	ResampleMode       string
	Actions            []ActionInput
}

type ActionInput struct {
	GenerationTaskActionID        string
	ActionKey                     string
	ActionNameSnapshot            string
	GenerationActionID            string
	GenerationAttemptID           string
	SourceArtifactID              string
	ActiveArtifactBindingRevision int64
	ReferenceAssetID              string
	ReferenceAssetContentHash     string
	GenerationPlanID              string
	GenerationPlanHash            string
	PromptDocumentID              string
	PromptContentHash             string
	ActionSpecSnapshot            *source.ActionSpecSnapshot
	ActionSpecHash                string
	Descriptor                    *source.ProcessingSourceDescriptor
}

func NewTaskBuilder(repo ProcessingRepo, manifestStore source.ManifestStore) *TaskBuilder {
	return &TaskBuilder{
		repo:            repo,
		manifestBuilder: source.NewManifestBuilder(),
		manifestStore:   manifestStore,
		now:             func() string { return time.Now().UTC().Format("2006-01-02 15:04:05") },
	}
}

func (b *TaskBuilder) Build(ctx context.Context, req *CreateTaskRequest) (*processing.ProcessingTask, error) {
	if req == nil {
		return nil, fmt.Errorf("taskbuilder: request is nil")
	}
	if req.GenerationTaskID == "" {
		return nil, fmt.Errorf("taskbuilder: generation task id is empty")
	}
	if len(req.Actions) == 0 {
		return nil, fmt.Errorf("taskbuilder: actions is empty")
	}

	cfg := contracts.NewDefaultConfigSnapshot(
		req.OutputWidth,
		req.OutputHeight,
		req.TargetHeightRatio,
		req.AnchorMode,
		req.BackgroundMode,
		req.BackgroundProvider,
		req.ResampleMode,
	)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("taskbuilder: invalid config: %w", err)
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("taskbuilder: marshal config snapshot: %w", err)
	}

	now := b.now()
	taskID := "pt_" + uuid.NewString()

	task := &processing.ProcessingTask{
		ID:                         taskID,
		GenerationTaskID:           req.GenerationTaskID,
		UserID:                     req.UserID,
		CharacterID:                req.CharacterID,
		Status:                     "queued",
		CurrentStage:               "created",
		Progress:                   0,
		OutputWidth:                req.OutputWidth,
		OutputHeight:               req.OutputHeight,
		TargetCharacterHeightRatio: req.TargetHeightRatio,
		AnchorMode:                 req.AnchorMode,
		BackgroundMode:             req.BackgroundMode,
		ConfigSnapshot:             string(cfgJSON),
		ConfigHash:                 cfg.ConfigHash,
		PipelineVersion:            contracts.PipelineVersion,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}

	var createdTask *processing.ProcessingTask

	err = b.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := b.repo.CreateProcessingTask(tx, task); err != nil {
			return fmt.Errorf("taskbuilder: create processing task: %w", err)
		}

		for i, ai := range req.Actions {
			if ai.ActionKey == "" {
				return fmt.Errorf("taskbuilder: action %d has empty action key", i)
			}
			if ai.Descriptor == nil {
				return fmt.Errorf("taskbuilder: action %s has nil descriptor", ai.ActionKey)
			}

			actionSpecSnapshotJSON := "{}"
			if ai.ActionSpecSnapshot != nil {
				data, mErr := json.Marshal(ai.ActionSpecSnapshot)
				if mErr != nil {
					return fmt.Errorf("taskbuilder: marshal action spec snapshot for %s: %w", ai.ActionKey, mErr)
				}
				actionSpecSnapshotJSON = string(data)
			}

			actionID := "pa_" + uuid.NewString()
			action := &processing.ProcessingAction{
				ID:                     actionID,
				ProcessingTaskID:       taskID,
				GenerationTaskActionID: ai.GenerationTaskActionID,
				ActionKey:              ai.ActionKey,
				ActionNameSnapshot:     ai.ActionNameSnapshot,
				Status:                 "pending",
				CurrentStage:           "created",
				ActionSpecSnapshot:     actionSpecSnapshotJSON,
				ActionSpecHash:         ai.ActionSpecHash,
				AttemptNumber:          1,
				NextRevisionNumber:     1,
				CreatedAt:              now,
				UpdatedAt:              now,
			}

			if err := b.repo.CreateProcessingAction(tx, action); err != nil {
				return fmt.Errorf("taskbuilder: create processing action %s: %w", ai.ActionKey, err)
			}

			manifestRecord, mErr := b.manifestBuilder.Build(source.BuildManifestRequest{
				ID:                            "pm_" + uuid.NewString(),
				UserID:                        req.UserID,
				CharacterID:                   req.CharacterID,
				ProcessingTaskID:              taskID,
				ProcessingActionID:            actionID,
				GenerationTaskID:              req.GenerationTaskID,
				GenerationActionID:            ai.GenerationActionID,
				Descriptor:                    ai.Descriptor,
				ActiveArtifactBindingRevision: ai.ActiveArtifactBindingRevision,
				ReferenceAssetID:              ai.ReferenceAssetID,
				ReferenceAssetContentHash:     ai.ReferenceAssetContentHash,
				GenerationPlanID:              ai.GenerationPlanID,
				GenerationPlanHash:            ai.GenerationPlanHash,
				PromptDocumentID:              ai.PromptDocumentID,
				PromptContentHash:             ai.PromptContentHash,
				ActionSpecSnapshot:            ai.ActionSpecSnapshot,
				ActionSpecHash:                ai.ActionSpecHash,
			})
			if mErr != nil {
				return fmt.Errorf("taskbuilder: build manifest for action %s: %w", ai.ActionKey, mErr)
			}

			if err := b.manifestStore.Create(ctx, manifestRecord); err != nil {
				return fmt.Errorf("taskbuilder: store manifest for action %s: %w", ai.ActionKey, err)
			}
		}

		createdTask = task
		return nil
	})
	if err != nil {
		return nil, err
	}

	return createdTask, nil
}
