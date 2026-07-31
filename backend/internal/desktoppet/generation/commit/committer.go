package commit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"gorm.io/gorm"
)

func generateUUID() string {
	return uuid.New().String()
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func computeSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var mimeExtensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

type ArtifactCommitter struct {
	journalRepo  JournalRepository
	artifactRepo generation.ArtifactRepository
	validator    *ArtifactValidator
}

func NewArtifactCommitter(journalRepo JournalRepository, artifactRepo generation.ArtifactRepository) *ArtifactCommitter {
	return &ArtifactCommitter{
		journalRepo:  journalRepo,
		artifactRepo: artifactRepo,
		validator:    NewArtifactValidator(),
	}
}

type CommitInput struct {
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

type CommitResult struct {
	Artifact *generation.GenerationArtifact
	Journal  *PublishJournal
}

func (c *ArtifactCommitter) Commit(input CommitInput) (*CommitResult, error) {
	if len(input.CandidateData) == 0 {
		return nil, fmt.Errorf("candidate data is empty")
	}
	if input.DataDir == "" {
		return nil, fmt.Errorf("data dir is empty")
	}
	if input.TaskID == "" || input.TaskActionID == "" || input.AttemptID == "" {
		return nil, fmt.Errorf("task id, task action id and attempt id are required")
	}
	if input.MetadataJSON != "" && !json.Valid([]byte(input.MetadataJSON)) {
		return nil, fmt.Errorf("invalid metadata json")
	}
	if input.LayoutJSON != "" && !json.Valid([]byte(input.LayoutJSON)) {
		return nil, fmt.Errorf("invalid layout json")
	}

	width, height, format, err := c.validator.ValidateAndMeasure(input.CandidateData, input.CandidateMIME, "", defaultMaxPixels)
	if err != nil {
		return nil, fmt.Errorf("validate artifact: %w", err)
	}

	contentHash := computeSHA256Hex(input.CandidateData)

	ext := extensionForMIME(input.CandidateMIME, format)
	fileName := fmt.Sprintf("segment-%d-candidate-%d%s", input.SegmentIndex, input.CandidateIndex, ext)

	stagingDir := filepath.Join(input.DataDir, "desktop-pets", "generation-tasks", input.TaskID, "generated", "actions", input.TaskActionID, "attempts", input.AttemptID, "staging")
	stagingPath := filepath.Join(stagingDir, fileName)

	finalDir := filepath.Join(input.DataDir, "desktop-pets", "generation-tasks", input.TaskID, "generated", "actions", input.TaskActionID, "attempts", input.AttemptID, "artifacts")
	finalPath := filepath.Join(finalDir, fileName)

	relPath := filepath.ToSlash(filepath.Join(
		"desktop-pets", "generation-tasks", input.TaskID, "generated",
		"actions", input.TaskActionID, "attempts", input.AttemptID, "artifacts", fileName,
	))

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}
	tmpPath := stagingPath + ".tmp"
	if err := os.WriteFile(tmpPath, input.CandidateData, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("write staging tmp file: %w", err)
	}
	if err := os.Rename(tmpPath, stagingPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("rename staging tmp file: %w", err)
	}

	artifactID := generateUUID()
	now := nowRFC3339()
	isPrimary := 0
	if input.IsPrimary {
		isPrimary = 1
	}
	metadataJSON := input.MetadataJSON
	if metadataJSON == "" {
		metadataJSON = "{}"
	}

	artifact := &generation.GenerationArtifact{
		ID:                     artifactID,
		TaskID:                 input.TaskID,
		TaskActionID:           input.TaskActionID,
		AttemptID:              input.AttemptID,
		ArtifactType:           input.ArtifactType,
		ArtifactRole:           input.ArtifactRole,
		SegmentIndex:           input.SegmentIndex,
		CandidateIndex:         input.CandidateIndex,
		IsPrimary:              isPrimary,
		Status:                 string(generation.ArtifactStatusStaging),
		RelativePath:           "",
		StorageKey:             input.StorageKey,
		StorageBackend:         "local",
		ContentHash:            contentHash,
		MIME:                   input.CandidateMIME,
		Width:                  width,
		Height:                 height,
		Size:                   int64(len(input.CandidateData)),
		Hash:                   contentHash,
		ProviderRequestID:      input.ProviderRequestID,
		ProviderOperationID:    input.ProviderOperationID,
		LayoutJSON:             input.LayoutJSON,
		MetadataJSON:           metadataJSON,
		SourceReferenceAssetID: input.ReferenceAssetID,
		SourcePromptHash:       input.PromptHash,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := c.artifactRepo.CreateArtifact(input.Tx, artifact); err != nil {
		_ = os.Remove(stagingPath)
		return nil, fmt.Errorf("create artifact record: %w", err)
	}

	journalID := generateUUID()
	journal := &PublishJournal{
		ID:            journalID,
		ArtifactID:    artifactID,
		AttemptID:     input.AttemptID,
		TaskID:        input.TaskID,
		TaskActionID:  input.TaskActionID,
		StagingPath:   stagingPath,
		FinalPath:     finalPath,
		ContentHash:   contentHash,
		StorageKey:    input.StorageKey,
		JournalStatus: JournalStatusStaging,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := c.journalRepo.Create(input.Tx, journal); err != nil {
		_ = os.Remove(stagingPath)
		_ = c.artifactRepo.UpdateArtifactTx(input.Tx, artifactID, map[string]interface{}{
			"status":     string(generation.ArtifactStatusPublishFailed),
			"updated_at": nowRFC3339(),
		})
		return nil, fmt.Errorf("create publish journal: %w", err)
	}

	if err := os.MkdirAll(finalDir, 0755); err != nil {
		c.markRenameFailure(artifactID, journalID, input.Tx, fmt.Sprintf("create final dir: %v", err))
		return nil, fmt.Errorf("create final dir: %w", err)
	}
	if err := os.Rename(stagingPath, finalPath); err != nil {
		c.markRenameFailure(artifactID, journalID, input.Tx, fmt.Sprintf("rename to final path: %v", err))
		return nil, fmt.Errorf("rename to final path: %w", err)
	}

	if err := c.artifactRepo.UpdateArtifactTx(input.Tx, artifactID, map[string]interface{}{
		"status":        string(generation.ArtifactStatusPersisted),
		"relative_path": relPath,
		"storage_key":   input.StorageKey,
		"content_hash":  contentHash,
	}); err != nil {
		_ = c.journalRepo.UpdateStatus(input.Tx, journalID, JournalStatusFailed, fmt.Sprintf("update artifact persisted: %v", err))
		return nil, fmt.Errorf("update artifact to persisted: %w", err)
	}

	if err := c.journalRepo.UpdateStatus(input.Tx, journalID, JournalStatusPersisted, ""); err != nil {
		return nil, fmt.Errorf("update journal to persisted: %w", err)
	}

	artifact.Status = string(generation.ArtifactStatusPersisted)
	artifact.RelativePath = relPath
	artifact.StorageKey = input.StorageKey
	artifact.ContentHash = contentHash
	journal.JournalStatus = JournalStatusPersisted
	journal.CompletedAt = nowRFC3339()

	return &CommitResult{
		Artifact: artifact,
		Journal:  journal,
	}, nil
}

func (c *ArtifactCommitter) markRenameFailure(artifactID, journalID string, tx *gorm.DB, errMsg string) {
	now := nowRFC3339()
	_ = c.artifactRepo.UpdateArtifactTx(tx, artifactID, map[string]interface{}{
		"status":     string(generation.ArtifactStatusPublishFailed),
		"updated_at": now,
	})
	_ = c.journalRepo.UpdateStatus(tx, journalID, JournalStatusFailed, errMsg)
}

func extensionForMIME(mime, format string) string {
	if ext, ok := mimeExtensions[mime]; ok {
		return ext
	}
	switch format {
	case "png":
		return ".png"
	case "jpeg", "jpg":
		return ".jpg"
	case "webp":
		return ".webp"
	}
	return ".bin"
}
