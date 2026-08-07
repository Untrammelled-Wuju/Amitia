package importer

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
)

type PackageImporter struct {
	repo           release.ReleaseRepository
	storage        release.ReleaseStoragePort
	validator      PackageValidator
	registry       *security.PathRootRegistry
	stagingRepo    security.ImportStagingRepository
	journalManager JournalManagerPort
}

type JournalManagerPort interface {
	CreateImportJournal(operationID, releaseID, petID string) (*release.ReleasePublishJournal, error)
	UpdateStage(journal *release.ReleasePublishJournal, stage, contentRootHash, stagingPath, publishedPath string) error
	MarkFailed(journal *release.ReleasePublishJournal, errMsg string) error
}

type PackageValidator interface {
	ValidatePackage(ctx context.Context, userID string, stagingID string) (*ImportValidationResult, error)
}

type ImportValidationResult struct {
	IsValid              bool
	SourcePackageHash    string
	SourceManifestHash   string
	SourceSchemaVersion  int
	Warnings             []string
	BindingDecision      string
	LicenseDecision      string
	RuntimeCompatibility string
	SelectedActions      []string
	Manifest             *packageformat.Manifest
	ValidationReport     *packageformat.ValidationReport
}

type ImportPackageRequest struct {
	UserID          string
	ImportStagingID string
	SourceFilePath  string
	IdempotencyKey  string
}

type ImportPackageResult struct {
	ImportSnapshot *release.ImportPackageSnapshot
	PetID          string
	ReleaseID      string
	OperationID    string
}

func NewPackageImporter(
	repo release.ReleaseRepository,
	storage release.ReleaseStoragePort,
	validator PackageValidator,
) *PackageImporter {
	return &PackageImporter{
		repo:      repo,
		storage:   storage,
		validator: validator,
	}
}

func NewPackageImporterWithStaging(
	repo release.ReleaseRepository,
	storage release.ReleaseStoragePort,
	validator PackageValidator,
	registry *security.PathRootRegistry,
	stagingRepo security.ImportStagingRepository,
) *PackageImporter {
	return &PackageImporter{
		repo:        repo,
		storage:     storage,
		validator:   validator,
		registry:    registry,
		stagingRepo: stagingRepo,
	}
}

func NewPackageImporterWithJournal(
	repo release.ReleaseRepository,
	storage release.ReleaseStoragePort,
	validator PackageValidator,
	registry *security.PathRootRegistry,
	stagingRepo security.ImportStagingRepository,
	journalManager JournalManagerPort,
) *PackageImporter {
	return &PackageImporter{
		repo:           repo,
		storage:        storage,
		validator:      validator,
		registry:       registry,
		stagingRepo:    stagingRepo,
		journalManager: journalManager,
	}
}

func (pi *PackageImporter) ImportPackage(ctx context.Context, req *ImportPackageRequest) (*ImportPackageResult, error) {
	if req.UserID == "" {
		return nil, release.NewReleaseError("INVALID_USER", "用户 ID 不能为空", nil)
	}
	if req.ImportStagingID == "" {
		return nil, release.NewReleaseError("INVALID_STAGING", "导入暂存 ID 不能为空", nil)
	}

	existing, err := pi.repo.GetImportSnapshot(req.ImportStagingID)
	if err == nil && existing != nil && existing.ReleaseID != "" {
		if existing.OperationID == "" {
			return nil, release.NewReleaseError("IMPORT_SNAPSHOT_INCOMPLETE", "导入快照缺少 OperationID", nil)
		}
		return &ImportPackageResult{
			ImportSnapshot: existing,
			PetID:          existing.PetID,
			ReleaseID:      existing.ReleaseID,
			OperationID:    existing.OperationID,
		}, nil
	}

	validation, err := pi.validator.ValidatePackage(ctx, req.UserID, req.ImportStagingID)
	if err != nil {
		return nil, release.NewReleaseError("VALIDATION_FAILED", "包验证失败", err)
	}
	if !validation.IsValid {
		return nil, release.NewReleaseError("PACKAGE_INVALID", fmt.Sprintf("包验证不通过: %v", validation.Warnings), nil)
	}

	result, err := pi.executeImport(ctx, req, validation)
	if err != nil {
		return nil, err
	}

	if result.ImportSnapshot == nil {
		return nil, release.NewReleaseError("SNAPSHOT_MISSING", "导入快照未在事务中创建", nil)
	}

	return result, nil
}

func (pi *PackageImporter) executeImport(ctx context.Context, req *ImportPackageRequest, validation *ImportValidationResult) (*ImportPackageResult, error) {
	manifest := validation.Manifest
	if manifest == nil {
		return nil, release.NewReleaseError("MANIFEST_MISSING", "验证结果缺少 manifest", nil)
	}

	if pi.registry == nil || pi.stagingRepo == nil {
		return nil, release.NewReleaseError("DEPENDENCY_MISSING", "导入依赖未初始化", nil)
	}

	staging, err := pi.stagingRepo.GetForUser(ctx, req.ImportStagingID, req.UserID)
	if err != nil {
		return nil, release.NewReleaseError("STAGING_READ_FAILED", "读取暂存记录失败", err)
	}

	sourcePath, err := pi.registry.Resolve(security.RootImportQuarantine, staging.StorageKey)
	if err != nil {
		return nil, release.NewReleaseError("PATH_RESOLVE_FAILED", "解析暂存路径失败", err)
	}

	actualHash, actualBytes, err := hashRegularFile(sourcePath)
	if err != nil {
		return nil, release.NewReleaseError("SOURCE_HASH_FAILED", "计算源文件哈希失败", err)
	}

	if actualHash != staging.SourceContentHash || actualBytes != staging.SourceBytes {
		return nil, release.NewReleaseError("STAGING_SOURCE_CHANGED", "暂存包在验证后发生变化", nil)
	}

	if pi.journalManager != nil {
		_, _ = pi.journalManager.CreateImportJournal("", "", "")
		_ = pi.journalManager.UpdateStage(&release.ReleasePublishJournal{}, release.ImportJournalStageValidated, "", "", "")
	}

	characterID := manifest.Binding.SourceCharacterID

	petID := "pet_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	var identity *release.PetIdentityData
	existingByIdentity, err := pi.repo.GetPetIdentityByCharacter(req.UserID, characterID)
	if err == nil && existingByIdentity != nil {
		identity = existingByIdentity
		petID = identity.ID
	}

	now := formatImportTimestamp(time.Now())
	operationID := uuid.NewString()
	releaseID := uuid.NewString()

	if identity == nil {
		name := strings.TrimSpace(manifest.Name)
		if name == "" {
			name = petID
		}
		bindingPolicy := strings.TrimSpace(manifest.Binding.Policy)
		if bindingPolicy == "" {
			bindingPolicy = "character_locked"
		}
		identity = &release.PetIdentityData{
			ID:                  petID,
			OwnerUserID:         req.UserID,
			SourceCharacterID:   characterID,
			Name:                name,
			Slug:                makeIdentitySlug(name),
			BindingPolicy:       bindingPolicy,
			UpstreamPetID:       strings.TrimSpace(manifest.PetID),
			DefaultActionKey:    manifest.DefaultAction,
			NextReleaseSequence: 1,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	}

	sequence := identity.NextReleaseSequence
	version := manifest.Version
	if version == "" {
		version = fmt.Sprintf("1.0.%d", sequence)
	}

	storageKey, err := pi.storage.PublishedStorageKey(petID, releaseID)
	if err != nil {
		return nil, release.NewReleaseError("STORAGE_KEY_FAILED", "计算发布存储键失败", err)
	}

	contentRootHash := strings.TrimSpace(manifest.Integrity.ContentRootHash)
	manifestHash := validation.SourceManifestHash
	totalBytes := manifest.Integrity.TotalBytes
	fileCount := manifest.Integrity.FileCount
	actionCount := len(manifest.Actions)
	minRuntimeVersion := strings.TrimSpace(manifest.Compatibility.MinRuntimeVersion)

	if totalBytes == 0 || fileCount == 0 {
		computedTotal, computedCount, err := pi.computeArchiveStats(sourcePath, manifest)
		if err == nil {
			totalBytes = computedTotal
			fileCount = computedCount
		}
	}

	releaseIntegrity := string(release.ReleaseIntegrityUnknown)
	if manifest.Integrity.Algorithm == "sha256" && contentRootHash != "" {
		releaseIntegrity = string(release.ReleaseIntegrityVerified)
	}

	manifestJSONBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, release.NewReleaseError("MARSHAL_MANIFEST_FAILED", "序列化 manifest 失败", err)
	}

	releaseRecord := &release.ReleaseData{
		ID:                   releaseID,
		PetID:                petID,
		OwnerUserID:          req.UserID,
		Version:              version,
		ReleaseSequence:      sequence,
		SchemaVersion:        manifest.SchemaVersion,
		Lifecycle:            string(release.ReleaseLifecycleBuilding),
		ContentRootHash:      contentRootHash,
		ManifestHash:         manifestHash,
		StorageKey:           storageKey,
		TotalBytes:           totalBytes,
		FileCount:            fileCount,
		ActionCount:          actionCount,
		DefaultActionKey:     manifest.DefaultAction,
		MinRuntimeVersion:    minRuntimeVersion,
		SourceType:           "imported",
		IntegrityStatus:      releaseIntegrity,
		CompatibilityStatus:  string(release.ReleaseCompatUnknown),
		ManifestJSON:         string(manifestJSONBytes),
		LegacyPackageID:      firstNonEmpty(staging.ID, staging.SourceFilename, staging.SourceContentHash),
		LegacyVersion:        sequence,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	releaseFiles := buildReleaseFilesFromManifest(releaseID, manifest, now)

	snapshot := &release.ImportPackageSnapshot{
		ID:                    uuid.NewString(),
		ImportStagingID:       req.ImportStagingID,
		SourcePackageHash:     validation.SourcePackageHash,
		SourceManifestHash:    validation.SourceManifestHash,
		SourceSchemaVersion:   validation.SourceSchemaVersion,
		NormalizationWarnings: formatWarnings(validation.Warnings),
		SelectedActionsJSON:   formatSelectedActions(validation.SelectedActions),
		BindingDecision:       validation.BindingDecision,
		LicenseDecision:       validation.LicenseDecision,
		RuntimeCompatibility:  validation.RuntimeCompatibility,
		UserID:                req.UserID,
		PetID:                 petID,
		ReleaseID:             releaseID,
		OperationID:           operationID,
		Status:                release.ImportSnapshotPreparing,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	buildOp := &release.ReleaseBuildOperation{
		ID:             operationID,
		UserID:         req.UserID,
		PetID:          petID,
		IdempotencyKey: req.IdempotencyKey,
		InputHash:      validation.SourcePackageHash,
		State:          release.BuildOpStateSnapshotting,
		Stage:          release.ImportJournalStageCreated,
		ReleaseID:      releaseID,
		StartedAt:      now,
		UpdatedAt:      now,
	}

	var journal *release.ReleasePublishJournal
	if pi.journalManager != nil {
		j, err := pi.journalManager.CreateImportJournal(operationID, releaseID, petID)
		if err != nil {
			return nil, release.NewReleaseError("JOURNAL_CREATE_FAILED", "创建发布日志失败", err)
		}
		journal = j
	}

	if err := pi.runImportSaga(ctx, req, &identity, releaseRecord, buildOp, validation, sourcePath, journal, releaseFiles, snapshot, staging.SourceContentHash); err != nil {
		return nil, err
	}

	return &ImportPackageResult{
		ImportSnapshot: snapshot,
		PetID:          releaseRecord.PetID,
		ReleaseID:      releaseID,
		OperationID:    operationID,
	}, nil
}

func (pi *PackageImporter) runImportSaga(
	ctx context.Context,
	req *ImportPackageRequest,
	identity **release.PetIdentityData,
	releaseRecord *release.ReleaseData,
	buildOp *release.ReleaseBuildOperation,
	validation *ImportValidationResult,
	sourcePath string,
	journal *release.ReleasePublishJournal,
	releaseFiles []release.ReleaseFileData,
	snapshot *release.ImportPackageSnapshot,
	sourceContentHash string,
) error {
	petID := releaseRecord.PetID
	releaseID := releaseRecord.ID
	operationID := buildOp.ID

	txErr := pi.repo.Transaction(func(tx *gorm.DB) error {
		if err := pi.repo.CreatePetIdentityTx(tx, *identity); err != nil {
			if !isDuplicateKeyError(err) {
				return err
			}
			existingIdentity, getErr := pi.repo.GetPetIdentity((*identity).ID)
			if getErr != nil || existingIdentity == nil {
				return err
			}
			if existingIdentity.OwnerUserID != req.UserID {
				return release.NewReleaseError("PET_IDENTITY_OWNER_MISMATCH", "manifest PetID 不属于当前用户", nil)
			}
			*identity = existingIdentity
			releaseRecord.PetID = (*identity).ID
			buildOp.PetID = (*identity).ID
			petID = (*identity).ID
			snapshot.PetID = petID
		}

		nextSeq := (*identity).NextReleaseSequence
		releaseRecord.ReleaseSequence = nextSeq
		if validation.Manifest.Version == "" {
			releaseRecord.Version = fmt.Sprintf("1.0.%d", nextSeq)
		}
		(*identity).NextReleaseSequence = nextSeq + 1
		(*identity).UpdatedAt = formatImportTimestamp(time.Now())
		if (*identity).DefaultActionKey == "" {
			(*identity).DefaultActionKey = validation.Manifest.DefaultAction
		}
		if uErr := pi.repo.UpdatePetIdentityTx(tx, *identity); uErr != nil {
			return uErr
		}

		if err := pi.repo.CreateOperationTx(tx, buildOp); err != nil {
			return err
		}

		if err := pi.repo.CreateReleaseTx(tx, releaseRecord); err != nil {
			return err
		}

		if len(releaseFiles) > 0 {
			if err := pi.repo.CreateReleaseFilesTx(tx, releaseFiles); err != nil {
				return err
			}
		}

		if err := pi.repo.CreateImportSnapshotTx(tx, snapshot); err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		pi.failJournal(journal, txErr)
		return release.NewReleaseError("TRANSACTION_FAILED", "导入事务提交失败", txErr)
	}

	if pi.journalManager != nil && journal != nil {
		_ = pi.journalManager.UpdateStage(journal, release.ImportJournalStageDatabasePrepared, "", sourcePath, "")
	}

	buildOp.State = release.BuildOpStateBuilding
	buildOp.Stage = release.ImportJournalStageDatabasePrepared
	buildOp.UpdatedAt = formatImportTimestamp(time.Now())
	_ = pi.repo.UpdateBuildOperation(buildOp)

	if err := pi.storage.EnsureWorkspaceDir(operationID); err != nil {
		pi.failJournal(journal, err)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("WORKSPACE_CREATE_FAILED", "创建工作区失败", err)
	}

	workspaceDir, err := pi.storage.WorkspaceDir(operationID)
	if err != nil {
		pi.failJournal(journal, err)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("WORKSPACE_DIR_FAILED", "获取工作区路径失败", err)
	}

	reader := packageformat.NewArchiveReader(packageformat.DefaultArchiveLimits())
	if err := reader.ExtractArchive(sourcePath, workspaceDir); err != nil {
		pi.failJournal(journal, err)
		pi.storage.RemoveWorkspaceDir(operationID)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("WORKSPACE_EXTRACT_FAILED", "解压到工作区失败", err)
	}

	workspaceReport := packageformat.NewValidator().ValidateDirectory(workspaceDir, validation.Manifest)
	if workspaceReport == nil || workspaceReport.Verdict == "invalid" {
		pi.failJournal(journal, err)
		pi.storage.RemoveWorkspaceDir(operationID)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("WORKSPACE_VALIDATION_FAILED", "解压后的桌宠包验证失败", nil)
	}

	if pi.journalManager != nil && journal != nil {
		_ = pi.journalManager.UpdateStage(journal, release.ImportJournalStageWorkspaceBuilt, "", workspaceDir, "")
	}

	buildOp.Stage = release.ImportJournalStageWorkspaceBuilt
	buildOp.UpdatedAt = formatImportTimestamp(time.Now())
	_ = pi.repo.UpdateBuildOperation(buildOp)

	if sourceInfo, statErr := os.Stat(sourcePath); statErr == nil && sourceInfo.Mode().IsRegular() {
		expectedHash := ""
		if sourceContentHash != "" {
			expectedHash = sourceContentHash
		}
		archiveStorageKey, archiveHash, archiveBytes, archiveErr := pi.storage.StoreVerifiedArchive(sourcePath, petID, releaseID, expectedHash)
		if archiveErr != nil {
			pi.failJournal(journal, archiveErr)
			pi.storage.RemoveWorkspaceDir(operationID)
			pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
			return release.NewReleaseError("ARCHIVE_STORE_FAILED", "归档存储失败", archiveErr)
		}
		releaseRecord.ArchiveStorageKey = archiveStorageKey
		releaseRecord.ArchiveHash = archiveHash
		releaseRecord.ArchiveBytes = archiveBytes
	} else {
		releaseRecord.ArchiveStorageKey = ""
		releaseRecord.ArchiveHash = ""
		releaseRecord.ArchiveBytes = 0
	}
	if uErr := pi.repo.UpdateRelease(releaseRecord); uErr != nil {
		pi.failJournal(journal, uErr)
		pi.storage.RemoveWorkspaceDir(operationID)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("ARCHIVE_HASH_UPDATE_FAILED", "更新归档记录失败", uErr)
	}

	if err := pi.storage.EnsureStagingDir(releaseID); err != nil {
		pi.failJournal(journal, err)
		pi.storage.RemoveWorkspaceDir(operationID)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("STAGING_ENSURE_FAILED", "创建暂存目录失败", err)
	}

	if err := pi.storage.MoveWorkspaceToStaging(operationID, releaseID); err != nil {
		pi.failJournal(journal, err)
		pi.storage.RemoveWorkspaceDir(operationID)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("WORKSPACE_TO_STAGING_FAILED", "工作区到暂存移动失败", err)
	}

	buildOp.Stage = release.ImportJournalStageFilesPublished
	buildOp.UpdatedAt = formatImportTimestamp(time.Now())
	_ = pi.repo.UpdateBuildOperation(buildOp)

	if err := pi.storage.AtomicRenameStagingToPublished(petID, releaseID); err != nil {
		pi.failJournal(journal, err)
		pi.storage.RemoveStagingDir(releaseID)
		pi.compensateAfterDatabasePrepared(petID, releaseID, buildOp, snapshot, true)
		return release.NewReleaseError("ATOMIC_PUBLISH_FAILED", "原子发布失败", err)
	}

	pi.storage.RemoveWorkspaceDir(operationID)

	if pi.journalManager != nil && journal != nil {
		_ = pi.journalManager.UpdateStage(journal, release.ImportJournalStageFilesPublished, releaseRecord.ContentRootHash, workspaceDir, "")
	}

	if err := pi.finalizeImport(ctx, releaseRecord, buildOp, snapshot, journal); err != nil {
		return err
	}

	return nil
}

func (pi *PackageImporter) finalizeImport(
	ctx context.Context,
	releaseRecord *release.ReleaseData,
	buildOp *release.ReleaseBuildOperation,
	snapshot *release.ImportPackageSnapshot,
	journal *release.ReleasePublishJournal,
) error {
	now := formatImportTimestamp(time.Now())

	txErr := pi.repo.Transaction(func(tx *gorm.DB) error {
		releaseRecord.Lifecycle = string(release.ReleaseLifecycleReady)
		releaseRecord.IntegrityStatus = string(release.ReleaseIntegrityVerified)
		releaseRecord.CompatibilityStatus = string(release.ReleaseCompatCompatible)
		releaseRecord.PublishedAt = now
		releaseRecord.UpdatedAt = now
		if err := pi.repo.UpdateRelease(releaseRecord); err != nil {
			return err
		}

		buildOp.State = release.BuildOpStateCompleted
		buildOp.Stage = release.ImportJournalStageCompleted
		buildOp.CompletedAt = now
		buildOp.UpdatedAt = now
		if err := pi.repo.UpdateBuildOperation(buildOp); err != nil {
			return err
		}

		snapshot.Status = release.ImportSnapshotCompleted
		snapshot.UpdatedAt = now
		if err := pi.repo.UpdateImportSnapshot(snapshot); err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		pi.failJournal(journal, txErr)
		if pi.journalManager != nil && journal != nil {
			_ = pi.journalManager.UpdateStage(journal, release.ImportJournalStageManualReview, releaseRecord.ContentRootHash, "", "")
		}
		return release.NewReleaseError("FINALIZE_TRANSACTION_FAILED", "完成导入事务失败", txErr)
	}

	if pi.journalManager != nil && journal != nil {
		_ = pi.journalManager.UpdateStage(journal, release.ImportJournalStageSnapshotCommitted, releaseRecord.ContentRootHash, "", "")
		_ = pi.journalManager.UpdateStage(journal, release.ImportJournalStageCompleted, releaseRecord.ContentRootHash, "", "")
	}

	return nil
}

func buildReleaseFilesFromManifest(releaseID string, manifest *packageformat.Manifest, now string) []release.ReleaseFileData {
	result := make([]release.ReleaseFileData, 0, len(manifest.Integrity.Files))
	for _, file := range manifest.Integrity.Files {
		result = append(result, release.ReleaseFileData{
			ID:        "release_file_" + uuid.NewString(),
			ReleaseID: releaseID,
			Path:      file.Path,
			SHA256:    file.SHA256,
			Bytes:     file.Bytes,
			MediaType: file.MediaType,
			Role:      file.Role,
			ActionKey: file.ActionKey,
			FrameID:   file.FrameID,
			CreatedAt: now,
		})
	}
	return result
}

func hashRegularFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := sha256.New()
	written, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hash.Sum(nil)), written, nil
}

func (pi *PackageImporter) failJournal(journal *release.ReleasePublishJournal, err error) {
	if pi.journalManager == nil || journal == nil {
		return
	}
	_ = pi.journalManager.MarkFailed(journal, err.Error())
}

func (pi *PackageImporter) compensateAfterDatabasePrepared(petID, releaseID string, buildOp *release.ReleaseBuildOperation, snapshot *release.ImportPackageSnapshot, cleanupFiles bool) {
	if cleanupFiles {
		if petID != "" && releaseID != "" {
			pi.storage.RemovePublishedDir(petID, releaseID)
		}
		if releaseID != "" {
			pi.storage.RemoveStagingDir(releaseID)
		}
		if buildOp != nil && buildOp.ID != "" {
			pi.storage.RemoveWorkspaceDir(buildOp.ID)
		}
	}

	if buildOp != nil {
		buildOp.State = release.BuildOpStateFailedRetryable
		buildOp.ErrorCode = "COMPENSATED"
		buildOp.ErrorMessage = "导入失败已补偿"
		buildOp.UpdatedAt = formatImportTimestamp(time.Now())
		_ = pi.repo.UpdateBuildOperation(buildOp)
	}

	if snapshot != nil {
		snapshot.Status = release.ImportSnapshotFailedRetryable
		snapshot.UpdatedAt = formatImportTimestamp(time.Now())
		_ = pi.repo.UpdateImportSnapshot(snapshot)
	}

	if releaseID != "" {
		if releaseRecord, err := pi.repo.GetRelease(releaseID); err == nil && releaseRecord != nil {
			releaseRecord.Lifecycle = string(release.ReleaseLifecycleBuilding)
			releaseRecord.IntegrityStatus = string(release.ReleaseIntegrityUnknown)
			releaseRecord.UpdatedAt = formatImportTimestamp(time.Now())
			_ = pi.repo.UpdateRelease(releaseRecord)
		}
	}
}

func (pi *PackageImporter) hashWorkspaceFiles(workspaceDir, releaseID string, manifest *packageformat.Manifest) ([]release.ReleaseFileData, int64, int, error) {
	var fileRecords []release.ReleaseFileData
	var totalBytes int64
	var fileCount int
	now := formatImportTimestamp(time.Now())

	err := filepath.Walk(workspaceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, relErr := filepath.Rel(workspaceDir, filePath)
		if relErr != nil {
			return relErr
		}
		clean := path.Clean(strings.ReplaceAll(relPath, `\`, "/"))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
			return fmt.Errorf("workspace path escape: %s", relPath)
		}

		file, openErr := os.Open(filePath)
		if openErr != nil {
			return openErr
		}
		defer file.Close()

		hash := sha256.New()
		written, copyErr := io.Copy(hash, file)
		if copyErr != nil {
			return copyErr
		}

		fileHash := hex.EncodeToString(hash.Sum(nil))
		fileID := uuid.NewString()
		mediaType := detectMediaType(clean)
		role := fileRoleFromPath(clean)
		actionKey := fileActionKeyFromManifest(manifest, clean)
		frameID := fileFrameIDFromManifest(manifest, clean)

		fileRecords = append(fileRecords, release.ReleaseFileData{
			ID:        fileID,
			ReleaseID: releaseID,
			Path:      clean,
			SHA256:    fileHash,
			Bytes:     written,
			MediaType: mediaType,
			Role:      role,
			ActionKey: actionKey,
			FrameID:   frameID,
			CreatedAt: now,
		})

		totalBytes += written
		fileCount++
		return nil
	})

	if err != nil {
		return nil, 0, 0, err
	}
	return fileRecords, totalBytes, fileCount, nil
}

func (pi *PackageImporter) computeArchiveStats(sourcePath string, manifest *packageformat.Manifest) (int64, int, error) {
	archive, err := zip.OpenReader(sourcePath)
	if err != nil {
		return 0, 0, err
	}
	defer archive.Close()

	var total int64
	var count int
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		total += int64(file.UncompressedSize64)
		count++
	}

	_ = manifest
	return total, count, nil
}

func (pi *PackageImporter) markOperationFailed(op *release.ReleaseBuildOperation, code string, err error) {
	op.State = release.BuildOpStateFailedTerminal
	op.ErrorCode = code
	if err != nil {
		op.ErrorMessage = err.Error()
	}
	op.UpdatedAt = formatImportTimestamp(time.Now())
	_ = pi.repo.UpdateBuildOperation(op)
}

func (pi *PackageImporter) GetStagingRepo() security.ImportStagingRepository {
	return pi.stagingRepo
}

func formatImportTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func makeIdentitySlug(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = "pet"
	}
	return result
}

func formatWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return "[]"
	}
	result := "["
	for i, w := range warnings {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%q", w)
	}
	result += "]"
	return result
}

func formatSelectedActions(actions []string) string {
	if len(actions) == 0 {
		return "[]"
	}
	result := "["
	for i, a := range actions {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%q", a)
	}
	result += "]"
	return result
}

func detectMediaType(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".json":
		return "application/json"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func fileRoleFromPath(filePath string) string {
	base := strings.ToLower(path.Base(filePath))
	if base == "manifest.json" {
		return "manifest"
	}
	if base == "integrity.json" {
		return "integrity"
	}
	if strings.HasPrefix(strings.ToLower(filePath), "frames/") || strings.HasPrefix(strings.ToLower(filePath), "frame/") {
		return "frame"
	}
	if strings.HasPrefix(strings.ToLower(filePath), "preview") {
		return "preview"
	}
	if strings.HasPrefix(strings.ToLower(filePath), "assets/") {
		return "asset"
	}
	return "data"
}

func fileActionKeyFromManifest(manifest *packageformat.Manifest, filePath string) string {
	if manifest == nil {
		return ""
	}
	for _, action := range manifest.Actions {
		prefix := path.Clean(strings.ReplaceAll(action.Key, `\`, "/")) + "/"
		clean := path.Clean(strings.ReplaceAll(filePath, `\`, "/"))
		if strings.HasPrefix(clean, prefix) {
			return action.Key
		}
	}
	return ""
}

func fileFrameIDFromManifest(manifest *packageformat.Manifest, filePath string) string {
	if manifest == nil {
		return ""
	}
	clean := path.Clean(strings.ReplaceAll(filePath, `\`, "/"))
	for _, f := range manifest.Integrity.Files {
		if f.Path == clean {
			return f.SHA256
		}
	}
	return ""
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "Duplicate")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}