package commit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing/source"
	"github.com/u-ai/backend/internal/desktoppet/processing/workspace"
	"gorm.io/gorm"
)

var _ Committer = (*ProcessingCommitter)(nil)
var _ CommitRepository = (*ProcessingCommitter)(nil)

type CommitRequest struct {
	Ctx                context.Context
	ProcessingTaskID   string
	ProcessingActionID string
	ActionKey          string
	AttemptID          string
	GenerationTaskID   string
	ProcessingVersion  int
	SourceManifestID   string
	ConfigSnapshot     string
	ConfigHash         string
	PipelineVersion    string
	PipelineResult     *PipelineResult
}

type PipelineResult struct {
	FrameCount       int
	FramesDir        string
	Frames           []FrameResult
	RootRelativePath string
	RevisionHash     string
}

type FrameResult struct {
	Index     int
	FilePath  string
	MaskPath  string
	FileHash  string
	PixelHash string
	Width     int
	Height    int
}

type CommitResult struct {
	RevisionID       string
	RevisionNumber   int
	RootRelativePath string
	Status           string
}

type Committer interface {
	Commit(req *CommitRequest) (*CommitResult, error)
}

type CommitRepository interface {
	CreateRevision(ctx context.Context, revision *processing.ProcessingRevision) error
	GetLatestRevisionNumber(ctx context.Context, processingActionID string) (int, error)
	UpdateRevisionStatus(ctx context.Context, revisionID, status, errorMessage string) error
}

type ProcessingCommitter struct {
	db        *gorm.DB
	repo      processing.Repository
	workspace *workspace.WorkspaceManager
	dataDir   string
}

func NewProcessingCommitter(db *gorm.DB, repo processing.Repository, ws *workspace.WorkspaceManager, dataDir string) *ProcessingCommitter {
	return &ProcessingCommitter{
		db:        db,
		repo:      repo,
		workspace: ws,
		dataDir:   dataDir,
	}
}

func (c *ProcessingCommitter) Commit(req *CommitRequest) (*CommitResult, error) {
	if req == nil {
		return nil, fmt.Errorf("commit: request is nil")
	}
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}
	if err := validatePipelineResult(req.PipelineResult); err != nil {
		return nil, fmt.Errorf("commit: validate pipeline result: %w", err)
	}

	commitID := "commit_" + uuid.NewString()
	revisionID := "rev_" + uuid.NewString()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	ws, err := c.workspace.CreateWorkspace(req.ProcessingTaskID, req.ProcessingActionID, req.AttemptID, commitID)
	if err != nil {
		return nil, fmt.Errorf("commit: create workspace: %w", err)
	}
	stagingDir := ws.StagingDir

	if err := c.copyPipelineToStaging(req.PipelineResult, stagingDir); err != nil {
		cleanupStaging(stagingDir)
		return nil, fmt.Errorf("commit: copy to staging: %w", err)
	}
	if err := c.writeRevisionManifest(stagingDir, revisionID, req); err != nil {
		cleanupStaging(stagingDir)
		return nil, fmt.Errorf("commit: write revision manifest: %w", err)
	}
	if err := c.writeSourceManifestRef(stagingDir, req); err != nil {
		cleanupStaging(stagingDir)
		return nil, fmt.Errorf("commit: write source manifest ref: %w", err)
	}

	contentRootHash, err := c.computeContentRootHash(req.PipelineResult)
	if err != nil {
		cleanupStaging(stagingDir)
		return nil, fmt.Errorf("commit: compute content root hash: %w", err)
	}

	latestNumber, err := c.GetLatestRevisionNumber(req.Ctx, req.ProcessingActionID)
	if err != nil {
		cleanupStaging(stagingDir)
		return nil, fmt.Errorf("commit: get latest revision number: %w", err)
	}
	revisionNumber := latestNumber + 1

	rootRelative := filepath.ToSlash(filepath.Join(
		"desktop-pets",
		"processing-tasks",
		req.ProcessingTaskID,
		"revisions",
		revisionID,
	))

	rev := &processing.ProcessingRevision{
		ID:                 revisionID,
		ProcessingTaskID:   req.ProcessingTaskID,
		ProcessingActionID: req.ProcessingActionID,
		RevisionNumber:     revisionNumber,
		SourceAttemptID:    req.AttemptID,
		Status:             contracts.RevisionStatusPreparing,
		ConfigSnapshot:     req.ConfigSnapshot,
		ConfigHash:         req.ConfigHash,
		PipelineVersion:    req.PipelineVersion,
		FrameCount:         req.PipelineResult.FrameCount,
		RootRelativePath:   rootRelative,
		RevisionHash:       req.PipelineResult.RevisionHash,
		Active:             0,
		ContentRootHash:    contentRootHash,
		SourceManifestID:   req.SourceManifestID,
		CommitID:           commitID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := c.CreateRevision(req.Ctx, rev); err != nil {
		cleanupStaging(stagingDir)
		return nil, fmt.Errorf("commit: create revision: %w", err)
	}

	finalDir := c.workspace.FinalDir(req.ProcessingTaskID, req.ProcessingActionID, revisionID)
	if err := c.workspace.AtomicPublish(stagingDir, finalDir); err != nil {
		cleanupStaging(stagingDir)
		c.markFailed(req.Ctx, revisionID, "atomic_publish_failed", err.Error())
		return nil, fmt.Errorf("commit: atomic publish: %w", err)
	}

	if err := c.UpdateRevisionStatus(req.Ctx, revisionID, contracts.RevisionStatusFilesPublished, ""); err != nil {
		c.markFailed(req.Ctx, revisionID, "status_update_failed", err.Error())
		return nil, fmt.Errorf("commit: update status to files_published: %w", err)
	}

	if err := c.commitToDatabase(req.Ctx, revisionID); err != nil {
		c.markFailed(req.Ctx, revisionID, "db_commit_failed", err.Error())
		return nil, fmt.Errorf("commit: db commit: %w", err)
	}

	return &CommitResult{
		RevisionID:       revisionID,
		RevisionNumber:   revisionNumber,
		RootRelativePath: rootRelative,
		Status:           contracts.RevisionStatusDBCommitted,
	}, nil
}

func (c *ProcessingCommitter) CreateRevision(ctx context.Context, revision *processing.ProcessingRevision) error {
	if revision == nil {
		return fmt.Errorf("commit: revision is nil")
	}
	return c.repo.CreateProcessingRevision(c.db.WithContext(ctx), revision)
}

func (c *ProcessingCommitter) GetLatestRevisionNumber(ctx context.Context, processingActionID string) (int, error) {
	var revs []processing.ProcessingRevision
	err := c.db.WithContext(ctx).
		Where("processing_action_id = ?", processingActionID).
		Order("revision_number DESC").
		Limit(1).
		Find(&revs).Error
	if err != nil {
		return 0, err
	}
	if len(revs) == 0 {
		return 0, nil
	}
	return revs[0].RevisionNumber, nil
}

func (c *ProcessingCommitter) UpdateRevisionStatus(ctx context.Context, revisionID, status, errorMessage string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}
	if errorMessage != "" {
		updates["error_code"] = status
		updates["error_message"] = errorMessage
	} else {
		updates["error_code"] = ""
		updates["error_message"] = ""
	}
	return c.db.WithContext(ctx).
		Model(&processing.ProcessingRevision{}).
		Where("id = ?", revisionID).
		Updates(updates).Error
}

func (c *ProcessingCommitter) commitToDatabase(ctx context.Context, revisionID string) error {
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		return tx.Model(&processing.ProcessingRevision{}).
			Where("id = ?", revisionID).
			Updates(map[string]interface{}{
				"status":       contracts.RevisionStatusDBCommitted,
				"published_at": now,
				"updated_at":   now,
			}).Error
	})
}

func (c *ProcessingCommitter) markFailed(ctx context.Context, revisionID, errorCode, errorMessage string) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_ = c.db.WithContext(ctx).
		Model(&processing.ProcessingRevision{}).
		Where("id = ?", revisionID).
		Updates(map[string]interface{}{
			"status":        contracts.RevisionStatusFailed,
			"error_code":    errorCode,
			"error_message": errorMessage,
			"updated_at":    now,
		}).Error
}

func (c *ProcessingCommitter) copyPipelineToStaging(result *PipelineResult, stagingDir string) error {
	framesDst := filepath.Join(stagingDir, "frames")
	masksDst := filepath.Join(stagingDir, "masks")
	if err := os.MkdirAll(framesDst, 0755); err != nil {
		return fmt.Errorf("create frames staging dir: %w", err)
	}
	if err := os.MkdirAll(masksDst, 0755); err != nil {
		return fmt.Errorf("create masks staging dir: %w", err)
	}
	for _, frame := range result.Frames {
		dst := filepath.Join(framesDst, frameFileName(frame))
		if err := copyFileVerified(frame.FilePath, dst); err != nil {
			return fmt.Errorf("copy frame %d: %w", frame.Index, err)
		}
		if frame.MaskPath != "" {
			maskDst := filepath.Join(masksDst, maskFileName(frame))
			if err := copyFileVerified(frame.MaskPath, maskDst); err != nil {
				return fmt.Errorf("copy mask %d: %w", frame.Index, err)
			}
		}
	}
	return nil
}

func (c *ProcessingCommitter) writeRevisionManifest(stagingDir, revisionID string, req *CommitRequest) error {
	frames := make([]contracts.ManifestFrame, 0, len(req.PipelineResult.Frames))
	for _, f := range req.PipelineResult.Frames {
		mf := contracts.ManifestFrame{
			Index:     f.Index,
			File:      filepath.Base(f.FilePath),
			FileHash:  f.FileHash,
			PixelHash: f.PixelHash,
		}
		if f.MaskPath != "" {
			mf.Mask = filepath.Base(f.MaskPath)
		}
		frames = append(frames, mf)
	}
	manifest := contracts.RevisionManifest{
		SchemaVersion:      contracts.RevisionManifestSchemaVersion,
		RevisionID:         revisionID,
		ProcessingTaskID:   req.ProcessingTaskID,
		ProcessingActionID: req.ProcessingActionID,
		ActionKey:          req.ActionKey,
		Source: contracts.ManifestSource{
			AttemptID: req.AttemptID,
		},
		PipelineVersion: req.PipelineVersion,
		ConfigHash:      req.ConfigHash,
		FrameCount:      req.PipelineResult.FrameCount,
		Frames:          frames,
		RevisionHash:    req.PipelineResult.RevisionHash,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	path := filepath.Join(stagingDir, "revision.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	return nil
}

func (c *ProcessingCommitter) writeSourceManifestRef(stagingDir string, req *CommitRequest) error {
	ref := source.ProcessingSourceManifestRecord{
		ID:                  req.SourceManifestID,
		ProcessingTaskID:    req.ProcessingTaskID,
		ProcessingActionID:  req.ProcessingActionID,
		GenerationTaskID:    req.GenerationTaskID,
		ActionKey:           req.ActionKey,
		GenerationAttemptID: req.AttemptID,
		CreatedAt:           time.Now().UTC().Format("2006-01-02 15:04:05"),
	}
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal source manifest ref: %w", err)
	}
	path := filepath.Join(stagingDir, "source-manifest-ref.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write source manifest ref %s: %w", path, err)
	}
	return nil
}

func (c *ProcessingCommitter) computeContentRootHash(result *PipelineResult) (string, error) {
	h := sha256.New()
	for _, frame := range result.Frames {
		data, err := os.ReadFile(frame.FilePath)
		if err != nil {
			return "", fmt.Errorf("read frame %d: %w", frame.Index, err)
		}
		sum := sha256.Sum256(data)
		h.Write(sum[:])
		h.Write([]byte(frame.FileHash))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validatePipelineResult(result *PipelineResult) error {
	if result == nil {
		return fmt.Errorf("pipeline result is nil")
	}
	if result.FrameCount <= 0 {
		return fmt.Errorf("frame count must be positive, got %d", result.FrameCount)
	}
	if result.FramesDir == "" {
		return fmt.Errorf("frames dir is empty")
	}
	if result.RevisionHash == "" {
		return fmt.Errorf("revision hash is empty")
	}
	if len(result.Frames) == 0 {
		return fmt.Errorf("no frames in pipeline result")
	}
	if len(result.Frames) != result.FrameCount {
		return fmt.Errorf("frame count mismatch: frames=%d frameCount=%d", len(result.Frames), result.FrameCount)
	}
	for i, f := range result.Frames {
		if f.FilePath == "" {
			return fmt.Errorf("frame %d file path is empty", i)
		}
		if f.FileHash == "" {
			return fmt.Errorf("frame %d file hash is empty", i)
		}
		if f.PixelHash == "" {
			return fmt.Errorf("frame %d pixel hash is empty", i)
		}
	}
	return nil
}

func cleanupStaging(stagingDir string) {
	_ = os.RemoveAll(stagingDir)
}

func frameFileName(frame FrameResult) string {
	ext := filepath.Ext(frame.FilePath)
	return fmt.Sprintf("frame_%d%s", frame.Index, ext)
}

func maskFileName(frame FrameResult) string {
	ext := filepath.Ext(frame.MaskPath)
	if ext == "" {
		ext = ".png"
	}
	return fmt.Sprintf("mask_%d%s", frame.Index, ext)
}

func copyFileVerified(src, dst string) error {
	srcHash, err := hashFile(src)
	if err != nil {
		return fmt.Errorf("hash source: %w", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("write destination: %w", err)
	}
	dstHash, err := hashFile(dst)
	if err != nil {
		return fmt.Errorf("hash destination: %w", err)
	}
	if srcHash != dstHash {
		return fmt.Errorf("hash mismatch after copy: %s != %s", srcHash, dstHash)
	}
	return nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
