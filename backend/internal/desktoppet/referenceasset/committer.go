package referenceasset

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReferenceAssetPublishJournal struct {
	ID                   string `gorm:"column:id;primaryKey;type:text" json:"id"`
	ReferenceAssetID     string `gorm:"column:reference_asset_id;type:text" json:"referenceAssetId"`
	StagingPath          string `gorm:"column:staging_path;type:text" json:"stagingPath"`
	FinalPath            string `gorm:"column:final_path;type:text" json:"finalPath"`
	SourceStorageKey     string `gorm:"column:source_storage_key;type:text" json:"sourceStorageKey"`
	NormalizedStorageKey string `gorm:"column:normalized_storage_key;type:text" json:"normalizedStorageKey"`
	ContentHash          string `gorm:"column:content_hash;type:text" json:"contentHash"`
	JournalStatus        string `gorm:"column:journal_status;type:text;default:'staging'" json:"journalStatus"`
	ErrorMessage         string `gorm:"column:error_message;type:text;default:''" json:"errorMessage"`
	CreatedAt            string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt          string `gorm:"column:completed_at;type:text;default:''" json:"completedAt"`
}

func (ReferenceAssetPublishJournal) TableName() string {
	return "desktop_pet_reference_asset_publish_journals"
}

const (
	JournalStatusStaging       = "staging"
	JournalStatusPersisted     = "persisted"
	JournalStatusPublishFailed = "publish_failed"
)

type JournalRepository interface {
	Create(tx *gorm.DB, journal *ReferenceAssetPublishJournal) error
	UpdateStatus(tx *gorm.DB, journalID string, status string, errMsg string) error
	GetByReferenceAssetID(referenceAssetID string) (*ReferenceAssetPublishJournal, error)
}

type journalRepository struct {
	db *gorm.DB
}

func NewJournalRepository(db *gorm.DB) JournalRepository {
	return &journalRepository{db: db}
}

func (r *journalRepository) Create(tx *gorm.DB, journal *ReferenceAssetPublishJournal) error {
	if tx == nil {
		tx = r.db
	}
	if journal.ID == "" {
		journal.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if journal.CreatedAt == "" {
		journal.CreatedAt = now
	}
	if journal.UpdatedAt == "" {
		journal.UpdatedAt = now
	}
	return tx.Create(journal).Error
}

func (r *journalRepository) UpdateStatus(tx *gorm.DB, journalID string, status string, errMsg string) error {
	if tx == nil {
		tx = r.db
	}
	updates := map[string]interface{}{
		"journal_status": status,
		"error_message":  errMsg,
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}
	if status == JournalStatusPersisted {
		updates["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	return tx.Model(&ReferenceAssetPublishJournal{}).Where("id = ?", journalID).Updates(updates).Error
}

func (r *journalRepository) GetByReferenceAssetID(referenceAssetID string) (*ReferenceAssetPublishJournal, error) {
	var journal ReferenceAssetPublishJournal
	err := r.db.Where("reference_asset_id = ?", referenceAssetID).First(&journal).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &journal, nil
}

type CommitInput struct {
	Tx                      *gorm.DB
	DataDir                 string
	UserID                  string
	CharacterID             string
	TaskID                  string
	UploadPath              string
	UploadName              string
	UploadMIME              string
	UploadHash              string
	UploadSize              int64
	UploadWidth             int
	UploadHeight            int
	NormalizeProfileID      string
	NormalizeProfileVersion string
	NormalizeConfig         NormalizeConfig
}

type CommitResult struct {
	ReferenceAsset *ReferenceAsset
	Journal        *ReferenceAssetPublishJournal
}

type Committer struct {
	repo        Repository
	journalRepo JournalRepository
}

func NewCommitter(repo Repository, journalRepo JournalRepository) *Committer {
	return &Committer{repo: repo, journalRepo: journalRepo}
}

func (c *Committer) Commit(input CommitInput) (*CommitResult, error) {
	if input.DataDir == "" {
		return nil, fmt.Errorf("data dir is empty")
	}
	if input.UploadPath == "" {
		return nil, fmt.Errorf("upload path is empty")
	}
	if input.TaskID == "" {
		return nil, fmt.Errorf("task id is empty")
	}

	assetID := uuid.New().String()

	sourceStorageKey := filepath.ToSlash(filepath.Join("desktop-pets", "reference-assets", assetID, "source"+filepath.Ext(input.UploadName)))
	normalizedStorageKey := filepath.ToSlash(filepath.Join("desktop-pets", "reference-assets", assetID, "normalized.png"))

	sourceAbsPath := filepath.Join(input.DataDir, sourceStorageKey)
	normalizedAbsPath := filepath.Join(input.DataDir, normalizedStorageKey)

	uploadAbsPath := input.UploadPath
	if !filepath.IsAbs(uploadAbsPath) {
		uploadAbsPath = filepath.Join(input.DataDir, uploadAbsPath)
	}

	if err := EnsureDir(filepath.Dir(sourceAbsPath)); err != nil {
		return nil, fmt.Errorf("create source asset dir: %w", err)
	}

	if err := copyFile(uploadAbsPath, sourceAbsPath); err != nil {
		return nil, fmt.Errorf("copy source file to asset dir: %w", err)
	}

	asset, err := Normalize(NormalizeInput{
		SourcePath:               sourceAbsPath,
		OutputPath:               normalizedAbsPath,
		Config:                   input.NormalizeConfig,
		UserID:                   input.UserID,
		CharacterID:              input.CharacterID,
		TaskID:                   input.TaskID,
		NormalizerProfileID:      input.NormalizeProfileID,
		NormalizerProfileVersion: input.NormalizeProfileVersion,
	})
	if err != nil {
		_ = os.Remove(sourceAbsPath)
		return nil, fmt.Errorf("normalize reference asset: %w", err)
	}

	asset.ID = assetID
	asset.SourceArtifactID = assetID
	asset.SourcePath = sourceStorageKey
	asset.NormalizedPath = normalizedStorageKey
	asset.StoragePath = normalizedStorageKey
	asset.Status = ReferenceAssetStatusStaging

	tx := input.Tx
	if tx == nil {
		tx = c.repo.(*repository).db.Begin()
		ownTx := true
		defer func() {
			if ownTx {
				if err != nil {
					tx.Rollback()
				}
			}
		}()
	}

	if err := c.repo.CreateReferenceAssetTx(tx, asset); err != nil {
		_ = os.Remove(sourceAbsPath)
		_ = os.Remove(normalizedAbsPath)
		return nil, fmt.Errorf("create reference asset record: %w", err)
	}

	journal := &ReferenceAssetPublishJournal{
		ReferenceAssetID:     assetID,
		StagingPath:          sourceAbsPath,
		FinalPath:            normalizedAbsPath,
		SourceStorageKey:     sourceStorageKey,
		NormalizedStorageKey: normalizedStorageKey,
		ContentHash:          asset.ContentHash,
		JournalStatus:        JournalStatusStaging,
	}

	if err := c.journalRepo.Create(tx, journal); err != nil {
		_ = os.Remove(sourceAbsPath)
		_ = os.Remove(normalizedAbsPath)
		return nil, fmt.Errorf("create publish journal: %w", err)
	}

	if err := c.repo.UpdateReferenceAssetStatus(tx, assetID, ReferenceAssetStatusPersisted); err != nil {
		_ = c.journalRepo.UpdateStatus(tx, journal.ID, JournalStatusPublishFailed, fmt.Sprintf("update status to persisted: %v", err))
		return nil, fmt.Errorf("update reference asset to persisted: %w", err)
	}

	if err := c.journalRepo.UpdateStatus(tx, journal.ID, JournalStatusPersisted, ""); err != nil {
		return nil, fmt.Errorf("update journal to persisted: %w", err)
	}

	asset.Status = ReferenceAssetStatusPersisted

	return &CommitResult{
		ReferenceAsset: asset,
		Journal:        journal,
	}, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
