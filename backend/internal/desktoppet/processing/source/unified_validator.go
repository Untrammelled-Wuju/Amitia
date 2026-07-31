package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"
)

const ErrCodeProcessingSourceInvalid = "processing_source_invalid"

var ErrSourceInvalid = errors.New("source: processing source invalid")

type ManifestValidatorRepo interface {
	GetTaskUserID(taskID string) (string, error)
	GetGenerationActionInfo(generationActionID string) (*GenerationActionValidationInfo, error)
	GetArtifactValidationInfo(artifactID string) (*ArtifactValidationInfo, error)
}

type GenerationActionValidationInfo struct {
	ID        string
	ActionKey string
	TaskID    string
}

type ArtifactValidationInfo struct {
	ArtifactID   string
	AttemptID    string
	Role         string
	Status       string
	ContentHash  string
	RelativePath string
	Width        int
	Height       int
}

type UnifiedSourceValidator interface {
	Validate(ctx context.Context, manifest *ProcessingSourceManifestRecord, userID string) error
}

type unifiedSourceValidator struct {
	repo    ManifestValidatorRepo
	dataDir string
}

func NewUnifiedSourceValidator(repo ManifestValidatorRepo, dataDir string) UnifiedSourceValidator {
	return &unifiedSourceValidator{repo: repo, dataDir: dataDir}
}

func (v *unifiedSourceValidator) Validate(ctx context.Context, manifest *ProcessingSourceManifestRecord, userID string) error {
	if manifest == nil {
		return fmt.Errorf("%w: manifest is nil", ErrSourceInvalid)
	}

	if !manifest.VerifyHash() {
		return fmt.Errorf("%w: manifest hash mismatch for action %s", ErrSourceInvalid, manifest.ProcessingActionID)
	}

	if manifest.GenerationAttemptID == "" {
		return fmt.Errorf("%w: generation attempt id is empty", ErrSourceInvalid)
	}

	if manifest.SourceArtifactID == "" {
		return fmt.Errorf("%w: source artifact id is empty", ErrSourceInvalid)
	}

	if err := v.validateOwnership(manifest, userID); err != nil {
		return err
	}

	if err := v.validateGenerationAction(manifest); err != nil {
		return err
	}

	if err := v.validateArtifact(manifest); err != nil {
		return err
	}

	if err := v.validateFile(manifest); err != nil {
		return err
	}

	if err := v.validateFrames(manifest); err != nil {
		return err
	}

	return nil
}

func (v *unifiedSourceValidator) validateOwnership(manifest *ProcessingSourceManifestRecord, userID string) error {
	if userID == "" {
		return nil
	}
	ownerID, err := v.repo.GetTaskUserID(manifest.GenerationTaskID)
	if err != nil {
		return fmt.Errorf("%w: get task owner: %v", ErrSourceInvalid, err)
	}
	if ownerID != userID {
		return fmt.Errorf("%w: owner mismatch expected=%s actual=%s", ErrSourceInvalid, userID, ownerID)
	}
	return nil
}

func (v *unifiedSourceValidator) validateGenerationAction(manifest *ProcessingSourceManifestRecord) error {
	if manifest.GenerationActionID == "" {
		return nil
	}
	action, err := v.repo.GetGenerationActionInfo(manifest.GenerationActionID)
	if err != nil {
		return fmt.Errorf("%w: get generation action: %v", ErrSourceInvalid, err)
	}
	if action == nil {
		return fmt.Errorf("%w: generation action not found %s", ErrSourceInvalid, manifest.GenerationActionID)
	}
	if action.ActionKey != manifest.ActionKey {
		return fmt.Errorf("%w: action key mismatch expected=%s actual=%s", ErrSourceInvalid, manifest.ActionKey, action.ActionKey)
	}
	if action.TaskID != manifest.GenerationTaskID {
		return fmt.Errorf("%w: generation task mismatch expected=%s actual=%s", ErrSourceInvalid, manifest.GenerationTaskID, action.TaskID)
	}
	return nil
}

func (v *unifiedSourceValidator) validateArtifact(manifest *ProcessingSourceManifestRecord) error {
	artifact, err := v.repo.GetArtifactValidationInfo(manifest.SourceArtifactID)
	if err != nil {
		return fmt.Errorf("%w: get artifact: %v", ErrSourceInvalid, err)
	}
	if artifact == nil {
		return fmt.Errorf("%w: artifact not found %s", ErrSourceInvalid, manifest.SourceArtifactID)
	}

	if artifact.AttemptID != manifest.GenerationAttemptID {
		return fmt.Errorf("%w: artifact attempt mismatch expected=%s actual=%s", ErrSourceInvalid, manifest.GenerationAttemptID, artifact.AttemptID)
	}

	if artifact.Role != "" && artifact.Role != "primary" {
		return fmt.Errorf("%w: artifact role not primary: %s", ErrSourceInvalid, artifact.Role)
	}

	if !isArtifactPersisted(artifact.Status) {
		return fmt.Errorf("%w: artifact not persisted status=%s", ErrSourceInvalid, artifact.Status)
	}

	if manifest.ArtifactContentHash != "" && artifact.ContentHash != "" && artifact.ContentHash != manifest.ArtifactContentHash {
		return fmt.Errorf("%w: artifact hash mismatch expected=%s actual=%s", ErrSourceInvalid, manifest.ArtifactContentHash, artifact.ContentHash)
	}

	return nil
}

func isArtifactPersisted(status string) bool {
	switch status {
	case "verified", "saved", "persisted":
		return true
	default:
		return false
	}
}

func (v *unifiedSourceValidator) validateFile(manifest *ProcessingSourceManifestRecord) error {
	if manifest.ArtifactRelativePath == "" {
		return fmt.Errorf("%w: artifact relative path is empty", ErrSourceInvalid)
	}

	absPath, err := ResolveRelativePath(v.dataDir, manifest.ArtifactRelativePath)
	if err != nil {
		return fmt.Errorf("%w: resolve path: %v", ErrSourceInvalid, err)
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("%w: file not found: %s", ErrSourceInvalid, absPath)
	}
	if stat.IsDir() {
		return fmt.Errorf("%w: path is directory: %s", ErrSourceInvalid, absPath)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("%w: read file: %v", ErrSourceInvalid, err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("%w: decode config: %v", ErrSourceInvalid, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("%w: invalid dimensions %dx%d", ErrSourceInvalid, cfg.Width, cfg.Height)
	}

	if manifest.ArtifactWidth > 0 && manifest.ArtifactWidth != cfg.Width {
		return fmt.Errorf("%w: width mismatch expected=%d actual=%d", ErrSourceInvalid, manifest.ArtifactWidth, cfg.Width)
	}
	if manifest.ArtifactHeight > 0 && manifest.ArtifactHeight != cfg.Height {
		return fmt.Errorf("%w: height mismatch expected=%d actual=%d", ErrSourceInvalid, manifest.ArtifactHeight, cfg.Height)
	}

	if manifest.ArtifactMimeType != "" {
		if !isAllowedMIME(manifest.ArtifactMimeType) {
			return fmt.Errorf("%w: unsupported mime %s", ErrSourceInvalid, manifest.ArtifactMimeType)
		}
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return fmt.Errorf("%w: unsupported extension %s", ErrSourceInvalid, ext)
	}

	return nil
}

func isAllowedMIME(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/jpg", "image/webp":
		return true
	default:
		return false
	}
}

func (v *unifiedSourceValidator) validateFrames(manifest *ProcessingSourceManifestRecord) error {
	descriptor, err := manifest.ToDescriptor()
	if err != nil {
		return fmt.Errorf("%w: to descriptor: %v", ErrSourceInvalid, err)
	}

	if len(descriptor.Frames) == 0 {
		return fmt.Errorf("%w: no frames in manifest", ErrSourceInvalid)
	}

	if manifest.ExpectedFrameCount > 0 && len(descriptor.Frames) != manifest.ExpectedFrameCount {
		return fmt.Errorf("%w: frame count mismatch expected=%d actual=%d", ErrSourceInvalid, manifest.ExpectedFrameCount, len(descriptor.Frames))
	}

	seen := make(map[int]bool, len(descriptor.Frames))
	for _, f := range descriptor.Frames {
		if seen[f.LogicalFrameIndex] {
			return fmt.Errorf("%w: duplicate frame index %d", ErrSourceInvalid, f.LogicalFrameIndex)
		}
		seen[f.LogicalFrameIndex] = true
	}

	for i, f := range descriptor.Frames {
		if f.LogicalFrameIndex != i {
			return fmt.Errorf("%w: frame index not contiguous expected=%d actual=%d", ErrSourceInvalid, i, f.LogicalFrameIndex)
		}
	}

	if manifest.GenerationMode == "sprite_sheet" {
		if descriptor.Artifact.Layout == nil {
			return fmt.Errorf("%w: sprite sheet layout missing", ErrSourceInvalid)
		}
		if descriptor.Artifact.Layout.Rows <= 0 || descriptor.Artifact.Layout.Columns <= 0 {
			return fmt.Errorf("%w: invalid layout rows=%d cols=%d", ErrSourceInvalid, descriptor.Artifact.Layout.Rows, descriptor.Artifact.Layout.Columns)
		}
	}

	return nil
}
