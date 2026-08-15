package artifact

import (
	"gorm.io/gorm"
)

type Repository interface {
	Create(artifact *Artifact) error
	GetByID(id ID) (*Artifact, error)
	GetByOwnerAndID(ownerUserID string, id ID) (*Artifact, error)
	SoftDelete(id ID) error
	InsertReference(ref *ArtifactReference) error
	CountReferences(artifactID ID) (int64, error)
}

type sqliteRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) Create(artifact *Artifact) error {
	return r.db.Create(artifact).Error
}

func (r *sqliteRepository) GetByID(id ID) (*Artifact, error) {
	var a Artifact
	err := r.db.Where("artifact_id = ?", id).First(&a).Error
	return &a, err
}

func (r *sqliteRepository) GetByOwnerAndID(ownerUserID string, id ID) (*Artifact, error) {
	var a Artifact
	err := r.db.Where("artifact_id = ? AND owner_user_id = ?", id, ownerUserID).First(&a).Error
	return &a, err
}

func (r *sqliteRepository) SoftDelete(id ID) error {
	now := r.db.NowFunc()
	return r.db.Model(&Artifact{}).Where("artifact_id = ?", id).Updates(map[string]interface{}{
		"status":     StatusDeleted,
		"deleted_at": now,
		"revision":   gorm.Expr("revision + 1"),
	}).Error
}

func (r *sqliteRepository) InsertReference(ref *ArtifactReference) error {
	return r.db.Create(ref).Error
}

func (r *sqliteRepository) CountReferences(artifactID ID) (int64, error) {
	var count int64
	err := r.db.Model(&ArtifactReference{}).Where("artifact_id = ?", artifactID).Count(&count).Error
	return count, err
}
