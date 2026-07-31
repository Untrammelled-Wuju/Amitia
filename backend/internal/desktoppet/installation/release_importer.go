package installation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"gorm.io/gorm"
)

type ReleaseImporter struct {
	repo          Repository
	storage       *ReleaseStorage
	writer        *packageformat.V2Writer
	validator     *packageformat.Validator
	archiveWriter *packageformat.ArchiveWriter
}

func NewReleaseImporter(repo Repository, storage *ReleaseStorage) *ReleaseImporter {
	return &ReleaseImporter{
		repo:          repo,
		storage:       storage,
		writer:        &packageformat.V2Writer{},
		validator:     packageformat.NewValidator(),
		archiveWriter: &packageformat.ArchiveWriter{},
	}
}

func (im *ReleaseImporter) ImportPackage(req *ImportPackageRequest) (*ImportPackageResult, error) {
	if req.IdempotencyKey != "" {
		existing, err := im.repo.GetPackageOperationByIdempotencyKey(req.UserID, req.IdempotencyKey, PackageOpTypeImport)
		if err == nil && existing.Status == OpStatusCompleted {
			return im.loadExistingImportResult(existing)
		}
	}

	now := time.Now().Format(installationTimeFormat)
	opID := uuid.NewString()
	op := &PackageOperation{
		ID:             opID,
		OperationType:  PackageOpTypeImport,
		UserID:         req.UserID,
		IdempotencyKey: req.IdempotencyKey,
		Stage:          OpStagePrepare,
		Status:         OpStatusRunning,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := im.repo.CreatePackageOperation(op); err != nil {
		return nil, err
	}

	identity, err := im.resolvePetIdentity(req)
	if err != nil {
		failPackageOperation(im.repo, op, OpStagePrepare, "PET_IDENTITY_FAILED", err.Error())
		return nil, err
	}

	petID := identity.ID
	op.PetID = petID
	_ = im.repo.UpdatePackageOperation(op)

	releaseSeq, err := im.repo.GetLatestReleaseSequence(petID)
	if err != nil {
		failPackageOperation(im.repo, op, OpStagePrepare, "SEQUENCE_FAILED", err.Error())
		return nil, err
	}
	releaseSeq++

	releaseID := uuid.NewString()
	version := fmt.Sprintf("1.0.%d", releaseSeq)

	op.Stage = OpStageStageFiles
	op.ReleaseID = releaseID
	op.StagingPathKey = releaseID
	_ = im.repo.UpdatePackageOperation(op)

	stagingDir := im.storage.StagingDir(releaseID)
	if err := im.storage.EnsureStagingDir(releaseID); err != nil {
		failPackageOperation(im.repo, op, OpStageStageFiles, "STAGING_DIR_FAILED", err.Error())
		return nil, err
	}

	safePackageDir, err := im.storage.ResolveImportPackageDir(req.PackageDir)
	if err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageStageFiles, "PACKAGE_DIR_INVALID", err.Error())
		return nil, NewInstallationError(ErrCodePackagePathTraversal, err.Error(), ErrPackagePathTraversal)
	}

	if _, err := os.Stat(safePackageDir); err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageStageFiles, "PACKAGE_DIR_NOT_FOUND", err.Error())
		return nil, err
	}

	if err := copyDirContents(safePackageDir, stagingDir); err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageStageFiles, "COPY_PACKAGE_FAILED", err.Error())
		return nil, err
	}

	oldManifest := filepath.Join(stagingDir, "manifest.json")
	os.Remove(oldManifest)

	if len(req.IncludedActions) > 0 {
		actionsDir := filepath.Join(stagingDir, "actions")
		if entries, dErr := os.ReadDir(actionsDir); dErr == nil {
			for _, entry := range entries {
				if entry.IsDir() && !containsString(req.IncludedActions, entry.Name()) {
					removeTree(filepath.Join(actionsDir, entry.Name()))
				}
			}
		}
	}

	manifestActions, err := scanImportedActions(stagingDir, req.IncludedActions)
	if err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageStageFiles, "SCAN_ACTIONS_FAILED", err.Error())
		return nil, err
	}

	if len(manifestActions) == 0 {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageStageFiles, "NO_ACTIONS_FOUND", "no actions found in package")
		return nil, fmt.Errorf("no actions found in package")
	}

	previewName := findPreviewFile(stagingDir)

	canvasWidth := req.CanvasWidth
	canvasHeight := req.CanvasHeight
	if canvasWidth <= 0 {
		canvasWidth = 256
	}
	if canvasHeight <= 0 {
		canvasHeight = 256
	}

	defaultAction := req.DefaultAction
	if defaultAction == "" && len(manifestActions) > 0 {
		defaultAction = manifestActions[0].Key
	}

	manifest := packageformat.NewManifest()
	manifest.PetID = petID
	manifest.ReleaseID = releaseID
	manifest.Version = version
	manifest.Name = req.PackageName
	if manifest.Name == "" {
		manifest.Name = identity.Name
	}
	manifest.Author = packageformat.ManifestAuthor{
		Name: "legacy_inferred",
		ID:   "legacy",
	}
	manifest.License = packageformat.ManifestLicense{
		SPDX: "legacy_inferred",
	}
	manifest.Compatibility = packageformat.ManifestCompatibility{
		MinRuntimeVersion: "0.0.0",
		RenderMode:         packageformat.RenderModeSprite,
	}
	manifest.Binding = packageformat.ManifestBinding{
		Policy:            packageformat.BindingPolicyInferred,
		SourceCharacterID: req.CharacterID,
	}
	manifest.Canvas = packageformat.ManifestCanvas{
		Width:            canvasWidth,
		Height:           canvasHeight,
		CoordinateSystem: packageformat.CoordinateSystemTopLeft,
	}
	manifest.DefaultAction = defaultAction
	manifest.Preview = previewName
	manifest.Actions = manifestActions
	manifest.Capabilities = packageformat.ManifestCapabilities{
		TransparentBackground: true,
		FrameSequence:         true,
	}
	manifest.Provenance = packageformat.ManifestProvenance{
		SourceType: SourceTypeMigrated,
		BuiltAt:    now,
		Builder:    "u-ai-release-importer",
	}

	manifest, err = im.archiveWriter.BuildManifestForArchive(stagingDir, manifest)
	if err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageStageFiles, "BUILD_INTEGRITY_FAILED", err.Error())
		return nil, err
	}

	op.Stage = OpStageVerify
	_ = im.repo.UpdatePackageOperation(op)

	validation := im.validator.ValidateDirectory(stagingDir, manifest)

	report := &PackageValidationReport{
		ID:           uuid.NewString(),
		ReleaseID:    releaseID,
		OperationID:  opID,
		Source:       "import",
		Verdict:      validation.Verdict,
		FileCount:    validation.FileCount,
		ErrorCount:   validation.ErrorCount,
		WarningCount: validation.WarningCount,
		FindingsJSON: serializeFindings(validation.Findings),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = im.repo.CreateValidationReport(report)

	if validation.Verdict == "invalid" {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageVerify, "VALIDATION_FAILED",
			fmt.Sprintf("package validation failed: %d errors", validation.ErrorCount))
		return nil, fmt.Errorf("package validation failed: %d errors", validation.ErrorCount)
	}

	manifestData, err := im.writer.WriteManifest(manifest)
	if err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageVerify, "WRITE_MANIFEST_FAILED", err.Error())
		return nil, err
	}
	manifestHash := hashBytes(manifestData)
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageVerify, "WRITE_MANIFEST_FAILED", err.Error())
		return nil, err
	}
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStageVerify, "WRITE_MANIFEST_FAILED", err.Error())
		return nil, err
	}

	op.Stage = OpStagePublishFiles
	_ = im.repo.UpdatePackageOperation(op)

	if err := im.storage.MoveStagingToPublished(petID, releaseID); err != nil {
		im.storage.RemoveStagingDir(releaseID)
		failPackageOperation(im.repo, op, OpStagePublishFiles, "MOVE_FAILED", err.Error())
		return nil, err
	}

	publishedDir := im.storage.PublishedDir(petID, releaseID)
	archivePath := im.storage.ArchivePath(petID, releaseID)
	if err := im.archiveWriter.WriteArchive(publishedDir, archivePath); err != nil {
		im.storage.RemovePublishedDir(petID, releaseID)
		failPackageOperation(im.repo, op, OpStagePublishFiles, "ARCHIVE_FAILED", err.Error())
		return nil, err
	}

	op.PublishedPathKey = im.storage.PublishedStorageKey(petID, releaseID)
	_ = im.repo.UpdatePackageOperation(op)

	release := &PackageRelease{
		ID:                releaseID,
		PetID:             petID,
		OwnerUserID:       req.UserID,
		Version:           version,
		ReleaseSequence:   releaseSeq,
		SchemaVersion:     packageformat.ManifestSchemaVersion,
		Status:            ReleaseStatusPublished,
		ContentRootHash:   manifest.Integrity.ContentRootHash,
		ManifestHash:      manifestHash,
		StorageKey:        im.storage.PublishedStorageKey(petID, releaseID),
		ArchiveStorageKey: im.storage.ArchiveStorageKey(petID, releaseID),
		TotalBytes:        manifest.Integrity.TotalBytes,
		FileCount:         manifest.Integrity.FileCount,
		ActionCount:       len(manifestActions),
		DefaultActionKey:  manifest.DefaultAction,
		MinRuntimeVersion: manifest.Compatibility.MinRuntimeVersion,
		SourceType:        SourceTypeMigrated,
		ManifestJSON:      string(manifestData),
		PublishedAt:       now,
		LegacyPackageID:   req.LegacyPackageID,
		LegacyVersion:     req.LegacyVersion,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	files := make([]ReleaseFile, 0, len(manifest.Integrity.Files))
	for _, f := range manifest.Integrity.Files {
		files = append(files, ReleaseFile{
			ID:        uuid.NewString(),
			ReleaseID: releaseID,
			Path:      f.Path,
			SHA256:    f.SHA256,
			Bytes:     f.Bytes,
			MediaType: f.MediaType,
			Role:      f.Role,
			ActionKey: f.ActionKey,
			FrameID:   f.FrameID,
			CreatedAt: now,
		})
	}

	op.Stage = OpStageCommitDB
	_ = im.repo.UpdatePackageOperation(op)

	err = im.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(release).Error; err != nil {
			return err
		}
		if len(files) > 0 {
			if err := tx.CreateInBatches(files, 100).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		im.storage.RemovePublishedDir(petID, releaseID)
		op.Stage = OpStageCommitDB
		op.Status = OpStatusRecovery
		op.ErrorCode = "COMMIT_DB_FAILED"
		op.ErrorMessage = err.Error()
		op.UpdatedAt = time.Now().Format(installationTimeFormat)
		_ = im.repo.UpdatePackageOperation(op)
		return nil, err
	}

	op.Stage = OpStageCompleted
	op.Status = OpStatusCompleted
	op.CompletedAt = time.Now().Format(installationTimeFormat)
	_ = im.repo.UpdatePackageOperation(op)

	return &ImportPackageResult{
		Release:  release,
		Identity: identity,
	}, nil
}

func (im *ReleaseImporter) resolvePetIdentity(req *ImportPackageRequest) (*PetIdentity, error) {
	identity, err := im.repo.GetPetIdentityByCharacter(req.UserID, req.CharacterID)
	if err == nil {
		return identity, nil
	}
	if !errors.Is(err, ErrInstallationNotFound) {
		return nil, err
	}

	now := time.Now().Format(installationTimeFormat)
	name := req.PackageName
	if name == "" {
		name = req.CharacterID
	}
	identity = &PetIdentity{
		ID:                uuid.NewString(),
		OwnerUserID:       req.UserID,
		SourceCharacterID: req.CharacterID,
		Name:              name,
		Slug:              makeSlug(name),
		BindingPolicy:     BindingPolicyCharacterLocked,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := im.repo.CreatePetIdentity(identity); err != nil {
		return nil, err
	}
	return identity, nil
}

func (im *ReleaseImporter) loadExistingImportResult(op *PackageOperation) (*ImportPackageResult, error) {
	release, err := im.repo.GetRelease(op.ReleaseID)
	if err != nil {
		return nil, err
	}
	identity, err := im.repo.GetPetIdentity(release.PetID)
	if err != nil {
		return nil, err
	}
	return &ImportPackageResult{
		Release:  release,
		Identity: identity,
	}, nil
}

func scanImportedActions(stagingDir string, includedActions []string) ([]packageformat.ManifestActionEntry, error) {
	actionsDir := filepath.Join(stagingDir, "actions")
	entries, err := os.ReadDir(actionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		key := entry.Name()
		if len(includedActions) > 0 && !containsString(includedActions, key) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	actions := make([]packageformat.ManifestActionEntry, 0, len(keys))
	for _, key := range keys {
		entry := packageformat.ManifestActionEntry{
			Key:            key,
			Name:           key,
			Config:         fmt.Sprintf("actions/%s/action.json", key),
			RevisionID:     "legacy_inferred",
			QualityVerdict: packageformat.QualityVerdictSkipped,
			LoopType:       packageformat.LoopTypeLoop,
		}

		actionJSONPath := filepath.Join(stagingDir, "actions", key, "action.json")
		if data, rErr := os.ReadFile(actionJSONPath); rErr == nil {
			var cfg actionConfig
			if jErr := json.Unmarshal(data, &cfg); jErr == nil {
				if cfg.ActionName != "" {
					entry.Name = cfg.ActionName
				}
				if cfg.LoopType != "" {
					entry.LoopType = cfg.LoopType
				}
				entry.FPS = cfg.DefaultFps
				entry.FrameCount = cfg.FrameCount
			}
		}

		if entry.FrameCount == 0 {
			framesDir := filepath.Join(stagingDir, "actions", key, "frames")
			if frameEntries, fErr := os.ReadDir(framesDir); fErr == nil {
				count := 0
				for _, fe := range frameEntries {
					if !fe.IsDir() {
						count++
					}
				}
				entry.FrameCount = count
			}
		}

		actions = append(actions, entry)
	}

	return actions, nil
}

func findPreviewFile(stagingDir string) string {
	for _, name := range []string{"preview.png", "preview.jpg", "preview.webp"} {
		if _, err := os.Stat(filepath.Join(stagingDir, name)); err == nil {
			return name
		}
	}
	return ""
}
