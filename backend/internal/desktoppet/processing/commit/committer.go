package commit

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/application"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing/events"
	"github.com/u-ai/backend/internal/desktoppet/processing/measurement"
	"github.com/u-ai/backend/internal/desktoppet/processing/source"
	"github.com/u-ai/backend/internal/desktoppet/processing/workspace"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
)

var _ Committer = (*ProcessingCommitter)(nil)

type CommitRequest struct {
	Ctx                        context.Context
	UserID                     string
	CharacterID                string
	ProcessingTaskID           string
	ProcessingActionID         string
	ProcessingAttemptID        string
	ActionKey                  string
	SourceManifestID           string
	SourceGenerationAttemptID  string
	SourceGenerationArtifactID string
	SourceArtifactContentHash  string
	ConfigSnapshot             string
	ConfigHash                 string
	PipelineVersion            string
	PipelineResult             *application.ProcessActionResult
	ExpectedActionRowVersion   int64
	ExecutionID                string
	LeaseOwner                 string
}

type CommitResult struct {
	CommitID        string
	RevisionID      string
	RevisionNumber  int
	RootStorageKey  string
	Status          string
	ContentRootHash string
}

type Committer interface {
	Commit(req *CommitRequest) (*CommitResult, error)
}

type ProcessingCommitter struct {
	db            *gorm.DB
	repo          processing.Repository
	workspace     *workspace.WorkspaceManager
	commitJournal events.CommitJournalRepository
	outbox        *events.EventOutbox
	manifestStore source.ManifestStore
	dataDir       string
	now           func() string
}

func NewProcessingCommitter(
	db *gorm.DB,
	repo processing.Repository,
	ws *workspace.WorkspaceManager,
	commitJournal events.CommitJournalRepository,
	outbox *events.EventOutbox,
	manifestStore source.ManifestStore,
	dataDir string,
) *ProcessingCommitter {
	return &ProcessingCommitter{
		db:            db,
		repo:          repo,
		workspace:     ws,
		commitJournal: commitJournal,
		outbox:        outbox,
		manifestStore: manifestStore,
		dataDir:       dataDir,
		now:           func() string { return time.Now().UTC().Format("2006-01-02 15:04:05") },
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
	now := c.now()

	journal := &events.CommitJournal{
		ID:                  "cj_" + uuid.NewString(),
		CommitID:            commitID,
		ProcessingAttemptID: req.ProcessingAttemptID,
		SourceManifestID:    req.SourceManifestID,
		Status:              events.CommitJournalStatusCreated,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := c.commitJournal.Create(c.db, journal); err != nil {
		return nil, fmt.Errorf("commit: create journal: %w", err)
	}

	ws, err := c.workspace.CreateWorkspace(req.ProcessingTaskID, req.ProcessingActionID, req.ProcessingAttemptID, commitID)
	if err != nil {
		return nil, fmt.Errorf("commit: create workspace: %w", err)
	}
	stagingDir := ws.StagingDir

	if err := c.copyPipelineToStaging(req.PipelineResult, stagingDir); err != nil {
		cleanupStaging(stagingDir)
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: copy to staging: %w", err)
	}

	_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusStagingPrepared, "")

	if err := c.writeRevisionManifest(stagingDir, revisionID, req); err != nil {
		cleanupStaging(stagingDir)
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: write revision manifest: %w", err)
	}

	if err := c.writeSourceManifestRef(stagingDir, req); err != nil {
		cleanupStaging(stagingDir)
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: write source manifest ref: %w", err)
	}

	contentRootHash, err := c.computeContentRootHash(req.PipelineResult, stagingDir, req)
	if err != nil {
		cleanupStaging(stagingDir)
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: compute content root hash: %w", err)
	}

	rootStorageKey := filepath.ToSlash(filepath.Join(
		"desktop-pets", "processing-tasks",
		req.ProcessingTaskID, "revisions", revisionID,
	))

	revisionNumber, err := c.allocateRevisionNumber(req.ProcessingActionID)
	if err != nil {
		cleanupStaging(stagingDir)
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: allocate revision number: %w", err)
	}

	rev := &processing.ProcessingRevision{
		ID:                         revisionID,
		ProcessingTaskID:           req.ProcessingTaskID,
		ProcessingActionID:         req.ProcessingActionID,
		ProcessingAttemptID:        req.ProcessingAttemptID,
		RevisionNumber:             revisionNumber,
		SourceAttemptID:            req.ProcessingAttemptID,
		SourceManifestID:           req.SourceManifestID,
		SourceGenerationAttemptID:  req.SourceGenerationAttemptID,
		SourceGenerationArtifactID: req.SourceGenerationArtifactID,
		SourceArtifactContentHash:  req.SourceArtifactContentHash,
		Status:                     contracts.RevisionStatusPreparing,
		ConfigSnapshot:             req.ConfigSnapshot,
		ConfigHash:                 req.ConfigHash,
		PipelineVersion:            req.PipelineVersion,
		FrameCount:                 req.PipelineResult.FrameCount,
		RootRelativePath:           rootStorageKey,
		RootStorageKey:             rootStorageKey,
		RevisionHash:               req.PipelineResult.RevisionHash,
		ContentRootHash:            contentRootHash,
		CommitID:                   commitID,
		Active:                     0,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}

	artifacts := c.buildArtifactRecords(revisionID, req)
	measurements := c.buildMeasurementRecords(revisionID, req)
	transforms := c.buildTransformRecords(revisionID, req)

	if err := c.db.WithContext(req.Ctx).Transaction(func(tx *gorm.DB) error {
		if err := c.repo.CreateProcessingRevision(tx, rev); err != nil {
			return fmt.Errorf("create revision: %w", err)
		}
		if len(artifacts) > 0 {
			if err := c.repo.CreateProcessingArtifacts(tx, artifacts); err != nil {
				return fmt.Errorf("create artifacts: %w", err)
			}
		}
		if len(measurements) > 0 {
			if err := c.repo.CreateFrameMeasurements(tx, measurements); err != nil {
				return fmt.Errorf("create measurements: %w", err)
			}
		}
		if len(transforms) > 0 {
			if err := c.repo.CreateProcessingTransforms(tx, transforms); err != nil {
				return fmt.Errorf("create transforms: %w", err)
			}
		}
		if err := c.commitJournal.UpdateStatus(tx, commitID, events.CommitJournalStatusRevisionRecorded, ""); err != nil {
			return fmt.Errorf("update journal: %w", err)
		}
		if err := c.commitJournal.UpdateRevisionID(tx, commitID, revisionID); err != nil {
			return fmt.Errorf("update journal revision id: %w", err)
		}
		return nil
	}); err != nil {
		cleanupStaging(stagingDir)
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: db transaction: %w", err)
	}

	finalDir := c.workspace.FinalDir(req.ProcessingTaskID, req.ProcessingActionID, revisionID)
	if err := c.workspace.AtomicPublish(stagingDir, finalDir); err != nil {
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, "atomic_publish_failed: "+err.Error())
		return nil, fmt.Errorf("commit: atomic publish: %w", err)
	}

	_ = c.commitJournal.UpdatePaths(c.db, commitID, stagingDir, finalDir, contentRootHash)
	_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFilesPublished, "")

	if err := c.db.WithContext(req.Ctx).Transaction(func(tx *gorm.DB) error {
		nowInner := c.now()
		if err := tx.Model(&processing.ProcessingRevision{}).
			Where("id = ?", revisionID).
			Updates(map[string]interface{}{
				"status":       contracts.RevisionStatusCommitted,
				"committed_at": nowInner,
				"updated_at":   nowInner,
			}).Error; err != nil {
			return fmt.Errorf("update revision to committed: %w", err)
		}

		if err := tx.Model(&processing.ProcessingActionAttempt{}).
			Where("id = ?", req.ProcessingAttemptID).
			Updates(map[string]interface{}{
				"status":       "committed",
				"commit_id":    commitID,
				"completed_at": nowInner,
				"updated_at":   nowInner,
			}).Error; err != nil {
			return fmt.Errorf("update attempt to committed: %w", err)
		}

		if _, err := c.repo.UpdateProcessingActionWithRowVersion(tx, req.ProcessingActionID, req.ExpectedActionRowVersion, map[string]interface{}{
			"status":             "succeeded",
			"active_revision_id": revisionID,
			"completed_at":       nowInner,
			"updated_at":         nowInner,
		}); err != nil {
			return fmt.Errorf("update action to succeeded: %w", err)
		}

		if err := c.commitJournal.UpdateStatus(tx, commitID, events.CommitJournalStatusRecordsCommitted, ""); err != nil {
			return fmt.Errorf("update journal to records_committed: %w", err)
		}

		outboxEvent := events.ProcessingRevisionCommittedEvent{
			UserID:                     req.UserID,
			CharacterID:                req.CharacterID,
			ProcessingTaskID:           req.ProcessingTaskID,
			ProcessingActionID:         req.ProcessingActionID,
			ProcessingAttemptID:        req.ProcessingAttemptID,
			ProcessingRevisionID:       revisionID,
			RevisionNumber:             revisionNumber,
			ActionKey:                  req.ActionKey,
			SourceManifestID:           req.SourceManifestID,
			SourceGenerationAttemptID:  req.SourceGenerationAttemptID,
			SourceGenerationArtifactID: req.SourceGenerationArtifactID,
			SourceArtifactContentHash:  req.SourceArtifactContentHash,
			FrameCount:                 req.PipelineResult.FrameCount,
			RevisionHash:               req.PipelineResult.RevisionHash,
			ContentRootHash:            contentRootHash,
			ConfigHash:                 req.ConfigHash,
			PipelineVersion:            req.PipelineVersion,
			OccurredAt:                 nowInner,
		}
		if err := c.outbox.EmitProcessingRevisionCommitted(tx, outboxEvent); err != nil {
			return fmt.Errorf("emit outbox event: %w", err)
		}

		if err := c.commitJournal.UpdateStatus(tx, commitID, events.CommitJournalStatusEventCommitted, ""); err != nil {
			return fmt.Errorf("update journal to event_committed: %w", err)
		}
		return nil
	}); err != nil {
		c.markRevisionFailed(req.Ctx, revisionID, "db_final_commit_failed", err.Error())
		_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusFailedRetryable, err.Error())
		return nil, fmt.Errorf("commit: final db transaction: %w", err)
	}

	_ = c.commitJournal.UpdateStatus(c.db, commitID, events.CommitJournalStatusCompleted, "")

	return &CommitResult{
		CommitID:        commitID,
		RevisionID:      revisionID,
		RevisionNumber:  revisionNumber,
		RootStorageKey:  rootStorageKey,
		Status:          contracts.RevisionStatusCommitted,
		ContentRootHash: contentRootHash,
	}, nil
}

func (c *ProcessingCommitter) allocateRevisionNumber(processingActionID string) (int, error) {
	var action processing.ProcessingAction
	if err := c.db.Where("id = ?", processingActionID).First(&action).Error; err != nil {
		return 0, fmt.Errorf("get action: %w", err)
	}
	nextNum := action.NextRevisionNumber
	if nextNum <= 0 {
		nextNum = 1
	}
	result := c.db.Model(&processing.ProcessingAction{}).
		Where("id = ? AND next_revision_number = ?", processingActionID, nextNum).
		Update("next_revision_number", nextNum+1)
	if result.Error != nil {
		return 0, fmt.Errorf("cas next_revision_number: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var refreshed processing.ProcessingAction
		if err := c.db.Where("id = ?", processingActionID).First(&refreshed).Error; err != nil {
			return 0, fmt.Errorf("reload action after cas miss: %w", err)
		}
		retryNum := refreshed.NextRevisionNumber
		if retryNum <= 0 {
			retryNum = 1
		}
		retryResult := c.db.Model(&processing.ProcessingAction{}).
			Where("id = ? AND next_revision_number = ?", processingActionID, retryNum).
			Update("next_revision_number", retryNum+1)
		if retryResult.Error != nil {
			return 0, fmt.Errorf("cas retry: %w", retryResult.Error)
		}
		if retryResult.RowsAffected == 0 {
			return 0, fmt.Errorf("revision number cas conflict after retry")
		}
		return retryNum, nil
	}
	return nextNum, nil
}

func (c *ProcessingCommitter) buildArtifactRecords(revisionID string, req *CommitRequest) []processing.ProcessingArtifactRecord {
	now := c.now()
	records := make([]processing.ProcessingArtifactRecord, 0, len(req.PipelineResult.Frames)+len(req.PipelineResult.Masks)+2)

	for _, f := range req.PipelineResult.Frames {
		idx := f.Index
		records = append(records, processing.ProcessingArtifactRecord{
			ID:               "art_" + uuid.NewString(),
			RevisionID:       revisionID,
			FrameIndex:       &idx,
			ArtifactKind:     contracts.ArtifactKindFrame,
			Stage:            "final",
			RelativePath:     filepath.ToSlash(filepath.Join("frames", f.FileName)),
			MimeType:         "image/png",
			Width:            f.Width,
			Height:           f.Height,
			ByteSize:         f.ByteSize,
			ContentHash:      f.FileHash,
			SourceArtifactID: req.SourceGenerationArtifactID,
			CreatedAt:        now,
		})
	}

	for _, m := range req.PipelineResult.Masks {
		idx := m.Index
		records = append(records, processing.ProcessingArtifactRecord{
			ID:               "art_" + uuid.NewString(),
			RevisionID:       revisionID,
			FrameIndex:       &idx,
			ArtifactKind:     contracts.ArtifactKindMask,
			Stage:            "final",
			RelativePath:     filepath.ToSlash(filepath.Join("masks", m.FileName)),
			MimeType:         "image/png",
			Width:            m.Width,
			Height:           m.Height,
			ByteSize:         m.ByteSize,
			ContentHash:      m.FileHash,
			SourceArtifactID: req.SourceGenerationArtifactID,
			CreatedAt:        now,
		})
	}

	records = append(records, processing.ProcessingArtifactRecord{
		ID:           "art_" + uuid.NewString(),
		RevisionID:   revisionID,
		ArtifactKind: contracts.ArtifactKindManifest,
		Stage:        "final",
		RelativePath: "revision.json",
		MimeType:     "application/json",
		ContentHash:  "",
		CreatedAt:    now,
	})

	records = append(records, processing.ProcessingArtifactRecord{
		ID:           "art_" + uuid.NewString(),
		RevisionID:   revisionID,
		ArtifactKind: "source-manifest",
		Stage:        "final",
		RelativePath: "source-manifest-ref.json",
		MimeType:     "application/json",
		ContentHash:  "",
		CreatedAt:    now,
	})

	if req.PipelineResult.Preview != nil {
		records = append(records, processing.ProcessingArtifactRecord{
			ID:           "art_" + uuid.NewString(),
			RevisionID:   revisionID,
			ArtifactKind: "preview",
			Stage:        "final",
			RelativePath: filepath.ToSlash(filepath.Join("preview", req.PipelineResult.Preview.FileName)),
			MimeType:     "image/png",
			Width:        req.PipelineResult.Preview.Width,
			Height:       req.PipelineResult.Preview.Height,
			ByteSize:     req.PipelineResult.Preview.ByteSize,
			ContentHash:  req.PipelineResult.Preview.FileHash,
			CreatedAt:    now,
		})
	}

	return records
}

func (c *ProcessingCommitter) buildMeasurementRecords(revisionID string, req *CommitRequest) []processing.FrameMeasurementRecord {
	now := c.now()
	records := make([]processing.FrameMeasurementRecord, 0, len(req.PipelineResult.Measurements))
	for _, m := range req.PipelineResult.Measurements {
		subjBox, _ := json.Marshal(measurement.SubjectBoxData{
			MinX: m.SubjectBox.MinX, MinY: m.SubjectBox.MinY,
			MaxX: m.SubjectBox.MaxX, MaxY: m.SubjectBox.MaxY,
			Space: m.SubjectBox.Space,
		})
		srcAnchor, _ := json.Marshal(m.SourceAnchor)
		tgtAnchor, _ := json.Marshal(m.TargetAnchor)
		edge, _ := json.Marshal(m.EdgeContact)
		clip, _ := json.Marshal(m.Clipping)
		traj, _ := json.Marshal(m.Trajectory)
		records = append(records, processing.FrameMeasurementRecord{
			ID:                       "meas_" + uuid.NewString(),
			RevisionID:               revisionID,
			FrameIndex:               m.FrameIndex,
			MeasurementSchemaVersion: 1,
			SubjectBoxJSON:           string(subjBox),
			SourceAnchorJSON:         string(srcAnchor),
			TargetAnchorJSON:         string(tgtAnchor),
			AlphaCoverage:            m.AlphaCoverage,
			ComponentCount:           m.ComponentCount,
			EdgeContactJSON:          string(edge),
			ClippingJSON:             string(clip),
			TrajectoryJSON:           string(traj),
			CreatedAt:                now,
			UpdatedAt:                now,
		})
	}
	return records
}

func (c *ProcessingCommitter) buildTransformRecords(revisionID string, req *CommitRequest) []processing.ProcessingTransformRecord {
	now := c.now()
	records := make([]processing.ProcessingTransformRecord, 0, len(req.PipelineResult.Transforms))
	for _, t := range req.PipelineResult.Transforms {
		records = append(records, processing.ProcessingTransformRecord{
			ID:               "tf_" + uuid.NewString(),
			RevisionID:       revisionID,
			FrameIndex:       t.FrameIndex,
			SequenceNumber:   t.SequenceNumber,
			FromSpace:        t.FromSpace,
			ToSpace:          t.ToSpace,
			TransformType:    t.TransformType,
			MatrixJSON:       t.MatrixJSON,
			ParametersJSON:   t.ParametersJSON,
			AlgorithmVersion: t.AlgorithmVersion,
			CreatedAt:        now,
		})
	}
	return records
}

func (c *ProcessingCommitter) markRevisionFailed(ctx context.Context, revisionID, errorCode, errorMessage string) {
	now := c.now()
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

func (c *ProcessingCommitter) copyPipelineToStaging(result *application.ProcessActionResult, stagingDir string) error {
	if result.WorkDir == nil {
		return fmt.Errorf("pipeline result workDir is nil")
	}
	framesDst := filepath.Join(stagingDir, "frames")
	masksDst := filepath.Join(stagingDir, "masks")
	if err := os.MkdirAll(framesDst, 0755); err != nil {
		return fmt.Errorf("create frames staging dir: %w", err)
	}
	if err := os.MkdirAll(masksDst, 0755); err != nil {
		return fmt.Errorf("create masks staging dir: %w", err)
	}
	for _, frame := range result.Frames {
		src := filepath.Join(result.WorkDir.FramesDir, frame.FileName)
		dst := filepath.Join(framesDst, frame.FileName)
		if err := copyFileVerified(src, dst); err != nil {
			return fmt.Errorf("copy frame %d: %w", frame.Index, err)
		}
	}
	for _, mask := range result.Masks {
		src := filepath.Join(result.WorkDir.MasksDir, mask.FileName)
		dst := filepath.Join(masksDst, mask.FileName)
		if err := copyFileVerified(src, dst); err != nil {
			return fmt.Errorf("copy mask %d: %w", mask.Index, err)
		}
	}
	if result.Preview != nil {
		previewDst := filepath.Join(stagingDir, "preview")
		if err := os.MkdirAll(previewDst, 0755); err != nil {
			return fmt.Errorf("create preview staging dir: %w", err)
		}
		if result.WorkDir != nil {
			src := filepath.Join(result.WorkDir.FramesDir, "..", "preview", result.Preview.FileName)
			dst := filepath.Join(previewDst, result.Preview.FileName)
			if err := copyFileVerified(src, dst); err != nil {
				return fmt.Errorf("copy preview: %w", err)
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
			File:      f.FileName,
			FileHash:  f.FileHash,
			PixelHash: f.PixelHash,
		}
		if len(req.PipelineResult.Masks) > f.Index {
			mf.Mask = req.PipelineResult.Masks[f.Index].FileName
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
			AttemptID:   req.SourceGenerationAttemptID,
			ArtifactID:  req.SourceGenerationArtifactID,
			ContentHash: req.SourceArtifactContentHash,
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
	ref := map[string]string{
		"sourceManifestId":           req.SourceManifestID,
		"sourceGenerationAttemptId":  req.SourceGenerationAttemptID,
		"sourceGenerationArtifactId": req.SourceGenerationArtifactID,
		"sourceArtifactContentHash":  req.SourceArtifactContentHash,
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

func (c *ProcessingCommitter) computeContentRootHash(result *application.ProcessActionResult, stagingDir string, req *CommitRequest) (string, error) {
	type fileEntry struct {
		relPath string
		hash    string
		bytes   int64
	}
	entries := make([]fileEntry, 0, len(result.Frames)+len(result.Masks)+4)

	for _, f := range result.Frames {
		dst := filepath.Join(stagingDir, "frames", f.FileName)
		h, b, err := hashAndSize(dst)
		if err != nil {
			return "", fmt.Errorf("hash frame %d: %w", f.Index, err)
		}
		entries = append(entries, fileEntry{relPath: "frames/" + f.FileName, hash: h, bytes: b})
	}
	for _, m := range result.Masks {
		dst := filepath.Join(stagingDir, "masks", m.FileName)
		h, b, err := hashAndSize(dst)
		if err != nil {
			return "", fmt.Errorf("hash mask %d: %w", m.Index, err)
		}
		entries = append(entries, fileEntry{relPath: "masks/" + m.FileName, hash: h, bytes: b})
	}

	manifestPath := filepath.Join(stagingDir, "revision.json")
	if h, b, err := hashAndSize(manifestPath); err == nil {
		entries = append(entries, fileEntry{relPath: "revision.json", hash: h, bytes: b})
	}
	refPath := filepath.Join(stagingDir, "source-manifest-ref.json")
	if h, b, err := hashAndSize(refPath); err == nil {
		entries = append(entries, fileEntry{relPath: "source-manifest-ref.json", hash: h, bytes: b})
	}

	entries = append(entries, fileEntry{relPath: "config.hash", hash: req.ConfigHash, bytes: int64(len(req.ConfigHash))})
	entries = append(entries, fileEntry{relPath: "pipeline.version", hash: req.PipelineVersion, bytes: int64(len(req.PipelineVersion))})

	for _, m := range result.Measurements {
		mJSON, _ := m.ToJSON()
		sum := sha256.Sum256([]byte(mJSON))
		entries = append(entries, fileEntry{
			relPath: fmt.Sprintf("measurements/frame_%04d.json", m.FrameIndex),
			hash:    hex.EncodeToString(sum[:]),
			bytes:   int64(len(mJSON)),
		})
	}
	for _, t := range result.Transforms {
		tJSON := t.MatrixJSON + t.ParametersJSON
		sum := sha256.Sum256([]byte(tJSON))
		entries = append(entries, fileEntry{
			relPath: fmt.Sprintf("transforms/frame_%04d_seq_%d.json", t.FrameIndex, t.SequenceNumber),
			hash:    hex.EncodeToString(sum[:]),
			bytes:   int64(len(tJSON)),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.relPath))
		h.Write([]byte(e.hash))
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(e.bytes))
		h.Write(buf[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validatePipelineResult(result *application.ProcessActionResult) error {
	if result == nil {
		return fmt.Errorf("pipeline result is nil")
	}
	if result.FrameCount <= 0 {
		return fmt.Errorf("frame count must be positive, got %d", result.FrameCount)
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
	if result.WorkDir == nil {
		return fmt.Errorf("workDir is nil")
	}
	for i, f := range result.Frames {
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
	_ = security.SafeRemoveTree(stagingDir)
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

func hashAndSize(path string) (string, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), info.Size(), nil
}
