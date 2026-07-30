package installation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"gorm.io/gorm"
)

type actionConfig struct {
	ActionKey  string       `json:"actionKey"`
	ActionName string       `json:"actionName"`
	FrameCount int          `json:"frameCount"`
	DefaultFps int          `json:"defaultFps"`
	LoopType   string       `json:"loopType"`
	Frames     []frameEntry `json:"frames"`
}

type frameEntry struct {
	FrameID     string `json:"frameId"`
	Index       int    `json:"index"`
	DurationMs  int    `json:"durationMs"`
	AssetID     string `json:"assetId"`
	ContentHash string `json:"contentHash"`
}

type ReleaseBuilder struct {
	repo          Repository
	storage       *ReleaseStorage
	source        RevisionSource
	writer        *packageformat.V2Writer
	validator     *packageformat.Validator
	archiveWriter *packageformat.ArchiveWriter
}

func NewReleaseBuilder(repo Repository, storage *ReleaseStorage, source RevisionSource) *ReleaseBuilder {
	return &ReleaseBuilder{
		repo:          repo,
		storage:       storage,
		source:        source,
		writer:        &packageformat.V2Writer{},
		validator:     packageformat.NewValidator(),
		archiveWriter: &packageformat.ArchiveWriter{},
	}
}

func (b *ReleaseBuilder) BuildRelease(req *BuildReleaseRequest) (*BuildReleaseResult, error) {
	if req.IdempotencyKey != "" {
		existing, err := b.repo.GetPackageOperationByIdempotencyKey(req.UserID, req.IdempotencyKey, PackageOpTypeBuild)
		if err == nil && existing.Status == OpStatusCompleted {
			return b.loadExistingResult(existing)
		}
	}

	now := time.Now().Format(installationTimeFormat)
	opID := uuid.NewString()
	op := &PackageOperation{
		ID:             opID,
		OperationType:  PackageOpTypeBuild,
		UserID:         req.UserID,
		PetID:          req.PetID,
		IdempotencyKey: req.IdempotencyKey,
		Stage:          OpStagePrepare,
		Status:         OpStatusRunning,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := b.repo.CreatePackageOperation(op); err != nil {
		return nil, err
	}

	taskInfo, err := b.source.GetProcessingTaskInfo(req.ProcessingTaskID)
	if err != nil {
		failPackageOperation(b.repo, op, OpStagePrepare, "PROCESSING_TASK_NOT_FOUND", err.Error())
		return nil, err
	}

	petID, identity, err := b.resolvePetIdentity(req, taskInfo)
	if err != nil {
		failPackageOperation(b.repo, op, OpStagePrepare, "PET_IDENTITY_FAILED", err.Error())
		return nil, err
	}

	op.PetID = petID
	_ = b.repo.UpdatePackageOperation(op)

	releaseSeq, err := b.repo.GetLatestReleaseSequence(petID)
	if err != nil {
		failPackageOperation(b.repo, op, OpStagePrepare, "SEQUENCE_FAILED", err.Error())
		return nil, err
	}
	releaseSeq++

	releaseID := uuid.NewString()
	version := req.Version
	if version == "" {
		version = fmt.Sprintf("1.0.%d", releaseSeq)
	}

	op.Stage = OpStageStageFiles
	op.ReleaseID = releaseID
	op.StagingPathKey = releaseID
	_ = b.repo.UpdatePackageOperation(op)

	stagingDir := b.storage.StagingDir(releaseID)
	if err := b.storage.EnsureStagingDir(releaseID); err != nil {
		failPackageOperation(b.repo, op, OpStageStageFiles, "STAGING_DIR_FAILED", err.Error())
		return nil, err
	}

	actionInfos, err := b.source.ListProcessingActions(req.ProcessingTaskID)
	if err != nil {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStageStageFiles, "LIST_ACTIONS_FAILED", err.Error())
		return nil, err
	}

	actionMap := make(map[string]BuilderActionInfo)
	for _, a := range actionInfos {
		if !a.Excluded {
			actionMap[a.ActionKey] = a
		}
	}

	for _, a := range actionInfos {
		if a.Excluded || a.Status != "succeeded" {
			continue
		}
		detail, dErr := b.source.GetActiveRevisionDetail(req.ProcessingTaskID, a.ActionKey)
		if dErr != nil {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, "QUALITY_GATE_CHECK_FAILED", dErr.Error())
			return nil, dErr
		}
		verdict := detail.QualityVerdict
		if verdict == "rejected" || verdict == "evaluation_failed" {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, ErrCodePackageQualityGateBlocked,
				fmt.Sprintf("动作 %s 质量评估结果为 %s，无法构建 Release", a.ActionKey, verdict))
			return nil, NewInstallationError(ErrCodePackageQualityGateBlocked,
				fmt.Sprintf("动作 %s 质量评估结果为 %s，无法构建 Release", a.ActionKey, verdict), ErrPackageQualityGateBlocked)
		}
		if verdict == "" || verdict == "pending" {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, ErrCodePackageQualityGateBlocked,
				fmt.Sprintf("动作 %s 质量评估尚未完成，无法构建 Release", a.ActionKey))
			return nil, NewInstallationError(ErrCodePackageQualityGateBlocked,
				fmt.Sprintf("动作 %s 质量评估尚未完成，无法构建 Release", a.ActionKey), ErrPackageQualityGateBlocked)
		}
	}

	actionsToBuild := req.IncludedActions
	if len(actionsToBuild) == 0 {
		for key := range actionMap {
			actionsToBuild = append(actionsToBuild, key)
		}
		sort.Strings(actionsToBuild)
	}

	manifestActions := make([]packageformat.ManifestActionEntry, 0, len(actionsToBuild))
	for _, actionKey := range actionsToBuild {
		detail, dErr := b.source.GetActiveRevisionDetail(req.ProcessingTaskID, actionKey)
		if dErr != nil {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, "REVISION_DETAIL_FAILED", dErr.Error())
			return nil, dErr
		}

		info := actionMap[actionKey]
		frames, stageErr := b.stageActionFrames(stagingDir, actionKey, detail)
		if stageErr != nil {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, "STAGE_FRAMES_FAILED", stageErr.Error())
			return nil, stageErr
		}

		cfg := actionConfig{
			ActionKey:  actionKey,
			ActionName: info.ActionNameSnapshot,
			FrameCount: detail.FrameCount,
			DefaultFps: detail.DefaultFPS,
			LoopType:   detail.LoopType,
			Frames:     frames,
		}
		if cfg.ActionName == "" {
			cfg.ActionName = actionKey
		}
		if cfg.LoopType == "" {
			cfg.LoopType = packageformat.LoopTypeLoop
		}

		configPath := filepath.Join(stagingDir, "actions", actionKey, "action.json")
		if err := writeJSONFile(configPath, cfg); err != nil {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, "WRITE_ACTION_CONFIG_FAILED", err.Error())
			return nil, err
		}

		manifestActions = append(manifestActions, packageformat.ManifestActionEntry{
			Key:                 actionKey,
			Name:                cfg.ActionName,
			Config:              fmt.Sprintf("actions/%s/action.json", actionKey),
			RevisionID:          detail.RevisionID,
			QualityVerdict:      detail.QualityVerdict,
			LoopType:            detail.LoopType,
			FPS:                 detail.DefaultFPS,
			FrameCount:          detail.FrameCount,
			SupportsDefaultIdle: info.SupportsDefaultIdle,
		})
	}

	previewName := ""
	previewSrc, pErr := b.source.GetPackagePreviewPath(req.ProcessingTaskID, taskInfo.ProcessingVersion)
	if pErr == nil && previewSrc != "" {
		previewDst := filepath.Join(stagingDir, "preview.png")
		if err := copyFileContents(previewSrc, previewDst); err != nil {
			b.storage.RemoveStagingDir(releaseID)
			failPackageOperation(b.repo, op, OpStageStageFiles, "COPY_PREVIEW_FAILED", err.Error())
			return nil, err
		}
		previewName = "preview.png"
	}

	manifest := packageformat.NewManifest()
	manifest.PetID = petID
	manifest.ReleaseID = releaseID
	manifest.Version = version
	manifest.Name = taskInfo.PackageName
	if manifest.Name == "" {
		manifest.Name = identity.Name
	}
	manifest.Canvas = packageformat.ManifestCanvas{
		Width:            taskInfo.OutputWidth,
		Height:           taskInfo.OutputHeight,
		CoordinateSystem: packageformat.CoordinateSystemTopLeft,
	}
	manifest.Binding = packageformat.ManifestBinding{
		Policy:            packageformat.BindingPolicyBound,
		SourceCharacterID: taskInfo.CharacterID,
	}
	manifest.DefaultAction = req.DefaultAction
	if manifest.DefaultAction == "" && len(actionsToBuild) > 0 {
		manifest.DefaultAction = actionsToBuild[0]
	}
	manifest.Preview = previewName
	manifest.Actions = manifestActions
	manifest.Capabilities = packageformat.ManifestCapabilities{
		TransparentBackground: true,
		FrameSequence:         true,
		PerFrameDuration:      true,
	}
	manifest.Compatibility = packageformat.ManifestCompatibility{
		MinRuntimeVersion: "0.0.0",
		RenderMode:        packageformat.RenderModeSprite,
	}
	manifest.Provenance = packageformat.ManifestProvenance{
		SourceType:       SourceTypeGenerated,
		GenerationTaskID: taskInfo.GenerationTaskID,
		ProcessingTaskID: req.ProcessingTaskID,
		BuiltAt:          now,
		Builder:          "u-ai-release-builder",
	}

	manifest, err = b.archiveWriter.BuildManifestForArchive(stagingDir, manifest)
	if err != nil {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStageStageFiles, "BUILD_INTEGRITY_FAILED", err.Error())
		return nil, err
	}

	op.Stage = OpStageVerify
	_ = b.repo.UpdatePackageOperation(op)

	validation := b.validator.ValidateDirectory(stagingDir, manifest)

	report := &PackageValidationReport{
		ID:           uuid.NewString(),
		ReleaseID:    releaseID,
		OperationID:  opID,
		Source:       "build",
		Verdict:      validation.Verdict,
		FileCount:    validation.FileCount,
		ErrorCount:   validation.ErrorCount,
		WarningCount: validation.WarningCount,
		FindingsJSON: serializeFindings(validation.Findings),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = b.repo.CreateValidationReport(report)

	if validation.Verdict == "invalid" {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStageVerify, "VALIDATION_FAILED",
			fmt.Sprintf("package validation failed: %d errors", validation.ErrorCount))
		return nil, fmt.Errorf("package validation failed: %d errors", validation.ErrorCount)
	}

	manifestData, err := b.writer.WriteManifest(manifest)
	if err != nil {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStageVerify, "WRITE_MANIFEST_FAILED", err.Error())
		return nil, err
	}
	manifestHash := hashBytes(manifestData)
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStageVerify, "WRITE_MANIFEST_FAILED", err.Error())
		return nil, err
	}
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStageVerify, "WRITE_MANIFEST_FAILED", err.Error())
		return nil, err
	}

	op.Stage = OpStagePublishFiles
	_ = b.repo.UpdatePackageOperation(op)

	if err := b.storage.MoveStagingToPublished(petID, releaseID); err != nil {
		b.storage.RemoveStagingDir(releaseID)
		failPackageOperation(b.repo, op, OpStagePublishFiles, "MOVE_FAILED", err.Error())
		return nil, err
	}

	publishedDir := b.storage.PublishedDir(petID, releaseID)
	archivePath := b.storage.ArchivePath(petID, releaseID)
	if err := b.archiveWriter.WriteArchive(publishedDir, archivePath); err != nil {
		b.storage.RemovePublishedDir(petID, releaseID)
		failPackageOperation(b.repo, op, OpStagePublishFiles, "ARCHIVE_FAILED", err.Error())
		return nil, err
	}

	op.PublishedPathKey = b.storage.PublishedStorageKey(petID, releaseID)
	_ = b.repo.UpdatePackageOperation(op)

	release := &PackageRelease{
		ID:                   releaseID,
		PetID:                petID,
		OwnerUserID:          req.UserID,
		Version:              version,
		ReleaseSequence:      releaseSeq,
		SchemaVersion:        packageformat.ManifestSchemaVersion,
		Status:               ReleaseStatusPublished,
		ContentRootHash:      manifest.Integrity.ContentRootHash,
		ManifestHash:         manifestHash,
		StorageKey:           b.storage.PublishedStorageKey(petID, releaseID),
		ArchiveStorageKey:    b.storage.ArchiveStorageKey(petID, releaseID),
		TotalBytes:           manifest.Integrity.TotalBytes,
		FileCount:            manifest.Integrity.FileCount,
		ActionCount:          len(manifestActions),
		DefaultActionKey:     manifest.DefaultAction,
		MinRuntimeVersion:    manifest.Compatibility.MinRuntimeVersion,
		SourceType:           SourceTypeGenerated,
		SourceProcessingTask: req.ProcessingTaskID,
		SourceGenerationTask: taskInfo.GenerationTaskID,
		ManifestJSON:         string(manifestData),
		PublishedAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
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
	_ = b.repo.UpdatePackageOperation(op)

	err = b.repo.Transaction(func(tx *gorm.DB) error {
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
		b.storage.RemovePublishedDir(petID, releaseID)
		op.Stage = OpStageCommitDB
		op.Status = OpStatusRecovery
		op.ErrorCode = "COMMIT_DB_FAILED"
		op.ErrorMessage = err.Error()
		op.UpdatedAt = time.Now().Format(installationTimeFormat)
		_ = b.repo.UpdatePackageOperation(op)
		return nil, err
	}

	op.Stage = OpStageCompleted
	op.Status = OpStatusCompleted
	op.CompletedAt = time.Now().Format(installationTimeFormat)
	_ = b.repo.UpdatePackageOperation(op)

	return &BuildReleaseResult{
		Release:    release,
		Manifest:   manifest,
		Validation: validation,
	}, nil
}

func (b *ReleaseBuilder) resolvePetIdentity(req *BuildReleaseRequest, taskInfo *ProcessingTaskInfo) (string, *PetIdentity, error) {
	if req.PetID != "" {
		identity, err := b.repo.GetPetIdentity(req.PetID)
		if err != nil {
			return "", nil, err
		}
		return identity.ID, identity, nil
	}

	identity, err := b.repo.GetPetIdentityByCharacter(req.UserID, taskInfo.CharacterID)
	if err == nil {
		return identity.ID, identity, nil
	}
	if !errors.Is(err, ErrInstallationNotFound) {
		return "", nil, err
	}

	now := time.Now().Format(installationTimeFormat)
	name := taskInfo.PackageName
	if name == "" {
		name = taskInfo.CharacterID
	}
	identity = &PetIdentity{
		ID:                uuid.NewString(),
		OwnerUserID:       req.UserID,
		SourceCharacterID: taskInfo.CharacterID,
		Name:              name,
		Slug:              makeSlug(name),
		BindingPolicy:     BindingPolicyCharacterLocked,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := b.repo.CreatePetIdentity(identity); err != nil {
		return "", nil, err
	}
	return identity.ID, identity, nil
}

func (b *ReleaseBuilder) stageActionFrames(stagingDir, actionKey string, detail *ActiveRevisionDetail) ([]frameEntry, error) {
	framesDir := filepath.Join(stagingDir, "actions", actionKey, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return nil, err
	}

	frames := make([]frameEntry, 0, len(detail.Frames))
	for _, f := range detail.Frames {
		src, err := b.source.GetAssetPath(f.AssetID)
		if err != nil {
			return nil, err
		}
		ext := extForMimeType(f.MimeType)
		dst := filepath.Join(framesDir, f.FrameID+ext)
		if err := copyFileContents(src, dst); err != nil {
			return nil, err
		}
		frames = append(frames, frameEntry{
			FrameID:     f.FrameID,
			Index:       f.LogicalIndex,
			DurationMs:  f.DurationMS,
			AssetID:     f.AssetID,
			ContentHash: f.ContentHash,
		})
	}
	return frames, nil
}

func (b *ReleaseBuilder) loadExistingResult(op *PackageOperation) (*BuildReleaseResult, error) {
	release, err := b.repo.GetRelease(op.ReleaseID)
	if err != nil {
		return nil, err
	}
	var manifest packageformat.Manifest
	if release.ManifestJSON != "" && release.ManifestJSON != "{}" {
		_ = json.Unmarshal([]byte(release.ManifestJSON), &manifest)
	}
	dbReport, _ := b.repo.GetValidationReport(release.ID)
	var validation *packageformat.ValidationReport
	if dbReport != nil {
		validation = convertValidationReport(dbReport)
	}
	return &BuildReleaseResult{
		Release:    release,
		Manifest:   &manifest,
		Validation: validation,
	}, nil
}

func failPackageOperation(repo Repository, op *PackageOperation, stage, code, msg string) {
	op.Stage = stage
	op.Status = OpStatusFailed
	op.ErrorCode = code
	op.ErrorMessage = msg
	op.UpdatedAt = time.Now().Format(installationTimeFormat)
	_ = repo.UpdatePackageOperation(op)
}

func extForMimeType(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/jpeg":
		return ".jpg"
	default:
		return ".png"
	}
}

func makeSlug(name string) string {
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

func writeJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func serializeFindings(findings []packageformat.Finding) string {
	if findings == nil {
		return "[]"
	}
	data, err := json.Marshal(findings)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func convertValidationReport(report *PackageValidationReport) *packageformat.ValidationReport {
	if report == nil {
		return nil
	}
	var findings []packageformat.Finding
	if report.FindingsJSON != "" && report.FindingsJSON != "[]" {
		_ = json.Unmarshal([]byte(report.FindingsJSON), &findings)
	}
	return &packageformat.ValidationReport{
		Verdict:      report.Verdict,
		Findings:     findings,
		FileCount:    report.FileCount,
		ErrorCount:   report.ErrorCount,
		WarningCount: report.WarningCount,
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
