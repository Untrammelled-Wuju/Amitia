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

func (g *SQLiteOwnershipGuard) RequireCharacter(ctx context.Context, actor *desktoppetAuth.ActorContext, characterID string) (*CharacterScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if characterID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		Owner string `gorm:"column:owner_user_id"`
	}
	err := g.db.WithContext(ctx).Table("desktop_pet_identities").
		Select("owner_user_id").
		Where("source_character_id = ?", characterID).
		Take(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.Owner == "" {
		return nil, ErrNotFound
	}
	if result.Owner != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &CharacterScope{UserID: result.Owner, CharacterID: characterID}, nil
}

func (g *SQLiteOwnershipGuard) RequireGenerationTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*GenerationTaskScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if taskID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_generation_tasks").
		Select("user_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &GenerationTaskScope{
		UserID:      result.UserID,
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
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_processing_tasks").
		Select("user_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &ProcessingTaskScope{
		UserID:      result.UserID,
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
	var activeResult struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	err := g.db.WithContext(ctx).Table("desktop_pet_action_active_revisions").
		Select("user_id, character_id").
		Where("active_action_revision_id = ?", revisionID).
		Take(&activeResult).Error
	if err == nil && activeResult.UserID != "" {
		if activeResult.UserID != actor.UserID && !actor.HasRole("admin") {
			return nil, ErrForbidden
		}
		return &ActionRevisionScope{
			UserID:      activeResult.UserID,
			CharacterID: activeResult.CharacterID,
			RevisionID:  revisionID,
		}, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var revResult struct {
		TaskID string `gorm:"column:processing_task_id"`
	}
	if taskErr := g.db.WithContext(ctx).Table("desktop_pet_action_revisions").
		Select("processing_task_id").
		Where("id = ?", revisionID).
		Take(&revResult).Error; taskErr != nil {
		if errors.Is(taskErr, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, taskErr
	}
	if revResult.TaskID == "" {
		return nil, ErrNotFound
	}
	return g.resolveProcessingTaskOwner(ctx, actor, revResult.TaskID, revisionID)
}

func (g *SQLiteOwnershipGuard) resolveProcessingTaskOwner(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID, revisionID string) (*ActionRevisionScope, error) {
	if taskID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_processing_tasks").
		Select("user_id, character_id").
		Where("id = ?", taskID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &ActionRevisionScope{
		UserID:           result.UserID,
		CharacterID:      result.CharacterID,
		RevisionID:       revisionID,
		ProcessingTaskID: taskID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireQualityEvaluation(ctx context.Context, actor *desktoppetAuth.ActorContext, evaluationID string) (*QualityScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if evaluationID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		ActionRevisionID string `gorm:"column:action_revision_id"`
		ProcessingTaskID string `gorm:"column:processing_task_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_quality_evaluations").
		Select("action_revision_id, processing_task_id").
		Where("id = ?", evaluationID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.ActionRevisionID == "" {
		return nil, ErrNotFound
	}
	return &QualityScope{
		EvaluationID:     evaluationID,
		ActionRevisionID: result.ActionRevisionID,
		ProcessingTaskID: result.ProcessingTaskID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireRelease(ctx context.Context, actor *desktoppetAuth.ActorContext, releaseID string) (*ReleaseScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if releaseID == "" {
		return nil, ErrNotFound
	}
	var result struct {
		Owner string `gorm:"column:owner_user_id"`
		PetID string `gorm:"column:pet_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_package_releases").
		Select("owner_user_id, pet_id").
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
	if result.Owner != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &ReleaseScope{
		UserID:    result.Owner,
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
		UserID   string `gorm:"column:user_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_installations").
		Select("user_id, device_id").
		Where("id = ?", installationID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &InstallationScope{
		UserID:         result.UserID,
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
		UserID string `gorm:"column:user_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_edit_sessions").
		Select("user_id").
		Where("id = ?", sessionID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &EditSessionScope{
		UserID:    result.UserID,
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
		UserID    string `gorm:"column:user_id"`
		SessionID string `gorm:"column:session_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_regeneration_jobs").
		Select("user_id, session_id").
		Where("id = ?", jobID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &RegenerationJobScope{
		UserID:    result.UserID,
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
		UserID string `gorm:"column:owner_user_id"`
		JobID  string `gorm:"column:job_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_edit_candidates").
		Select("owner_user_id, job_id").
		Where("id = ?", candidateID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &CandidateScope{
		UserID:      result.UserID,
		CandidateID: candidateID,
		JobID:       result.JobID,
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
		UserID   string `gorm:"column:user_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_runtime_commands").
		Select("user_id, device_id").
		Where("id = ?", commandID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &RuntimeCommandScope{
		UserID:    result.UserID,
		DeviceID:  result.DeviceID,
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
		UserID         string `gorm:"column:user_id"`
		InstallationID string `gorm:"column:installation_id"`
	}
	if err := g.db.WithContext(ctx).Table("desktop_pet_behavior_bindings").
		Select("user_id, installation_id").
		Where("id = ?", bindingID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID == "" {
		return nil, ErrNotFound
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &BehaviorBindingScope{
		UserID:         result.UserID,
		BindingID:      bindingID,
		InstallationID: result.InstallationID,
	}, nil
}
