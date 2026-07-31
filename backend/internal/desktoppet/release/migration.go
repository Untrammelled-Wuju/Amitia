package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type LegacyPackageMigrationService struct {
	repo           ReleaseRepository
	storage        ReleaseStoragePort
	eventPublisher EventPublisher
}

func NewLegacyPackageMigrationService(
	repo ReleaseRepository,
	storage ReleaseStoragePort,
	eventPublisher EventPublisher,
) *LegacyPackageMigrationService {
	return &LegacyPackageMigrationService{
		repo:           repo,
		storage:        storage,
		eventPublisher: eventPublisher,
	}
}

type MigrateLegacyPackageRequest struct {
	LegacyPackageID   string
	UserID            string
	SourceContentHash string
	ManifestJSON      string
	LegacyVersion     int
	PetID             string
	CharacterID       string
	PackageName       string
}

type MigrateLegacyPackageResult struct {
	MigrationOperation *LegacyPackageMigrationOperation
	Mapping            *LegacyPackageMapping
	Release            *ReleaseData
}

func (s *LegacyPackageMigrationService) Migrate(ctx context.Context, req *MigrateLegacyPackageRequest) (*MigrateLegacyPackageResult, error) {
	existingMapping, _ := s.repo.GetLegacyPackageMapping(req.LegacyPackageID)
	if existingMapping != nil {
		if existingMapping.MigrationStatus == LegacyMigrationStatusMigrated {
			releaseData, err := s.repo.GetRelease(existingMapping.MigratedReleaseID)
			if err != nil {
				return nil, fmt.Errorf("查询已迁移 Release 失败: %w", err)
			}
			return &MigrateLegacyPackageResult{
				MigrationOperation: nil,
				Mapping:            existingMapping,
				Release:            releaseData,
			}, nil
		}
		if existingMapping.MigrationStatus == LegacyMigrationStatusPending {
			return s.resumeMigration(ctx, existingMapping, req)
		}
	}

	now := formatMigrationTimestamp(time.Now())
	opID := uuid.NewString()
	op := &LegacyPackageMigrationOperation{
		ID:              opID,
		LegacyPackageID: req.LegacyPackageID,
		UserID:          req.UserID,
		State:           LegacyMigrationOpStatePending,
		StartedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.CreateLegacyMigrationOperation(op); err != nil {
		return nil, fmt.Errorf("创建迁移操作失败: %w", err)
	}

	mappingID := uuid.NewString()
	mapping := &LegacyPackageMapping{
		ID:                mappingID,
		LegacyPackageID:   req.LegacyPackageID,
		SourceContentHash: req.SourceContentHash,
		MigrationStatus:   LegacyMigrationStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.CreateLegacyPackageMapping(mapping); err != nil {
		s.failMigrationOp(op, "MAPPING_CREATE_FAILED", err)
		return nil, err
	}

	return s.executeMigration(ctx, op, mapping, req)
}

func (s *LegacyPackageMigrationService) resumeMigration(ctx context.Context, mapping *LegacyPackageMapping, req *MigrateLegacyPackageRequest) (*MigrateLegacyPackageResult, error) {
	op := &LegacyPackageMigrationOperation{
		ID:              uuid.NewString(),
		LegacyPackageID: req.LegacyPackageID,
		UserID:          req.UserID,
		State:           LegacyMigrationOpStateValidating,
		StartedAt:       formatMigrationTimestamp(time.Now()),
		UpdatedAt:       formatMigrationTimestamp(time.Now()),
	}
	if err := s.repo.CreateLegacyMigrationOperation(op); err != nil {
		return nil, err
	}
	return s.executeMigration(ctx, op, mapping, req)
}

func (s *LegacyPackageMigrationService) executeMigration(ctx context.Context, op *LegacyPackageMigrationOperation, mapping *LegacyPackageMapping, req *MigrateLegacyPackageRequest) (*MigrateLegacyPackageResult, error) {
	op.State = LegacyMigrationOpStateValidating
	s.updateMigrationOp(op)

	manifest, err := s.parseLegacyManifest(req.ManifestJSON)
	if err != nil {
		s.failMigrationOp(op, "MANIFEST_PARSE_FAILED", err)
		s.updateMappingStatus(mapping, LegacyMigrationStatusFailed, err.Error())
		return nil, err
	}

	if manifest.SchemaVersion < 2 {
		op.State = LegacyMigrationOpStateRebuilding
		s.updateMigrationOp(op)
		manifest = s.upgradeManifestToV2(manifest, req)
	}

	op.State = LegacyMigrationOpStateRebuilding
	s.updateMigrationOp(op)

	petID := req.PetID
	if petID == "" {
		identity, err := s.repo.GetPetIdentityByCharacter(req.UserID, req.CharacterID)
		if err != nil {
			now := formatMigrationTimestamp(time.Now())
			identity = &PetIdentityData{
				ID:                uuid.NewString(),
				OwnerUserID:       req.UserID,
				SourceCharacterID: req.CharacterID,
				Name:              req.PackageName,
				Slug:              req.CharacterID,
				BindingPolicy:     "character_locked",
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if err := s.repo.CreatePetIdentity(identity); err != nil {
				s.failMigrationOp(op, "PET_IDENTITY_FAILED", err)
				return nil, err
			}
			petID = identity.ID
		} else {
			petID = identity.ID
		}
	}

	releaseID := uuid.NewString()
	now := formatMigrationTimestamp(time.Now())
	contentRootHash := req.SourceContentHash
	if contentRootHash == "" {
		contentRootHash = fmt.Sprintf("legacy-%s", req.LegacyPackageID)
	}

	releaseData := &ReleaseData{
		ID:                  releaseID,
		PetID:               petID,
		OwnerUserID:         req.UserID,
		Version:             fmt.Sprintf("1.0.%d", req.LegacyVersion),
		ReleaseSequence:     req.LegacyVersion,
		SchemaVersion:       2,
		Lifecycle:           string(ReleaseLifecycleReady),
		ContentRootHash:     contentRootHash,
		ManifestHash:        hashMigrationManifest(req.ManifestJSON),
		SourceType:          "migrated",
		LegacyPackageID:     req.LegacyPackageID,
		LegacyVersion:       req.LegacyVersion,
		DefaultActionKey:    manifest.DefaultAction,
		IntegrityStatus:     string(ReleaseIntegrityVerified),
		CompatibilityStatus: string(ReleaseCompatCompatible),
		ManifestJSON:        req.ManifestJSON,
		PublishedAt:         now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.repo.CreateRelease(releaseData); err != nil {
		s.failMigrationOp(op, "RELEASE_CREATE_FAILED", err)
		s.updateMappingStatus(mapping, LegacyMigrationStatusFailed, err.Error())
		return nil, err
	}

	mapping.MigratedPetID = petID
	mapping.MigratedReleaseID = releaseID
	mapping.MigrationStatus = LegacyMigrationStatusMigrated
	s.updateMappingStatus(mapping, LegacyMigrationStatusMigrated, "")

	op.State = LegacyMigrationOpStateCompleted
	op.CompletedAt = formatMigrationTimestamp(time.Now())
	s.updateMigrationOp(op)

	if s.eventPublisher != nil {
		s.eventPublisher.PublishReleaseEvent(ReleaseEvent{
			EventType:  EventLegacyPackageMigrated,
			UserID:     req.UserID,
			PetID:      petID,
			ReleaseID:  releaseID,
			OccurredAt: now,
		})
	}

	return &MigrateLegacyPackageResult{
		MigrationOperation: op,
		Mapping:            mapping,
		Release:            releaseData,
	}, nil
}

type legacyManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	PetID         string `json:"petId"`
	Name          string `json:"name"`
	DefaultAction string `json:"defaultAction"`
	Actions       []struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		Frames int    `json:"frameCount"`
	} `json:"actions"`
}

func (s *LegacyPackageMigrationService) parseLegacyManifest(manifestJSON string) (*legacyManifest, error) {
	if manifestJSON == "" {
		return &legacyManifest{SchemaVersion: 1}, nil
	}
	var m legacyManifest
	if err := json.Unmarshal([]byte(manifestJSON), &m); err != nil {
		return nil, fmt.Errorf("解析旧清单失败: %w", err)
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = 1
	}
	return &m, nil
}

func (s *LegacyPackageMigrationService) upgradeManifestToV2(m *legacyManifest, req *MigrateLegacyPackageRequest) *legacyManifest {
	m.SchemaVersion = 2
	if m.PetID == "" {
		m.PetID = req.PetID
	}
	if m.Name == "" {
		m.Name = req.PackageName
	}
	return m
}

func (s *LegacyPackageMigrationService) failMigrationOp(op *LegacyPackageMigrationOperation, code string, err error) {
	op.State = LegacyMigrationOpStateFailedTerm
	op.ErrorCode = code
	if err != nil {
		op.ErrorMessage = err.Error()
	}
	op.UpdatedAt = formatMigrationTimestamp(time.Now())
	s.repo.UpdateLegacyMigrationOperation(op)
}

func (s *LegacyPackageMigrationService) updateMigrationOp(op *LegacyPackageMigrationOperation) {
	op.UpdatedAt = formatMigrationTimestamp(time.Now())
	s.repo.UpdateLegacyMigrationOperation(op)
}

func (s *LegacyPackageMigrationService) updateMappingStatus(mapping *LegacyPackageMapping, status, errMsg string) {
	mapping.MigrationStatus = status
	mapping.ErrorMessage = errMsg
	mapping.UpdatedAt = formatMigrationTimestamp(time.Now())
	s.repo.UpdateLegacyPackageMapping(mapping)
}

func (s *LegacyPackageMigrationService) ListPendingMigrations() ([]*LegacyPackageMapping, error) {
	return s.repo.ListPendingLegacyMappings()
}

func (s *LegacyPackageMigrationService) GetMigrationStatus(legacyPackageID string) (*LegacyPackageMapping, error) {
	return s.repo.GetLegacyPackageMapping(legacyPackageID)
}

func formatMigrationTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func hashMigrationManifest(manifestJSON string) string {
	h := sha256.Sum256([]byte(manifestJSON))
	return hex.EncodeToString(h[:])
}
