package generation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/imageprovider"
	"gorm.io/gorm"

	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

const (
	maxArtifactImageSize = 10 * 1024 * 1024
	artifactDirName      = "artifacts"
	receiptFileName      = "provider-receipt.json"
)

var allowedArtifactMIMEs = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

type ArtifactCommitInput struct {
	Tx                  *gorm.DB
	TaskID              string
	TaskActionID        string
	AttemptID           string
	ReferenceAssetID    string
	PromptHash          string
	CandidateData       []byte
	CandidateMIME       string
	SegmentIndex        int
	CandidateIndex      int
	ArtifactType        string
	ArtifactRole        string
	IsPrimary           bool
	LayoutJSON          string
	MetadataJSON        string
	ProviderRequestID   string
	ProviderOperationID string
	StorageKey          string
	DataDir             string
}

type ArtifactCommitFunc func(input ArtifactCommitInput) (*GenerationArtifact, error)

type ArtifactPersister struct {
	artifactRepo ArtifactRepository
	commitFunc   ArtifactCommitFunc
}

func NewArtifactPersister(artifactRepo ArtifactRepository) *ArtifactPersister {
	return &ArtifactPersister{artifactRepo: artifactRepo}
}

func (p *ArtifactPersister) WithCommitFunc(fn ArtifactCommitFunc) *ArtifactPersister {
	p.commitFunc = fn
	return p
}

type PersistInput struct {
	Tx                  *gorm.DB
	TaskID              string
	TaskActionID        string
	AttemptID           string
	Plan                *GenerationPlanSnapshot
	Result              *imageprovider.GenerationResult
	SegmentIndex        int
	ExecutionID         string
	ProviderRequestID   string
	ProviderOperationID string
	DataDir             string
}

type PersistResult struct {
	Artifacts       []*GenerationArtifact
	PrimaryArtifact *GenerationArtifact
	ReceiptArtifact *GenerationArtifact
}

func (p *ArtifactPersister) Persist(input PersistInput) (*PersistResult, error) {
	if input.Result == nil {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, "generation result is nil", nil)
	}
	if !input.Result.IsSucceeded() && !input.Result.HasCandidates() {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, "generation result has no candidates", nil)
	}

	result := &PersistResult{
		Artifacts: make([]*GenerationArtifact, 0),
	}

	dataDir := input.DataDir
	if dataDir == "" {
		dataDir = config.AppCfg.Storage.DataDir
	}

	if p.commitFunc != nil {
		return p.persistWithCommitter(input, dataDir, result)
	}

	artifactDir := p.buildArtifactDir(dataDir, input.TaskID, input.TaskActionID, input.AttemptID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("create artifact dir failed: %v", err), err)
	}

	for i := range input.Result.Candidates {
		artifact, err := p.persistCandidate(input, artifactDir, &input.Result.Candidates[i], i)
		if err != nil {
			return nil, p.rollbackPersistFailure(dataDir, result, err)
		}
		result.Artifacts = append(result.Artifacts, artifact)
		if i == 0 {
			artifact.IsPrimary = 1
			result.PrimaryArtifact = artifact
		}
	}

	if result.PrimaryArtifact != nil {
		for _, a := range result.Artifacts {
			if err := p.artifactRepo.CreateArtifact(input.Tx, a); err != nil {
				persistErr := NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("create artifact record failed: %v", err), err)
				return nil, p.rollbackPersistFailure(dataDir, result, persistErr)
			}
		}
	}

	receipt, err := p.persistReceipt(input, artifactDir)
	if err != nil {
		return nil, p.rollbackPersistFailure(dataDir, result, err)
	}
	if receipt != nil {
		result.ReceiptArtifact = receipt
	}

	return result, nil
}

func (p *ArtifactPersister) persistWithCommitter(input PersistInput, dataDir string, result *PersistResult) (*PersistResult, error) {
	artifactType := p.resolveArtifactType(input.Plan)
	layoutJSON := p.extractLayoutJSON(input.Plan)
	referenceAssetID := ""
	promptHash := ""
	if input.Plan != nil {
		referenceAssetID = input.Plan.ReferenceAssetID
		promptHash = input.Plan.PromptHash
	}

	for i := range input.Result.Candidates {
		candidate := &input.Result.Candidates[i]
		metadata := map[string]any{
			"candidateIndex": i,
			"segmentIndex":   input.SegmentIndex,
		}
		if candidate.RemoteURL != "" {
			metadata["remoteUrl"] = candidate.RemoteURL
		}
		if candidate.RemoteReceipt != nil {
			metadata["remoteReceiptExpiresAt"] = candidate.RemoteReceipt.ExpiresAt
		}
		metadataJSON, _ := json.Marshal(metadata)

		commitInput := ArtifactCommitInput{
			Tx:                  input.Tx,
			TaskID:              input.TaskID,
			TaskActionID:        input.TaskActionID,
			AttemptID:           input.AttemptID,
			ReferenceAssetID:    referenceAssetID,
			PromptHash:          promptHash,
			CandidateData:       candidate.Bytes,
			CandidateMIME:       candidate.MimeType,
			SegmentIndex:        input.SegmentIndex,
			CandidateIndex:      i,
			ArtifactType:        string(artifactType),
			ArtifactRole:        string(ArtifactRolePrimary),
			IsPrimary:           i == 0,
			LayoutJSON:          layoutJSON,
			MetadataJSON:        string(metadataJSON),
			ProviderRequestID:   input.ProviderRequestID,
			ProviderOperationID: input.ProviderOperationID,
			DataDir:             dataDir,
		}

		artifact, err := p.commitFunc(commitInput)
		if err != nil {
			persistErr := NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("commit candidate %d failed: %v", i, err), err)
			return nil, p.rollbackPersistFailure(dataDir, result, persistErr)
		}
		result.Artifacts = append(result.Artifacts, artifact)
		if i == 0 {
			result.PrimaryArtifact = artifact
		}
	}

	receipt, err := p.persistReceipt(input, p.buildArtifactDir(dataDir, input.TaskID, input.TaskActionID, input.AttemptID))
	if err != nil {
		return nil, p.rollbackPersistFailure(dataDir, result, err)
	}
	if receipt != nil {
		result.ReceiptArtifact = receipt
	}

	return result, nil
}

func (p *ArtifactPersister) persistCandidate(input PersistInput, dir string, candidate *imageprovider.CandidateImage, index int) (*GenerationArtifact, error) {
	data := candidate.Bytes
	if len(data) == 0 {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d has empty data", index), nil)
	}
	if len(data) > maxArtifactImageSize {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d exceeds max size %d", index, maxArtifactImageSize), nil)
	}

	detectedMIME := http.DetectContentType(data)
	normalizedMIME := strings.TrimSpace(strings.Split(detectedMIME, ";")[0])
	ext, ok := allowedArtifactMIMEs[normalizedMIME]
	if !ok {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d has unsupported mime: %s", index, normalizedMIME), nil)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d decode failed: %v", index, err), err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d has invalid dimensions %dx%d", index, width, height), nil)
	}

	if input.Plan != nil {
		if input.Plan.SheetWidth > 0 && input.Plan.SheetHeight > 0 {
			if width != input.Plan.SheetWidth || height != input.Plan.SheetHeight {
				if input.Plan.CellWidth > 0 && input.Plan.CellHeight > 0 {
					if width != input.Plan.CellWidth || height != input.Plan.CellHeight {
						return nil, NewGenerationError(ErrCodeArtifactHashMismatch,
							fmt.Sprintf("candidate %d dimensions %dx%d mismatch plan sheet %dx%d or cell %dx%d",
								index, width, height,
								input.Plan.SheetWidth, input.Plan.SheetHeight,
								input.Plan.CellWidth, input.Plan.CellHeight), nil)
					}
				}
			}
		}
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	fileName := fmt.Sprintf("segment-%d-candidate-%d%s", input.SegmentIndex, index, ext)
	finalPath := filepath.Join(dir, fileName)
	tmpPath := finalPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("write candidate %d failed: %v", index, err), err)
	}

	verifyImg, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d verify decode failed: %v", index, err), err)
	}
	verifyBounds := verifyImg.Bounds()
	if verifyBounds.Dx() != width || verifyBounds.Dy() != height {
		_ = os.Remove(tmpPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("candidate %d verify dimension mismatch", index), nil)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("rename candidate %d failed: %v", index, err), err)
	}

	relPath := p.buildRelativePath(input.TaskID, input.TaskActionID, input.AttemptID, fileName)

	artifactType := p.resolveArtifactType(input.Plan)

	metadata := map[string]any{
		"candidateIndex": index,
		"segmentIndex":   input.SegmentIndex,
	}
	if candidate.RemoteURL != "" {
		metadata["remoteUrl"] = candidate.RemoteURL
	}
	if candidate.RemoteReceipt != nil {
		metadata["remoteReceiptExpiresAt"] = candidate.RemoteReceipt.ExpiresAt
	}
	metadataJSON, _ := json.Marshal(metadata)

	artifact := &GenerationArtifact{
		ID:                  generateUUID(),
		TaskID:              input.TaskID,
		TaskActionID:        input.TaskActionID,
		AttemptID:           input.AttemptID,
		ArtifactType:        string(artifactType),
		SegmentIndex:        input.SegmentIndex,
		CandidateIndex:      index,
		IsPrimary:           0,
		Status:              string(ArtifactStatusSaved),
		RelativePath:        relPath,
		MIME:                normalizedMIME,
		Width:               width,
		Height:              height,
		Size:                int64(len(data)),
		Hash:                hash,
		ProviderRequestID:   input.ProviderRequestID,
		ProviderOperationID: input.ProviderOperationID,
		LayoutJSON:          p.extractLayoutJSON(input.Plan),
		MetadataJSON:        string(metadataJSON),
		CreatedAt:           nowRFC3339(),
		UpdatedAt:           nowRFC3339(),
	}

	return artifact, nil
}

func (p *ArtifactPersister) persistReceipt(input PersistInput, dir string) (*GenerationArtifact, error) {
	if input.Result == nil {
		return nil, nil
	}

	receiptData := map[string]any{
		"provider":       input.Result.Provider,
		"model":          input.Result.Model,
		"operationID":    input.Result.OperationID,
		"requestID":      input.Result.RequestID,
		"candidateCount": len(input.Result.Candidates),
	}
	if input.Result.Usage != nil {
		receiptData["usage"] = input.Result.Usage
	}
	if input.Result.RawMetadata != nil {
		receiptData["rawMetadata"] = input.Result.RawMetadata
	}
	if input.Result.ErrorCode != "" {
		receiptData["errorCode"] = input.Result.ErrorCode
	}
	if input.Result.ErrorMessage != "" {
		receiptData["errorMessage"] = input.Result.ErrorMessage
	}

	receiptJSON, err := json.MarshalIndent(receiptData, "", "  ")
	if err != nil {
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("marshal receipt failed: %v", err), err)
	}

	receiptPath := filepath.Join(dir, receiptFileName)
	tmpPath := receiptPath + ".tmp"
	if err := os.WriteFile(tmpPath, receiptJSON, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("write receipt failed: %v", err), err)
	}
	if err := os.Rename(tmpPath, receiptPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("rename receipt failed: %v", err), err)
	}

	relPath := p.buildRelativePath(input.TaskID, input.TaskActionID, input.AttemptID, receiptFileName)

	artifact := &GenerationArtifact{
		ID:                  generateUUID(),
		TaskID:              input.TaskID,
		TaskActionID:        input.TaskActionID,
		AttemptID:           input.AttemptID,
		ArtifactType:        string(ArtifactTypeProviderReceipt),
		SegmentIndex:        0,
		CandidateIndex:      0,
		IsPrimary:           0,
		Status:              string(ArtifactStatusSaved),
		RelativePath:        relPath,
		MIME:                "application/json",
		Width:               0,
		Height:              0,
		Size:                int64(len(receiptJSON)),
		Hash:                computeSHA256Hex(string(receiptJSON)),
		ProviderRequestID:   input.ProviderRequestID,
		ProviderOperationID: input.ProviderOperationID,
		MetadataJSON:        "{}",
		CreatedAt:           nowRFC3339(),
		UpdatedAt:           nowRFC3339(),
	}

	if err := p.artifactRepo.CreateArtifact(input.Tx, artifact); err != nil {
		_ = os.Remove(receiptPath)
		return nil, NewGenerationError(ErrCodeArtifactPersistFailed, fmt.Sprintf("create receipt artifact failed: %v", err), err)
	}

	return artifact, nil
}

func (p *ArtifactPersister) resolveArtifactType(plan *GenerationPlanSnapshot) ArtifactType {
	if plan == nil {
		return ArtifactTypeLegacyFrameRaw
	}
	switch imageprovider.GenerationMode(plan.Mode) {
	case imageprovider.ModeSpriteSheet:
		return ArtifactTypeSpriteSheetRaw
	case imageprovider.ModeKeyframe:
		return ArtifactTypeKeyframeSheetRaw
	case imageprovider.ModeSingleFrame:
		return ArtifactTypeSingleFrameRaw
	default:
		return ArtifactTypeLegacyFrameRaw
	}
}

func (p *ArtifactPersister) extractLayoutJSON(plan *GenerationPlanSnapshot) string {
	if plan == nil {
		return ""
	}
	return plan.LayoutJSON
}

func (p *ArtifactPersister) buildArtifactDir(dataDir, taskID, taskActionID, attemptID string) string {
	if dataDir == "" {
		dataDir = config.AppCfg.Storage.DataDir
	}
	return filepath.Join(
		dataDir,
		"desktop-pets",
		"generation-tasks",
		taskID,
		"generated",
		"actions",
		taskActionID,
		"attempts",
		attemptID,
		artifactDirName,
	)
}

// RollbackPersistedFiles removes filesystem artifacts that were published as
// part of a database transaction which is about to be rolled back. Generation
// persistence writes files before committing its SQL transaction so that the
// database never points at a missing final file; the inverse failure mode must
// therefore be handled explicitly to avoid orphaned files when finalization or
// the SQL commit fails.
func (p *ArtifactPersister) RollbackPersistedFiles(dataDir string, result *PersistResult) error {
	if result == nil {
		return nil
	}
	if dataDir == "" {
		dataDir = config.AppCfg.Storage.DataDir
	}

	root, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve artifact rollback root: %w", err)
	}

	artifacts := make([]*GenerationArtifact, 0, len(result.Artifacts)+1)
	artifacts = append(artifacts, result.Artifacts...)
	if result.ReceiptArtifact != nil {
		artifacts = append(artifacts, result.ReceiptArtifact)
	}

	seen := make(map[string]struct{}, len(artifacts))
	var rollbackErrs []error
	for _, artifact := range artifacts {
		if artifact == nil || strings.TrimSpace(artifact.RelativePath) == "" {
			continue
		}

		rel := filepath.Clean(filepath.FromSlash(artifact.RelativePath))
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("refuse rollback outside data dir: %q", artifact.RelativePath))
			continue
		}

		absPath := filepath.Join(root, rel)
		containedRel, relErr := filepath.Rel(root, absPath)
		if relErr != nil || containedRel == ".." || strings.HasPrefix(containedRel, ".."+string(filepath.Separator)) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("refuse rollback outside data dir: %q", artifact.RelativePath))
			continue
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}

		if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove rolled back artifact %s: %w", artifact.RelativePath, removeErr))
		}
	}

	return errors.Join(rollbackErrs...)
}

func (p *ArtifactPersister) rollbackPersistFailure(dataDir string, result *PersistResult, cause error) error {
	if rollbackErr := p.RollbackPersistedFiles(dataDir, result); rollbackErr != nil {
		return fmt.Errorf("%w; filesystem rollback failed: %v", cause, rollbackErr)
	}
	return cause
}

func (p *ArtifactPersister) buildRelativePath(taskID, taskActionID, attemptID, fileName string) string {
	return filepath.ToSlash(filepath.Join(
		"desktop-pets",
		"generation-tasks",
		taskID,
		"generated",
		"actions",
		taskActionID,
		"attempts",
		attemptID,
		artifactDirName,
		fileName,
	))
}
