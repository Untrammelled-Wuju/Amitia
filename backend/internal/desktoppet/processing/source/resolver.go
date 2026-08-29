package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type SourceResolver interface {
	Resolve(ctx context.Context, req ResolveRequest) (*ProcessingSourceDescriptor, error)
}

type ResolveRequest struct {
	ProcessingTaskID              string
	ProcessingActionID            string
	ActionKey                     string
	GenerationTaskID              string
	UserID                        string
	SourceAttemptID               string
	SourceGenerationAttemptNumber int
	CandidateIndex                int
	DataDir                       string
}

type LegacyFrameRepo interface {
	GetGenerationTask(taskID string) (GenerationTaskInfo, error)
	ListActionsByTaskID(taskID string) ([]GenerationTaskActionInfo, error)
	ListFramesByAction(actionID string, attemptNumber int) ([]GenerationFrameInfo, error)
}

type GenerationTaskInfo struct {
	ID     string
	UserID string
	Status string
}

type GenerationTaskActionInfo struct {
	ID             string
	ActionKey      string
	ActionName     string
	CurrentAttempt int
	FrameCount     int
	SortOrder      int
}

type GenerationFrameInfo struct {
	ID              string
	FrameIndex      int
	ResultImagePath string
	ResultHash      string
	Status          string
	AttemptNumber   int
}

type LegacyFrameAdapter struct {
	repo LegacyFrameRepo
}

func NewLegacyFrameAdapter(repo LegacyFrameRepo) *LegacyFrameAdapter {
	return &LegacyFrameAdapter{repo: repo}
}

var (
	ErrTaskNotFound            = errors.New("source: generation task not found")
	ErrTaskUserMismatch        = errors.New("source: generation task user mismatch")
	ErrActionNotFound          = errors.New("source: action not found for action key")
	ErrNoFrames                = errors.New("source: no frames found for action")
	ErrFrameIndexNotContiguous = errors.New("source: frame indexes are not contiguous")
)

func (a *LegacyFrameAdapter) Resolve(ctx context.Context, req ResolveRequest) (*ProcessingSourceDescriptor, error) {
	task, err := a.repo.GetGenerationTask(req.GenerationTaskID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTaskNotFound, err)
	}
	if task.UserID != req.UserID {
		return nil, fmt.Errorf("%w: expected=%s actual=%s", ErrTaskUserMismatch, req.UserID, task.UserID)
	}

	actions, err := a.repo.ListActionsByTaskID(req.GenerationTaskID)
	if err != nil {
		return nil, fmt.Errorf("source: list actions: %w", err)
	}

	var matched *GenerationTaskActionInfo
	for i := range actions {
		if actions[i].ActionKey == req.ActionKey {
			matched = &actions[i]
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("%w: actionKey=%s", ErrActionNotFound, req.ActionKey)
	}

	attemptNumber := req.SourceGenerationAttemptNumber
	if attemptNumber <= 0 {
		attemptNumber = matched.CurrentAttempt
	}
	if attemptNumber <= 0 {
		attemptNumber = 1
	}

	frames, err := a.repo.ListFramesByAction(matched.ID, attemptNumber)
	if err != nil {
		return nil, fmt.Errorf("source: list frames: %w", err)
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("%w: actionID=%s attempt=%d", ErrNoFrames, matched.ID, attemptNumber)
	}

	sort.Slice(frames, func(i, j int) bool {
		return frames[i].FrameIndex < frames[j].FrameIndex
	})

	for i, f := range frames {
		if f.FrameIndex != i {
			return nil, fmt.Errorf("%w: expected=%d actual=%d", ErrFrameIndexNotContiguous, i, f.FrameIndex)
		}
	}

	sourceFrames := make([]ProcessingSourceFrame, 0, len(frames))
	hashParts := make([]string, 0, len(frames))
	for _, f := range frames {
		sourceFrames = append(sourceFrames, ProcessingSourceFrame{
			LogicalFrameIndex: f.FrameIndex,
			SourceArtifactID:  f.ID,
			RelativePath:      f.ResultImagePath,
			CropRect:          PixelRect{},
			ExpectedHash:      f.ResultHash,
		})
		if f.ResultHash != "" {
			hashParts = append(hashParts, f.ResultHash)
		}
	}

	descriptor := &ProcessingSourceDescriptor{
		SourceKind:       SourceLegacyFrame,
		ActionKey:        matched.ActionKey,
		GenerationMode:   "legacy_frame",
		SourceAttemptID:  req.SourceAttemptID,
		CandidateIndex:   req.CandidateIndex,
		Frames:           sourceFrames,
		SourceConfigHash: computeSourceHash(hashParts),
	}

	return descriptor, nil
}

func computeSourceHash(hashParts []string) string {
	joined := strings.Join(hashParts, "|")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}
