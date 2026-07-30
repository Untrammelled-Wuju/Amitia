package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type ArtifactSourceRepo interface {
	GetTaskActionID(taskID, actionKey string) (string, error)
	GetActiveAttemptInfo(taskActionID string) (*AttemptInfo, error)
	GetPrimaryArtifact(attemptID string) (*ArtifactInfo, error)
	GetTaskUserID(taskID string) (string, error)
}

type AttemptInfo struct {
	ID     string
	Mode   string
	Status string
}

type ArtifactInfo struct {
	ArtifactID    string
	TaskID        string
	TaskActionID  string
	AttemptID     string
	ArtifactType  string
	Status        string
	RelativePath  string
	Hash          string
	Width         int
	Height        int
	LayoutJSON    string
	IsPrimary     bool
}

var (
	ErrUnsupportedGenerationMode = errors.New("source: unsupported generation mode")
	ErrSourcePlanNotFound        = errors.New("source: generation plan not found")
	ErrSourceAttemptNotFound     = errors.New("source: active attempt not found")
	ErrSourceArtifactNotFound    = errors.New("source: primary artifact not found")
	ErrSourceArtifactNotReady    = errors.New("source: artifact not ready")
	ErrSourceHashMismatch        = errors.New("source: artifact hash mismatch")
	ErrSourceLayoutInvalid       = errors.New("source: layout invalid")
	ErrSourceOwnerMismatch       = errors.New("source: owner mismatch")
	ErrSourceModeUnsupported     = errors.New("source: generation mode unsupported for this adapter")
)

type SpriteSheetSourceAdapter struct {
	repo ArtifactSourceRepo
}

func NewSpriteSheetSourceAdapter(repo ArtifactSourceRepo) *SpriteSheetSourceAdapter {
	return &SpriteSheetSourceAdapter{repo: repo}
}

func (a *SpriteSheetSourceAdapter) Resolve(ctx context.Context, req ResolveRequest) (*ProcessingSourceDescriptor, error) {
	if err := validateOwnership(a.repo, req); err != nil {
		return nil, err
	}

	taskActionID, err := a.repo.GetTaskActionID(req.GenerationTaskID, req.ActionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: taskID=%s actionKey=%s", ErrSourcePlanNotFound, req.GenerationTaskID, req.ActionKey)
	}

	attempt, err := a.repo.GetActiveAttemptInfo(taskActionID)
	if err != nil {
		return nil, fmt.Errorf("%w: taskActionID=%s", ErrSourceAttemptNotFound, taskActionID)
	}
	if attempt == nil {
		return nil, fmt.Errorf("%w: taskActionID=%s", ErrSourceAttemptNotFound, taskActionID)
	}

	artifact, err := a.repo.GetPrimaryArtifact(attempt.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: attemptID=%s", ErrSourceArtifactNotFound, attempt.ID)
	}
	if artifact == nil {
		return nil, fmt.Errorf("%w: attemptID=%s", ErrSourceArtifactNotFound, attempt.ID)
	}
	if artifact.Status != "verified" && artifact.Status != "saved" {
		return nil, fmt.Errorf("%w: artifactID=%s status=%s", ErrSourceArtifactNotReady, artifact.ArtifactID, artifact.Status)
	}

	layout, err := parseSpriteSheetLayout(artifact.LayoutJSON)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSourceLayoutInvalid, err)
	}
	if layout == nil || layout.Rows <= 0 || layout.Columns <= 0 {
		return nil, fmt.Errorf("%w: rows=%d cols=%d", ErrSourceLayoutInvalid, layoutSafe(layout).Rows, layoutSafe(layout).Columns)
	}

	frames := buildSpriteSheetFrames(layout, artifact)

	hashParts := make([]string, 0, len(frames))
	for _, f := range frames {
		if f.ExpectedHash != "" {
			hashParts = append(hashParts, f.ExpectedHash)
		}
	}

	descriptor := &ProcessingSourceDescriptor{
		SourceKind:      SourceSpriteSheet,
		ActionKey:       req.ActionKey,
		GenerationMode:  "sprite_sheet",
		SourceAttemptID: attempt.ID,
		CandidateIndex:  req.CandidateIndex,
		Artifact: GenerationArtifactDescriptor{
			ArtifactID:   artifact.ArtifactID,
			AttemptID:    artifact.AttemptID,
			RelativePath: artifact.RelativePath,
			ContentHash:  artifact.Hash,
			Width:        artifact.Width,
			Height:       artifact.Height,
			Primary:      true,
			Layout:       layout,
		},
		Frames:           frames,
		SourceConfigHash: computeSourceHash(hashParts),
	}

	return descriptor, nil
}

type SingleFrameSourceAdapter struct {
	repo ArtifactSourceRepo
}

func NewSingleFrameSourceAdapter(repo ArtifactSourceRepo) *SingleFrameSourceAdapter {
	return &SingleFrameSourceAdapter{repo: repo}
}

func (a *SingleFrameSourceAdapter) Resolve(ctx context.Context, req ResolveRequest) (*ProcessingSourceDescriptor, error) {
	if err := validateOwnership(a.repo, req); err != nil {
		return nil, err
	}

	taskActionID, err := a.repo.GetTaskActionID(req.GenerationTaskID, req.ActionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: taskID=%s actionKey=%s", ErrSourcePlanNotFound, req.GenerationTaskID, req.ActionKey)
	}

	attempt, err := a.repo.GetActiveAttemptInfo(taskActionID)
	if err != nil {
		return nil, fmt.Errorf("%w: taskActionID=%s", ErrSourceAttemptNotFound, taskActionID)
	}
	if attempt == nil {
		return nil, fmt.Errorf("%w: taskActionID=%s", ErrSourceAttemptNotFound, taskActionID)
	}

	artifact, err := a.repo.GetPrimaryArtifact(attempt.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: attemptID=%s", ErrSourceArtifactNotFound, attempt.ID)
	}
	if artifact == nil {
		return nil, fmt.Errorf("%w: attemptID=%s", ErrSourceArtifactNotFound, attempt.ID)
	}
	if artifact.Status != "verified" && artifact.Status != "saved" {
		return nil, fmt.Errorf("%w: artifactID=%s status=%s", ErrSourceArtifactNotReady, artifact.ArtifactID, artifact.Status)
	}

	frame := ProcessingSourceFrame{
		LogicalFrameIndex: 0,
		SourceArtifactID:  artifact.ArtifactID,
		RelativePath:      artifact.RelativePath,
		ExpectedHash:      artifact.Hash,
		ExpectedWidth:     artifact.Width,
		ExpectedHeight:    artifact.Height,
	}

	descriptor := &ProcessingSourceDescriptor{
		SourceKind:      SourceSingleFrame,
		ActionKey:       req.ActionKey,
		GenerationMode:  "single_frame",
		SourceAttemptID: attempt.ID,
		CandidateIndex:  req.CandidateIndex,
		Artifact: GenerationArtifactDescriptor{
			ArtifactID:   artifact.ArtifactID,
			AttemptID:    artifact.AttemptID,
			RelativePath: artifact.RelativePath,
			ContentHash:  artifact.Hash,
			Width:        artifact.Width,
			Height:       artifact.Height,
			Primary:      true,
		},
		Frames:           []ProcessingSourceFrame{frame},
		SourceConfigHash: computeSourceHash([]string{artifact.Hash}),
	}

	return descriptor, nil
}

type KeyframeSourceAdapter struct {
	repo ArtifactSourceRepo
}

func NewKeyframeSourceAdapter(repo ArtifactSourceRepo) *KeyframeSourceAdapter {
	return &KeyframeSourceAdapter{repo: repo}
}

func (a *KeyframeSourceAdapter) Resolve(ctx context.Context, req ResolveRequest) (*ProcessingSourceDescriptor, error) {
	if err := validateOwnership(a.repo, req); err != nil {
		return nil, err
	}

	taskActionID, err := a.repo.GetTaskActionID(req.GenerationTaskID, req.ActionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: taskID=%s actionKey=%s", ErrSourcePlanNotFound, req.GenerationTaskID, req.ActionKey)
	}

	attempt, err := a.repo.GetActiveAttemptInfo(taskActionID)
	if err != nil {
		return nil, fmt.Errorf("%w: taskActionID=%s", ErrSourceAttemptNotFound, taskActionID)
	}
	if attempt == nil {
		return nil, fmt.Errorf("%w: taskActionID=%s", ErrSourceAttemptNotFound, taskActionID)
	}

	artifact, err := a.repo.GetPrimaryArtifact(attempt.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: attemptID=%s", ErrSourceArtifactNotFound, attempt.ID)
	}
	if artifact == nil {
		return nil, fmt.Errorf("%w: attemptID=%s", ErrSourceArtifactNotFound, attempt.ID)
	}
	if artifact.Status != "verified" && artifact.Status != "saved" {
		return nil, fmt.Errorf("%w: artifactID=%s status=%s", ErrSourceArtifactNotReady, artifact.ArtifactID, artifact.Status)
	}

	layout, _ := parseSpriteSheetLayout(artifact.LayoutJSON)
	frames := buildKeyframeFrames(layout, artifact)

	descriptor := &ProcessingSourceDescriptor{
		SourceKind:      SourceKeyframe,
		ActionKey:       req.ActionKey,
		GenerationMode:  "keyframe",
		SourceAttemptID: attempt.ID,
		CandidateIndex:  req.CandidateIndex,
		Artifact: GenerationArtifactDescriptor{
			ArtifactID:   artifact.ArtifactID,
			AttemptID:    artifact.AttemptID,
			RelativePath: artifact.RelativePath,
			ContentHash:  artifact.Hash,
			Width:        artifact.Width,
			Height:       artifact.Height,
			Primary:      true,
			Layout:       layout,
		},
		Frames:           frames,
		SourceConfigHash: computeSourceHash([]string{artifact.Hash}),
	}

	return descriptor, nil
}

func validateOwnership(repo ArtifactSourceRepo, req ResolveRequest) error {
	if req.UserID == "" {
		return nil
	}
	ownerID, err := repo.GetTaskUserID(req.GenerationTaskID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSourcePlanNotFound, err)
	}
	if ownerID != req.UserID {
		return fmt.Errorf("%w: expected=%s actual=%s", ErrSourceOwnerMismatch, req.UserID, ownerID)
	}
	return nil
}

func parseSpriteSheetLayout(layoutJSON string) (*SpriteSheetLayoutSnapshot, error) {
	if layoutJSON == "" || layoutJSON == "null" {
		return nil, nil
	}
	var layout SpriteSheetLayoutSnapshot
	if err := json.Unmarshal([]byte(layoutJSON), &layout); err != nil {
		return nil, err
	}
	if layout.Rows <= 0 || layout.Columns <= 0 {
		return nil, fmt.Errorf("invalid layout dimensions: rows=%d cols=%d", layout.Rows, layout.Columns)
	}
	return &layout, nil
}

func layoutSafe(l *SpriteSheetLayoutSnapshot) *SpriteSheetLayoutSnapshot {
	if l == nil {
		return &SpriteSheetLayoutSnapshot{}
	}
	return l
}

func buildSpriteSheetFrames(layout *SpriteSheetLayoutSnapshot, artifact *ArtifactInfo) []ProcessingSourceFrame {
	frames := make([]ProcessingSourceFrame, 0, len(layout.Cells))
	sortedCells := make([]SpriteSheetCell, len(layout.Cells))
	copy(sortedCells, layout.Cells)
	sort.Slice(sortedCells, func(i, j int) bool {
		return cellFrameIndex(sortedCells[i]) < cellFrameIndex(sortedCells[j])
	})

	for _, cell := range sortedCells {
		if cell.Empty {
			continue
		}
		fi := cellFrameIndex(cell)
		frames = append(frames, ProcessingSourceFrame{
			LogicalFrameIndex: fi,
			SourceArtifactID:  artifact.ArtifactID,
			RelativePath:      artifact.RelativePath,
			CropRect: PixelRect{
				MinX: cell.X,
				MinY: cell.Y,
				MaxX: cell.X + cell.Width,
				MaxY: cell.Y + cell.Height,
			},
			ExpectedHash:   artifact.Hash,
			ExpectedWidth:  cell.Width,
			ExpectedHeight: cell.Height,
		})
	}

	if len(frames) == 0 {
		frameIdx := 0
		for row := 0; row < layout.Rows; row++ {
			for col := 0; col < layout.Columns; col++ {
				if isEmptyCell(layout.EmptyCellIndexes, frameIdx) {
					frameIdx++
					continue
				}
				cw := layout.CellWidth
				ch := layout.CellHeight
				if cw <= 0 {
					cw = artifact.Width / layout.Columns
				}
				if ch <= 0 {
					ch = artifact.Height / layout.Rows
				}
				frames = append(frames, ProcessingSourceFrame{
					LogicalFrameIndex: frameIdx,
					SourceArtifactID:  artifact.ArtifactID,
					RelativePath:      artifact.RelativePath,
					CropRect: PixelRect{
						MinX: col * cw,
						MinY: row * ch,
						MaxX: (col + 1) * cw,
						MaxY: (row + 1) * ch,
					},
					ExpectedHash:   artifact.Hash,
					ExpectedWidth:  cw,
					ExpectedHeight: ch,
				})
				frameIdx++
			}
		}
	}

	return frames
}

func buildKeyframeFrames(layout *SpriteSheetLayoutSnapshot, artifact *ArtifactInfo) []ProcessingSourceFrame {
	if layout != nil && len(layout.Cells) > 0 {
		return buildSpriteSheetFrames(layout, artifact)
	}
	return []ProcessingSourceFrame{
		{
			LogicalFrameIndex: 0,
			SourceArtifactID:  artifact.ArtifactID,
			RelativePath:      artifact.RelativePath,
			ExpectedHash:      artifact.Hash,
			ExpectedWidth:     artifact.Width,
			ExpectedHeight:    artifact.Height,
		},
	}
}

func cellFrameIndex(cell SpriteSheetCell) int {
	if cell.FrameIndex != nil {
		return *cell.FrameIndex
	}
	return cell.CellIndex
}

func isEmptyCell(emptyIndexes []int, idx int) bool {
	for _, e := range emptyIndexes {
		if e == idx {
			return true
		}
	}
	return false
}

func ResolveRelativePath(dataDir, relativePath string) (string, error) {
	cleaned := filepath.Clean(relativePath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path not allowed: %s", relativePath)
	}
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path traversal not allowed: %s", relativePath)
	}
	abs := filepath.Join(dataDir, cleaned)
	absCleaned := filepath.Clean(abs)
	return absCleaned, nil
}

func ComputeContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
