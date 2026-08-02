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
	return &SQLiteOwnershipGuard{db: db}
}

func (g *SQLiteOwnershipGuard) RequireCharacter(ctx context.Context, actor *desktoppetAuth.ActorContext, characterID string) (*CharacterScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID string `gorm:"column:user_id"`
	}
	err := g.db.WithContext(ctx).Raw(`
		SELECT owner_user_id as user_id FROM desktop_pet_identities WHERE source_character_id = ?
	`, characterID).Scan(&result).Error
	if err != nil {
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
	return &CharacterScope{UserID: result.UserID, CharacterID: characterID}, nil
}

func (g *SQLiteOwnershipGuard) RequireGenerationTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*GenerationTaskScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID         string `gorm:"column:user_id"`
		CharacterID    string `gorm:"column:character_id"`
		ActionStreamID string `gorm:"column:action_stream_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, character_id, action_stream_id FROM desktop_pet_generation_tasks WHERE id = ?", taskID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &GenerationTaskScope{
		UserID:         result.UserID,
		TaskID:         taskID,
		CharacterID:    result.CharacterID,
		ActionStreamID: result.ActionStreamID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireProcessingTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*ProcessingTaskScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, character_id FROM desktop_pet_processing_tasks WHERE id = ?", taskID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
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
	var result struct {
		UserID           string `gorm:"column:user_id"`
		CharacterID      string `gorm:"column:character_id"`
		ActionStreamID   string `gorm:"column:action_stream_id"`
		ProcessingTaskID string `gorm:"column:processing_task_id"`
	}
	if err := g.db.WithContext(ctx).Raw(`
		SELECT r.user_id, r.character_id, r.action_stream_id, r.processing_task_id
		FROM desktop_pet_action_revisions r WHERE r.id = ?
	`, revisionID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &ActionRevisionScope{
		UserID:           result.UserID,
		CharacterID:      result.CharacterID,
		ActionStreamID:   result.ActionStreamID,
		RevisionID:       revisionID,
		ProcessingTaskID: result.ProcessingTaskID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireQualityEvaluation(ctx context.Context, actor *desktoppetAuth.ActorContext, evaluationID string) (*QualityScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID      string `gorm:"column:user_id"`
		RevisionID  string `gorm:"column:revision_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := g.db.WithContext(ctx).Raw(`
		SELECT e.user_id, e.revision_id, e.character_id
		FROM desktop_pet_quality_evaluations e WHERE e.id = ?
	`, evaluationID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &QualityScope{
		UserID:       result.UserID,
		EvaluationID: evaluationID,
		RevisionID:   result.RevisionID,
		CharacterID:  result.CharacterID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireRelease(ctx context.Context, actor *desktoppetAuth.ActorContext, releaseID string) (*ReleaseScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID string `gorm:"column:owner_user_id"`
		PetID  string `gorm:"column:pet_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT owner_user_id, pet_id FROM desktop_pet_releases WHERE id = ?", releaseID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &ReleaseScope{
		UserID:    result.UserID,
		ReleaseID: releaseID,
		PetID:     result.PetID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireInstallation(ctx context.Context, actor *desktoppetAuth.ActorContext, deviceID, installationID string) (*InstallationScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID   string `gorm:"column:user_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, device_id FROM desktop_pet_installations WHERE id = ?", installationID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
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
	var result struct {
		UserID string `gorm:"column:user_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id FROM desktop_pet_edit_sessions WHERE id = ?", sessionID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
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
	var result struct {
		UserID     string `gorm:"column:user_id"`
		SessionID  string `gorm:"column:session_id"`
		RevisionID string `gorm:"column:revision_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, session_id, revision_id FROM desktop_pet_regeneration_jobs WHERE id = ?", jobID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &RegenerationJobScope{
		UserID:     result.UserID,
		JobID:      jobID,
		SessionID:  result.SessionID,
		RevisionID: result.RevisionID,
	}, nil
}

func (g *SQLiteOwnershipGuard) RequireCandidate(ctx context.Context, actor *desktoppetAuth.ActorContext, candidateID string) (*CandidateScope, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var result struct {
		UserID string `gorm:"column:user_id"`
		JobID  string `gorm:"column:job_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, job_id FROM desktop_pet_candidates WHERE id = ?", candidateID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
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
	var result struct {
		UserID   string `gorm:"column:user_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, device_id FROM desktop_pet_runtime_commands WHERE id = ?", commandID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
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
	var result struct {
		UserID   string `gorm:"column:user_id"`
		DeviceID string `gorm:"column:device_id"`
	}
	if err := g.db.WithContext(ctx).Raw("SELECT user_id, device_id FROM desktop_pet_behavior_bindings WHERE id = ?", bindingID).Scan(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if result.UserID != actor.UserID && !actor.HasRole("admin") {
		return nil, ErrForbidden
	}
	return &BehaviorBindingScope{
		UserID:    result.UserID,
		BindingID: bindingID,
		DeviceID:  result.DeviceID,
	}, nil
}
