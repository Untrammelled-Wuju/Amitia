// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"context"
	"errors"

	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"gorm.io/gorm"
)

type SQLiteOwnershipGuard struct {
	db *gorm.DB
}

func NewSQLiteOwnershipGuard(db *gorm.DB) *SQLiteOwnershipGuard {
	if db == nil {
		panic("NewSQLiteOwnershipGuard: db is nil")
	}
	return &SQLiteOwnershipGuard{db: db}
}

func (g *SQLiteOwnershipGuard) RequireGenerationTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*GenerationTaskScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if taskID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_generation_tasks").
		Select("space_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &GenerationTaskScope{
		SpaceID:     result.SpaceID,
		TaskID:      taskID,
		CharacterID: result.CharacterID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireProcessingTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*ProcessingTaskScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if taskID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_processing_tasks").
		Select("space_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &ProcessingTaskScope{
		SpaceID:     result.SpaceID,
		TaskID:      taskID,
		CharacterID: result.CharacterID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireActionRevision(ctx context.Context, actor *desktoppetAuth.ActorContext, revisionID string) (*ActionRevisionScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if revisionID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID        string `gorm:"column:space_id"`
		CharacterID    string `gorm:"column:character_id"`
		ActionStreamID string `gorm:"column:action_stream_id"`
		TaskID         string `gorm:"column:processing_task_id"`
	}
	err := g.db.WithContext(ctx).Table("desktop_pet_action_revisions").
		Select("space_id, character_id, action_stream_id, processing_task_id").
		Where("id = ?", revisionID).
		Take(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		var taskResult struct {
			SpaceID     string `gorm:"column:space_id"`
			CharacterID string `gorm:"column:character_id"`
		}
		if result.TaskID != "" {
			if taskErr := g.db.WithContext(ctx).Table("desktop_pet_processing_tasks").
				Select("space_id, character_id").
				Where("id = ?", result.TaskID).
				Take(&taskResult).Error; taskErr == nil && taskResult.SpaceID != "" {
				if taskResult.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
					return nil, ErrForbidden
				}
				return &ActionRevisionScope{
					SpaceID:          taskResult.SpaceID,
					CharacterID:      taskResult.CharacterID,
					RevisionID:       revisionID,
					ProcessingTaskID: result.TaskID,
				}, nil
			}
		}
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &ActionRevisionScope{
		SpaceID:          result.SpaceID,
		CharacterID:      result.CharacterID,
		RevisionID:       revisionID,
		ProcessingTaskID: result.TaskID,
	}, nil
}

func (g *SQLiteOwnershipGuard) resolveProcessingTaskOwner(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID, revisionID string) (*ActionRevisionScope, error) {
	if taskID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_processing_tasks").
		Select("space_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &ActionRevisionScope{
		SpaceID:          result.SpaceID,
		CharacterID:      result.CharacterID,
		RevisionID:       revisionID,
		ProcessingTaskID: taskID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireActionStream(ctx context.Context, actor *desktoppetAuth.ActorContext, streamID string) (*ActionStreamScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if streamID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	err := g.db.WithContext(ctx).Table("desktop_pet_action_streams").
		Select("space_id, character_id").
		Where("id = ?", streamID).
		Take(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &ActionStreamScope{
		SpaceID:     result.SpaceID,
		StreamID:    streamID,
		CharacterID: result.CharacterID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireQualityEvaluation(ctx context.Context, actor *desktoppetAuth.ActorContext, evaluationID string) (*QualityScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if evaluationID == "" {
		return nil, ErrNotFound
	}
	var evalResult struct {
		ActionRevisionID string `gorm:"column:action_revision_id"`
		ProcessingTaskID string `gorm:"column:processing_task_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_quality_evaluations").
		Select("action_revision_id, processing_task_id").
		Where("id = ?", evaluationID).
		Take(&evalResult).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if evalResult.ActionRevisionID == "" {
		return nil, ErrNotFound
	}

	var owner string
	var characterID string
	var revErr error
	if evalResult.ActionRevisionID != "" {
		owner, characterID, revErr = g.resolveActionRevisionOwner(ctx, evalResult.ActionRevisionID)
		if revErr != nil && !errors.Is(revErr, gorm.ErrRecordNotFound) {
			return nil, revErr
		}
	}
	if (owner == "" || errors.Is(revErr, gorm.ErrRecordNotFound)) && evalResult.ProcessingTaskID != "" {
		owner, characterID, revErr = g.resolveProcessingTaskOwnerLegacy(ctx, actor, evalResult.ProcessingTaskID)
		if revErr != nil && !errors.Is(revErr, gorm.ErrRecordNotFound) {
			return nil, revErr
		}
		_ = revErr
	}
	if owner == "" {
		return nil, ErrNotFound
	}
	if owner != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}

	return &QualityScope{
		SpaceID:          owner,
		EvaluationID:     evaluationID,
		ActionRevisionID: evalResult.ActionRevisionID,
		CharacterID:      characterID,
		ProcessingTaskID: evalResult.ProcessingTaskID,
	}, nil
}

func (g *SQLiteOwnershipGuard) resolveActionRevisionOwner(ctx context.Context, revisionID string) (string, string, error) {
	if revisionID == "" {
		return "", "", gorm.ErrRecordNotFound
	}
	var result struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	err := g.db.WithContext(ctx).Table("desktop_pet_action_revisions").
		Select("space_id, character_id").
		Where("id = ?", revisionID).
		Take(&result).Error
	if err != nil {
		return "", "", err
	}
	if result.SpaceID == "" {
		return "", "", gorm.ErrRecordNotFound
	}
	return result.SpaceID, result.CharacterID, nil
}

func (g *SQLiteOwnershipGuard) resolveProcessingTaskOwnerLegacy(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (string, string, error) {
	if taskID == "" {
		return "", "", gorm.ErrRecordNotFound
	}
	var result struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_processing_tasks").
		Select("space_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		return "", "", err
	}
	if result.SpaceID == "" {
		return "", "", gorm.ErrRecordNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return "", "", ErrForbidden
	}
	return result.SpaceID, result.CharacterID, nil
}

func (g *SQLiteOwnershipGuard) RequireRelease(ctx context.Context, actor *desktoppetAuth.ActorContext, releaseID string) (*ReleaseScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if releaseID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		Owner string `gorm:"column:owner_space_id"`
		PetID string `gorm:"column:pet_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_package_releases").
		Select("owner_space_id, pet_id").
		Where("id = ?", releaseID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.Owner == "" {
		return nil, ErrNotFound
	}
	if result.Owner != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &ReleaseScope{
		SpaceID:   result.Owner,
		ReleaseID: releaseID,
		PetID:     result.PetID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireInstallation(ctx context.Context, actor *desktoppetAuth.ActorContext, deviceID, installationID string) (*InstallationScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if installationID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID  string `gorm:"column:space_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_installations").
		Select("space_id, device_id").
		Where("id = ?", installationID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &InstallationScope{
		SpaceID:        result.SpaceID,
		DeviceID:       result.DeviceID,
		InstallationID: installationID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireInstallationStrict(ctx context.Context, actor *desktoppetAuth.ActorContext, deviceID, installationID string) (*InstallationScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if installationID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID  string `gorm:"column:space_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_installations").
		Select("space_id, device_id").
		Where("id = ?", installationID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	if deviceID == "" || result.DeviceID == "" || result.DeviceID != deviceID {
		return nil, ErrNotFound
	}
	return &InstallationScope{
		SpaceID:        result.SpaceID,
		DeviceID:       result.DeviceID,
		InstallationID: installationID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireEditSession(ctx context.Context, actor *desktoppetAuth.ActorContext, sessionID string) (*EditSessionScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if sessionID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID string `gorm:"column:space_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_edit_sessions").
		Select("space_id").
		Where("id = ?", sessionID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &EditSessionScope{
		SpaceID:   result.SpaceID,
		SessionID: sessionID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireRegenerationJob(ctx context.Context, actor *desktoppetAuth.ActorContext, jobID string) (*RegenerationJobScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if jobID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID   string `gorm:"column:space_id"`
		SessionID string `gorm:"column:session_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_regeneration_jobs").
		Select("space_id, session_id").
		Where("id = ?", jobID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &RegenerationJobScope{
		SpaceID:   result.SpaceID,
		JobID:     jobID,
		SessionID: result.SessionID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireCandidate(ctx context.Context, actor *desktoppetAuth.ActorContext, candidateID string) (*CandidateScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if candidateID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID   string `gorm:"column:owner_space_id"`
		JobID     string `gorm:"column:job_id"`
		SessionID string `gorm:"column:session_id"`
	}
	if err := g.db.WithContext(ctx).
		Table("desktop_pet_edit_candidates AS c").
		Select("c.owner_space_id, c.job_id, j.session_id").
		Joins("JOIN desktop_pet_regeneration_jobs AS j ON j.id = c.job_id").
		Where("c.id = ?", candidateID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &CandidateScope{
		SpaceID:     result.SpaceID,
		CandidateID: candidateID,
		JobID:       result.JobID,
		SessionID:   result.SessionID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireRuntimeCommand(ctx context.Context, actor *desktoppetAuth.ActorContext, commandID string) (*RuntimeCommandScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if commandID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID   string `gorm:"column:space_id"`
		DeviceID  string `gorm:"column:device_id"`
		RuntimeID string `gorm:"column:runtime_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_runtime_commands_v2").
		Select("space_id, device_id, runtime_id").
		Where("id = ?", commandID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.DeviceID == "" {
		return nil, ErrNotFound
	}
	if result.RuntimeID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &RuntimeCommandScope{
		SpaceID:   result.SpaceID,
		DeviceID:  result.DeviceID,
		RuntimeID: result.RuntimeID,
		CommandID: commandID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireBehaviorBinding(ctx context.Context, actor *desktoppetAuth.ActorContext, bindingID string) (*BehaviorBindingScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if bindingID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		SpaceID        string `gorm:"column:space_id"`
		InstallationID string `gorm:"column:installation_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_behavior_bindings").
		Select("space_id, installation_id").
		Where("id = ?", bindingID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.SpaceID == "" {
		return nil, ErrNotFound
	}
	if result.SpaceID != string(actor.SpaceID) && !actor.HasPermission(desktoppetAuth.PermSystemAdmin) {
		return nil, ErrForbidden
	}
	return &BehaviorBindingScope{
		SpaceID:        result.SpaceID,
		BindingID:      bindingID,
		InstallationID: result.InstallationID,
	}, nil
}
