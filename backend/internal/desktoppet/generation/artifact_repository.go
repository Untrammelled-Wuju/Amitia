package generation

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrArtifactNotFound = NewGenerationError(ErrCodeArtifactNotFound, "artifact not found", nil)
)

type ArtifactRepository interface {
	CreateArtifact(tx *gorm.DB, artifact *GenerationArtifact) error
	GetArtifactByID(id string) (*GenerationArtifact, error)
	ListArtifactsByAttemptID(attemptID string) ([]GenerationArtifact, error)
	ListPrimaryArtifactsByActionID(taskActionID string) ([]GenerationArtifact, error)
	UpdateArtifact(id string, updates map[string]interface{}) error
	UpdateArtifactTx(tx *gorm.DB, id string, updates map[string]interface{}) error
	HasPrimaryArtifact(attemptID string) (bool, error)
	DeleteArtifactsByTaskID(tx *gorm.DB, taskID string) error
}

type artifactRepository struct {
	db *gorm.DB
}

func NewArtifactRepository(db *gorm.DB) ArtifactRepository {
	return &artifactRepository{db: db}
}

func (r *artifactRepository) CreateArtifact(tx *gorm.DB, artifact *GenerationArtifact) error {
	if tx == nil {
		tx = r.db
	}
	if artifact.ID == "" {
		artifact.ID = generateUUID()
	}
	if artifact.CreatedAt == "" {
		artifact.CreatedAt = nowRFC3339()
	}
	if artifact.UpdatedAt == "" {
		artifact.UpdatedAt = nowRFC3339()
	}
	if err := tx.Create(artifact).Error; err != nil {
		return fmt.Errorf("create artifact: %w", err)
	}
	return nil
}

func (r *artifactRepository) GetArtifactByID(id string) (*GenerationArtifact, error) {
	var artifact GenerationArtifact
	err := r.db.Where("id = ?", id).First(&artifact).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, err
	}
	return &artifact, nil
}

func (r *artifactRepository) ListArtifactsByAttemptID(attemptID string) ([]GenerationArtifact, error) {
	var artifacts []GenerationArtifact
	err := r.db.Where("attempt_id = ?", attemptID).Order("segment_index ASC, candidate_index ASC").Find(&artifacts).Error
	if err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (r *artifactRepository) ListPrimaryArtifactsByActionID(taskActionID string) ([]GenerationArtifact, error) {
	var artifacts []GenerationArtifact
	err := r.db.Where("task_action_id = ? AND is_primary = 1", taskActionID).
		Order("segment_index ASC").Find(&artifacts).Error
	if err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (r *artifactRepository) UpdateArtifact(id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	if _, ok := updates["updated_at"]; !ok {
		updates["updated_at"] = nowRFC3339()
	}
	result := r.db.Model(&GenerationArtifact{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrArtifactNotFound
	}
	return nil
}

func (r *artifactRepository) UpdateArtifactTx(tx *gorm.DB, id string, updates map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}
	if len(updates) == 0 {
		return nil
	}
	if _, ok := updates["updated_at"]; !ok {
		updates["updated_at"] = nowRFC3339()
	}
	result := tx.Model(&GenerationArtifact{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrArtifactNotFound
	}
	return nil
}

func (r *artifactRepository) HasPrimaryArtifact(attemptID string) (bool, error) {
	var count int64
	err := r.db.Model(&GenerationArtifact{}).
		Where("attempt_id = ? AND is_primary = 1", attemptID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *artifactRepository) DeleteArtifactsByTaskID(tx *gorm.DB, taskID string) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.Where("task_id = ?", taskID).Delete(&GenerationArtifact{}).Error; err != nil {
		return fmt.Errorf("delete artifacts by task id: %w", err)
	}
	return nil
}
