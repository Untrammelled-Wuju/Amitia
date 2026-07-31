package activebinding

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrBindingNotFound        = errors.New("active binding not found")
	ErrBindingCASConflict     = errors.New("active binding cas conflict")
	ErrAttemptNotSucceeded    = errors.New("attempt status is not succeeded")
	ErrArtifactNotPersisted   = errors.New("artifact status is not persisted")
	ErrArtifactHashMismatch   = errors.New("artifact hash mismatch")
	ErrArtifactNotPrimary     = errors.New("artifact is not primary")
	ErrAttemptActionMismatch  = errors.New("attempt does not belong to task action")
	ErrArtifactActionMismatch = errors.New("artifact does not belong to task action")
)

type BindRequest struct {
	TaskActionID      string
	AttemptID         string
	PrimaryArtifactID string
	ArtifactHash      string
	Reason            string
}

type BindingService struct {
	repo Repository
}

func NewBindingService(repo Repository) *BindingService {
	return &BindingService{repo: repo}
}

func (s *BindingService) BindActiveArtifact(tx *gorm.DB, req BindRequest) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if req.TaskActionID == "" {
		return fmt.Errorf("task action id is required")
	}
	if req.AttemptID == "" {
		return fmt.Errorf("attempt id is required")
	}
	if req.PrimaryArtifactID == "" {
		return fmt.Errorf("primary artifact id is required")
	}

	var attempt struct {
		ID           string `gorm:"column:id"`
		TaskActionID string `gorm:"column:task_action_id"`
		Status       string `gorm:"column:status"`
	}
	err := tx.Table("desktop_pet_action_generation_attempts").
		Where("id = ?", req.AttemptID).
		First(&attempt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("attempt not found: %s", req.AttemptID)
		}
		return fmt.Errorf("query attempt: %w", err)
	}
	if attempt.Status != "succeeded" {
		return fmt.Errorf("%w: got %s", ErrAttemptNotSucceeded, attempt.Status)
	}
	if attempt.TaskActionID != req.TaskActionID {
		return fmt.Errorf("%w: expected %s, got %s", ErrAttemptActionMismatch, req.TaskActionID, attempt.TaskActionID)
	}

	var artifact struct {
		ID           string `gorm:"column:id"`
		TaskActionID string `gorm:"column:task_action_id"`
		AttemptID    string `gorm:"column:attempt_id"`
		IsPrimary    int    `gorm:"column:is_primary"`
		Status       string `gorm:"column:status"`
		Hash         string `gorm:"column:hash"`
		ContentHash  string `gorm:"column:content_hash"`
	}
	err = tx.Table("desktop_pet_generation_artifacts").
		Where("id = ?", req.PrimaryArtifactID).
		First(&artifact).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("artifact not found: %s", req.PrimaryArtifactID)
		}
		return fmt.Errorf("query artifact: %w", err)
	}
	if artifact.TaskActionID != req.TaskActionID {
		return fmt.Errorf("%w: expected %s, got %s", ErrArtifactActionMismatch, req.TaskActionID, artifact.TaskActionID)
	}
	if artifact.IsPrimary != 1 {
		return fmt.Errorf("%w: %s", ErrArtifactNotPrimary, req.PrimaryArtifactID)
	}
	if !isArtifactPersisted(artifact.Status) {
		return fmt.Errorf("%w: got %s", ErrArtifactNotPersisted, artifact.Status)
	}

	artifactHash := artifact.Hash
	if artifactHash == "" {
		artifactHash = artifact.ContentHash
	}
	if artifactHash != "" && req.ArtifactHash != "" && artifactHash != req.ArtifactHash {
		return fmt.Errorf("%w: expected %s, got %s", ErrArtifactHashMismatch, artifactHash, req.ArtifactHash)
	}

	existing, err := s.repo.GetByActionID(req.TaskActionID)
	if err != nil {
		return fmt.Errorf("get existing active binding: %w", err)
	}

	now := nowRFC3339()
	newBinding := &ActiveBinding{
		GenerationActionID:      req.TaskActionID,
		ActiveAttemptID:         req.AttemptID,
		ActivePrimaryArtifactID: req.PrimaryArtifactID,
		ArtifactContentHash:     req.ArtifactHash,
		BoundReason:             req.Reason,
		BoundAt:                 now,
		UpdatedAt:               now,
	}

	if existing == nil {
		newBinding.BindingRevision = 1
		newBinding.CreatedAt = now
		if err := s.repo.Create(tx, newBinding); err != nil {
			return fmt.Errorf("create active binding: %w", err)
		}
		return nil
	}

	newBinding.CreatedAt = existing.CreatedAt
	ok, err := s.repo.CASUpdate(tx, req.TaskActionID, existing.BindingRevision, newBinding)
	if err != nil {
		return fmt.Errorf("cas update active binding: %w", err)
	}
	if !ok {
		return ErrBindingCASConflict
	}
	return nil
}

func (s *BindingService) GetActiveBinding(actionID string) (*ActiveBinding, error) {
	binding, err := s.repo.GetByActionID(actionID)
	if err != nil {
		return nil, fmt.Errorf("get active binding: %w", err)
	}
	if binding == nil {
		return nil, ErrBindingNotFound
	}
	return binding, nil
}

func isArtifactPersisted(status string) bool {
	switch status {
	case "persisted", "saved", "verified":
		return true
	default:
		return false
	}
}
