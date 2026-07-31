package baseline

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/u-ai/backend/internal/desktoppet/editing"
)

type Repository interface {
	GetActionRevisionBySource(processingRevisionID, sourceType string) (*editing.ActionRevision, error)
	CreateActionRevision(rev *editing.ActionRevision) error
	GetActionRevisionForUser(userID, revisionID string) (*editing.ActionRevision, error)
	UpdateActionRevisionStatus(id, status string) error
	UpdateActionRevisionQuality(id, evaluationID, verdict string) error
	ListRevisionFrames(revisionID string) ([]editing.ActionRevisionFrame, error)
	CreateRevisionFrames(frames []editing.ActionRevisionFrame) error
	DB() *gorm.DB
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) GetActionRevisionBySource(processingRevisionID, sourceType string) (*editing.ActionRevision, error) {
	var rev editing.ActionRevision
	err := r.db.Where("source_processing_revision_id = ? AND source_type = ?", processingRevisionID, sourceType).First(&rev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rev, nil
}

func (r *repository) CreateActionRevision(rev *editing.ActionRevision) error {
	return r.db.Create(rev).Error
}

func (r *repository) GetActionRevisionForUser(userID, revisionID string) (*editing.ActionRevision, error) {
	var rev editing.ActionRevision
	err := r.db.Where("id = ? AND user_id = ?", revisionID, userID).First(&rev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, editing.ErrOwnershipDenied
		}
		return nil, err
	}
	return &rev, nil
}

func (r *repository) UpdateActionRevisionStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]any{"status": status, "updated_at": now}
	if status == RevisionStatusReady {
		updates["ready_at"] = now
	}
	return r.db.Model(&editing.ActionRevision{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) UpdateActionRevisionQuality(id, evaluationID, verdict string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.ActionRevision{}).Where("id = ?", id).Updates(map[string]any{
		"quality_evaluation_id": evaluationID,
		"quality_verdict":       verdict,
		"updated_at":            now,
	}).Error
}

func (r *repository) ListRevisionFrames(revisionID string) ([]editing.ActionRevisionFrame, error) {
	var frames []editing.ActionRevisionFrame
	err := r.db.Where("revision_id = ?", revisionID).Order("logical_index ASC").Find(&frames).Error
	return frames, err
}

func (r *repository) CreateRevisionFrames(frames []editing.ActionRevisionFrame) error {
	if len(frames) == 0 {
		return nil
	}
	return r.db.CreateInBatches(frames, 100).Error
}
