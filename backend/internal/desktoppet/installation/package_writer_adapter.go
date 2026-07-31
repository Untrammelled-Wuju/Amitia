package installation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	relbuild "github.com/u-ai/backend/internal/desktoppet/release/build"
)

type packageWriterAdapter struct {
	storage       *ReleaseStorage
	source        RevisionSource
	writer        *packageformat.V2Writer
	validator     *packageformat.Validator
	archiveWriter *packageformat.ArchiveWriter
}

func NewPackageWriterAdapter(
	storage *ReleaseStorage,
	source RevisionSource,
	writer *packageformat.V2Writer,
	validator *packageformat.Validator,
	archiveWriter *packageformat.ArchiveWriter,
) relbuild.PackageWriter {
	return &packageWriterAdapter{
		storage:       storage,
		source:        source,
		writer:        writer,
		validator:     validator,
		archiveWriter: archiveWriter,
	}
}

func (a *packageWriterAdapter) StagePackage(
	snapshot *release.ReleaseBuildSnapshot,
	actionSnapshots []release.ReleaseActionSnapshot,
	taskInfo *release.TaskInfo,
	identity *release.PetIdentityData,
	previewArtifactID string,
	defaultActionKey string,
) (*relbuild.StagedPackage, error) {
	releaseID := uuid.NewString()
	stagingDir := a.storage.StagingDir(releaseID)
	if err := a.storage.EnsureStagingDir(releaseID); err != nil {
		return nil, err
	}

	actionInfos, err := a.source.ListProcessingActions(snapshot.ProcessingTaskID)
	if err != nil {
		a.storage.RemoveStagingDir(releaseID)
		return nil, err
	}
	actionMap := make(map[string]BuilderActionInfo)
	for _, ai := range actionInfos {
		actionMap[ai.ActionKey] = ai
	}

	manifestActions := make([]packageformat.ManifestActionEntry, 0, len(actionSnapshots))
	stagedActions := make([]relbuild.StagedAction, 0, len(actionSnapshots))

	for _, actionSnap := range actionSnapshots {
		detail, dErr := a.source.GetActiveRevisionDetail(snapshot.ProcessingTaskID, actionSnap.ActionKey)
		if dErr != nil {
			a.storage.RemoveStagingDir(releaseID)
			return nil, dErr
		}

		info := actionMap[actionSnap.ActionKey]
		frames, stageErr := a.stageActionFrames(stagingDir, actionSnap.ActionKey, detail)
		if stageErr != nil {
			a.storage.RemoveStagingDir(releaseID)
			return nil, stageErr
		}

		displayName := info.ActionNameSnapshot
		if displayName == "" {
			displayName = actionSnap.ActionKey
		}
		playbackMode := packageformat.NormalizePlaybackMode(detail.LoopType)
		if playbackMode == "" {
			playbackMode = packageformat.LoopTypeLoop
		}

		cfg := actionConfig{
			SchemaVersion: ActionConfigSchemaVersion,
			ActionKey:     actionSnap.ActionKey,
			DisplayName:   displayName,
			Fps:           detail.DefaultFPS,
			PlaybackMode:  playbackMode,
			Interruptible: detail.Interruptible,
			Priority:      detail.Priority,
			CooldownMs:    detail.CooldownMs,
			MinimumPlayMs: detail.MinimumPlayMs,
			MaximumPlayMs: detail.MaximumPlayMs,
			MutexGroup:    detail.MutexGroup,
			ReturnTo:      buildReturnToRule(detail.ReturnAction, detail.ReturnPolicy),
			Anchor: anchorConfig{
				X:               DefaultAnchorX,
				Y:               DefaultAnchorY,
				CoordinateSpace: DefaultAnchorCoordinateSpace,
			},
			Frames: frames,
		}

		configPath := filepath.Join(stagingDir, "actions", actionSnap.ActionKey, "action.json")
		if err := writeJSONFile(configPath, cfg); err != nil {
			a.storage.RemoveStagingDir(releaseID)
			return nil, err
		}

		manifestActions = append(manifestActions, packageformat.ManifestActionEntry{
			Key:                 actionSnap.ActionKey,
			Name:                displayName,
			Config:              fmt.Sprintf("actions/%s/action.json", actionSnap.ActionKey),
			RevisionID:          detail.RevisionID,
			QualityVerdict:      detail.QualityVerdict,
			LoopType:            playbackMode,
			FPS:                 detail.DefaultFPS,
			FrameCount:          detail.FrameCount,
			SupportsDefaultIdle: info.SupportsDefaultIdle,
		})

		stagedFrameEntries := make([]relbuild.StagedFrameEntry, 0, len(frames))
		for _, f := range frames {
			stagedFrameEntries = append(stagedFrameEntries, relbuild.StagedFrameEntry{
				FrameID:     f.FrameID,
				Index:       f.Index,
				File:        f.File,
				DurationMs:  f.DurationMs,
				AssetID:     f.AssetID,
				ContentHash: f.ContentHash,
			})
		}
		stagedActions = append(stagedActions, relbuild.StagedAction{
			ActionKey:           actionSnap.ActionKey,
			DisplayName:         displayName,
			RevisionID:          detail.RevisionID,
			QualityVerdict:      detail.QualityVerdict,
			LoopType:            playbackMode,
			FPS:                 detail.DefaultFPS,
			FrameCount:          detail.FrameCount,
			SupportsDefaultIdle: info.SupportsDefaultIdle,
			FrameEntries:        stagedFrameEntries,
		})
	}

	previewName := ""
	if previewArtifactID != "" {
		if _, err := os.Stat(previewArtifactID); err == nil {
			previewDst := filepath.Join(stagingDir, "preview.png")
			if err := copyFileContents(previewArtifactID, previewDst); err != nil {
				a.storage.RemoveStagingDir(releaseID)
				return nil, err
			}
			previewName = "preview.png"
		}
	}

	now := time.Now().Format(installationTimeFormat)
	manifest := packageformat.NewManifest()
	manifest.PetID = identity.ID
	manifest.ReleaseID = releaseID
	manifest.Version = "1.0.0"
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
	manifest.DefaultAction = defaultActionKey
	manifest.Preview = previewName
	manifest.Actions = manifestActions
	manifest.Capabilities = packageformat.ManifestCapabilities{
		TransparentBackground: true,
		FrameSequence:         true,
		PerFrameDuration:      true,
	}
	manifest.Compatibility = packageformat.ManifestCompatibility{
		MinRuntimeVersion: DefaultMinRuntimeVersion,
		RenderMode:        packageformat.RenderModeSprite,
	}
	manifest.Provenance = packageformat.ManifestProvenance{
		SourceType:       SourceTypeGenerated,
		GenerationTaskID: taskInfo.GenerationTaskID,
		ProcessingTaskID: snapshot.ProcessingTaskID,
		BuiltAt:          now,
		Builder:          "u-ai-release-builder",
	}

	manifest, err = a.archiveWriter.BuildManifestForArchive(stagingDir, manifest)
	if err != nil {
		a.storage.RemoveStagingDir(releaseID)
		return nil, err
	}

	manifestData, err := a.writer.WriteManifest(manifest)
	if err != nil {
		a.storage.RemoveStagingDir(releaseID)
		return nil, err
	}

	stagedFiles := make([]relbuild.StagedFile, 0, len(manifest.Integrity.Files))
	for _, f := range manifest.Integrity.Files {
		stagedFiles = append(stagedFiles, relbuild.StagedFile{
			Path:      f.Path,
			SHA256:    f.SHA256,
			Bytes:     f.Bytes,
			MediaType: f.MediaType,
			Role:      f.Role,
			ActionKey: f.ActionKey,
			FrameID:   f.FrameID,
		})
	}

	return &relbuild.StagedPackage{
		StagingDir:      stagingDir,
		Actions:         stagedActions,
		PreviewName:     previewName,
		ManifestData:    manifestData,
		ContentRootHash: manifest.Integrity.ContentRootHash,
		TotalBytes:      manifest.Integrity.TotalBytes,
		FileCount:       manifest.Integrity.FileCount,
		Files:           stagedFiles,
	}, nil
}

func (a *packageWriterAdapter) stageActionFrames(stagingDir, actionKey string, detail *ActiveRevisionDetail) ([]frameEntry, error) {
	framesDir := filepath.Join(stagingDir, "actions", actionKey, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return nil, err
	}

	frames := make([]frameEntry, 0, len(detail.Frames))
	for _, f := range detail.Frames {
		src, err := a.source.GetAssetPath(f.AssetID)
		if err != nil {
			return nil, err
		}
		ext := extForMimeType(f.MimeType)
		fileName := fmt.Sprintf("%04d%s", f.LogicalIndex, ext)
		dst := filepath.Join(framesDir, fileName)
		if err := copyFileContents(src, dst); err != nil {
			return nil, err
		}
		frames = append(frames, frameEntry{
			FrameID:     f.FrameID,
			Index:       f.LogicalIndex,
			File:        fmt.Sprintf("frames/%s", fileName),
			DurationMs:  f.DurationMS,
			AssetID:     f.AssetID,
			ContentHash: f.ContentHash,
		})
	}
	return frames, nil
}

func (a *packageWriterAdapter) ValidatePackage(staged *relbuild.StagedPackage) (*relbuild.ValidationResult, error) {
	var manifest packageformat.Manifest
	if err := json.Unmarshal(staged.ManifestData, &manifest); err != nil {
		return nil, err
	}
	report := a.validator.ValidateDirectory(staged.StagingDir, &manifest)

	findingsJSON := "[]"
	if report.Findings != nil {
		data, _ := json.Marshal(report.Findings)
		findingsJSON = string(data)
	}

	return &relbuild.ValidationResult{
		Verdict:      report.Verdict,
		FileCount:    report.FileCount,
		ErrorCount:   report.ErrorCount,
		WarningCount: report.WarningCount,
		FindingsJSON: findingsJSON,
	}, nil
}

func (a *packageWriterAdapter) WriteManifest(staged *relbuild.StagedPackage) error {
	var manifest packageformat.Manifest
	if err := json.Unmarshal(staged.ManifestData, &manifest); err != nil {
		return err
	}

	manifestData, err := a.writer.WriteManifest(&manifest)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(staged.StagingDir, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		return err
	}

	staged.ManifestData = manifestData
	staged.ManifestHash = hashBytes(manifestData)
	return nil
}

func (a *packageWriterAdapter) BuildArchive(publishedDir string, petID, releaseID string) error {
	archivePath := a.storage.ArchivePath(petID, releaseID)
	return a.archiveWriter.WriteArchive(publishedDir, archivePath)
}

func (a *packageWriterAdapter) MoveStagingToPublished(petID, releaseID string, stagingDir string) error {
	publishedDir := a.storage.PublishedDir(petID, releaseID)
	parent := filepath.Dir(publishedDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	return os.Rename(stagingDir, publishedDir)
}

func (a *packageWriterAdapter) RemoveStagingDir(stagingDir string) error {
	if filepath.IsAbs(stagingDir) {
		return removeTree(stagingDir)
	}
	return a.storage.RemoveStagingDir(stagingDir)
}

func (a *packageWriterAdapter) RemovePublishedDir(petID, releaseID string) error {
	return a.storage.RemovePublishedDir(petID, releaseID)
}

func (a *packageWriterAdapter) PublishedDir(petID, releaseID string) string {
	return a.storage.PublishedDir(petID, releaseID)
}

func (a *packageWriterAdapter) PublishedStorageKey(petID, releaseID string) string {
	return a.storage.PublishedStorageKey(petID, releaseID)
}

func (a *packageWriterAdapter) ArchiveStorageKey(petID, releaseID string) string {
	return a.storage.ArchiveStorageKey(petID, releaseID)
}
