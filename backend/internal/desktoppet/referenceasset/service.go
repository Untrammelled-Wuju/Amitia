package referenceasset

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type CreateReferenceAssetRequest struct {
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
}

type ReferenceAssetService interface {
	CreateForGenerationTask(ctx context.Context, tx *gorm.DB, req CreateReferenceAssetRequest) (*ReferenceAsset, error)
	ValidateForTask(ctx context.Context, taskID, userID, characterID string) (*ReferenceAsset, error)
}

type referenceAssetService struct {
	repo      Repository
	committer *Committer
	dataDir   string
}

func NewReferenceAssetService(repo Repository, committer *Committer, dataDir string) ReferenceAssetService {
	return &referenceAssetService{repo: repo, committer: committer, dataDir: dataDir}
}

func (s *referenceAssetService) CreateForGenerationTask(ctx context.Context, tx *gorm.DB, req CreateReferenceAssetRequest) (*ReferenceAsset, error) {
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.UploadPath == "" {
		return nil, fmt.Errorf("upload path is required")
	}

	normalizeConfig := NormalizeConfig{
		TargetWidth:     0,
		TargetHeight:    0,
		TargetMIME:      "image/png",
		MaxBytes:        10 * 1024 * 1024,
		BackgroundColor: "transparent",
	}

	profileID := req.NormalizeProfileID
	if profileID == "" {
		profileID = NormalizerProfileIDDefault
	}
	profileVersion := req.NormalizeProfileVersion
	if profileVersion == "" {
		profileVersion = NormalizerProfileVersionDefault
	}

	result, err := s.committer.Commit(CommitInput{
		Tx:                      tx,
		DataDir:                 s.dataDir,
		UserID:                  req.UserID,
		CharacterID:             req.CharacterID,
		TaskID:                  req.TaskID,
		UploadPath:              req.UploadPath,
		UploadName:              req.UploadName,
		UploadMIME:              req.UploadMIME,
		UploadHash:              req.UploadHash,
		UploadSize:              req.UploadSize,
		UploadWidth:             req.UploadWidth,
		UploadHeight:            req.UploadHeight,
		NormalizeProfileID:      profileID,
		NormalizeProfileVersion: profileVersion,
		NormalizeConfig:         normalizeConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("commit reference asset: %w", err)
	}

	return result.ReferenceAsset, nil
}

func (s *referenceAssetService) ValidateForTask(ctx context.Context, taskID, userID, characterID string) (*ReferenceAsset, error) {
	asset, err := s.repo.GetReferenceAssetByTaskID(taskID)
	if err != nil {
		return nil, fmt.Errorf("reference asset not found for task %s: %w", taskID, err)
	}
	if asset == nil {
		return nil, ErrReferenceAssetNotFound
	}
	if asset.Status != ReferenceAssetStatusPersisted {
		return nil, fmt.Errorf("reference asset status is not persisted: %s", asset.Status)
	}
	if userID != "" && asset.UserID != userID {
		return nil, fmt.Errorf("reference asset ownership mismatch: expected user %s, got %s", userID, asset.UserID)
	}
	if characterID != "" && asset.CharacterID != characterID {
		return nil, fmt.Errorf("reference asset character mismatch: expected %s, got %s", characterID, asset.CharacterID)
	}
	return asset, nil
}
