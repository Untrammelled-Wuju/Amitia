package editing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	OpFrameReorder           = "frame.reorder"
	OpFrameDelete            = "frame.delete"
	OpFrameRestore           = "frame.restore"
	OpFrameDuplicate         = "frame.duplicate"
	OpFrameInsertAsset       = "frame.insert_asset"
	OpFrameReplaceAsset      = "frame.replace_asset"
	OpFrameSetDuration       = "frame.set_duration"
	OpFrameBatchSetDuration  = "frame.batch_set_duration"
	OpActionSetDefaultFPS    = "action.set_default_fps"
	OpActionSetLoopType      = "action.set_loop_type"
	OpActionSetReturnAction  = "action.set_return_action"
	OpActionSetInterruptible = "action.set_interruptible"
	OpActionSetPriority      = "action.set_priority_override"
	OpActionSetCooldown      = "action.set_cooldown_override"
	OpBackgroundApplyPatch   = "background.apply_patch"
	OpBackgroundResetPatch   = "background.reset_patch"
	OpAnchorSetFrame         = "anchor.set_frame"
	OpAnchorBatchOffset      = "anchor.batch_offset"
	OpAnchorReset            = "anchor.reset"

	ManifestSchemaVersion  = 1
	OperationSchemaVersion = 1
	LineageOriginal        = "original"
	LineageDuplicated      = "duplicated"
	LineageInserted        = "inserted"
	LineageReplaced        = "replaced"
	LineageRegenerated     = "regenerated"
	DefaultFrameDurationMS = 100
	DefaultAnchorX         = 0.5
	DefaultAnchorY         = 0.5
)

type Service interface {
	CheckProcessingTaskOwnership(ctx context.Context, processingTaskID, userID string) error
	ListRevisions(ctx context.Context, processingTaskID, actionKey string) ([]RevisionSummary, error)
	GetRevision(ctx context.Context, revisionID string) (*RevisionDetail, error)
	GetActiveRevision(ctx context.Context, processingTaskID, actionKey string) (*RevisionDetail, error)
	ActivateRevision(ctx context.Context, processingTaskID, actionKey, revisionID string, expectedVersion int64, reason, userID string) error
	GetPreviewManifest(ctx context.Context, revisionID string) (*RevisionManifest, error)
	GetFrameImage(ctx context.Context, revisionID, frameID string) (path, mimeType string, err error)
	GetFrameThumbnail(ctx context.Context, revisionID, frameID string) (path, mimeType string, err error)
	GetActionEditSummary(ctx context.Context, processingTaskID, actionKey string) (*ActionEditSummary, error)

	CreateSession(ctx context.Context, processingTaskID, actionKey, userID string, req CreateSessionRequest) (*CreateSessionResponse, error)
	GetSession(ctx context.Context, sessionID string) (*EditSession, error)
	ApplyOperation(ctx context.Context, sessionID, userID string, req ApplyOperationRequest) (*ApplyOperationResponse, error)
	Undo(ctx context.Context, sessionID, userID string, baseVersion int64) (*ApplyOperationResponse, error)
	Redo(ctx context.Context, sessionID, userID string, baseVersion int64) (*ApplyOperationResponse, error)
	CreateCheckpoint(ctx context.Context, sessionID string) error
	CommitSession(ctx context.Context, sessionID, userID string, req CommitSessionRequest) (*CommitSessionResponse, error)
	AbandonSession(ctx context.Context, sessionID, userID string) error
	GetSessionEvents(ctx context.Context, sessionID string) ([]SessionEvent, error)

	CreateRegenerationJob(ctx context.Context, sessionID, userID string, req CreateRegenerationJobRequest) (*CreateRegenerationJobResponse, error)
	GetRegenerationJob(ctx context.Context, jobID string) (*RegenerationJob, error)
	ListRegenerationJobs(ctx context.Context, userID string, limit, offset int) ([]RegenerationJob, error)
	CancelRegenerationJob(ctx context.Context, jobID, userID string) error
	AcceptCandidate(ctx context.Context, candidateID, userID string, req AcceptCandidateRequest) error
	RejectCandidate(ctx context.Context, candidateID, userID string, req RejectCandidateRequest) error
	ListCandidates(ctx context.Context, sessionID string) ([]EditCandidate, error)

	UploadCandidate(ctx context.Context, sessionID, userID string, data []byte, mimeType string, targetFrameID string) (*UploadCandidateResponse, error)

	ApplyBackgroundPatch(ctx context.Context, sessionID, frameID, userID string, req BackgroundApplyPatchPayload) error
	ResetBackgroundPatch(ctx context.Context, sessionID, frameID, userID string) error
	SetFrameAnchor(ctx context.Context, sessionID, userID string, req AnchorSetFramePayload) error
	BatchOffsetAnchors(ctx context.Context, sessionID, userID string, req AnchorBatchOffsetPayload) error
	ResetAnchors(ctx context.Context, sessionID, userID string, req AnchorResetPayload) error

	TriggerQualityEvaluation(ctx context.Context, revisionID string) (string, error)
	GetLatestQualityEvaluation(ctx context.Context, revisionID string) (*QualityEvaluationInfo, error)

	RecoverPendingJournals(ctx context.Context) error
	ExpireSessions(ctx context.Context) error

	ImportLegacyRevision(ctx context.Context, processingTaskID, actionKey, userID string) (*CommitSessionResponse, error)

	ListActionStreams(ctx context.Context, userID string) ([]ActionStreamSummary, error)
	ListRevisionsByStream(ctx context.Context, userID string, streamID string) ([]RevisionSummary, error)
	GetActiveRevisionByStream(ctx context.Context, userID string, streamID string) (*RevisionDetail, error)
}

type service struct {
	repo        Repository
	assetStore  RevisionAssetStore
	genAdapter  GenerationAdapter
	procAdapter ProcessingAdapter
	qualAdapter QualityAdapter
	db          *gorm.DB
	dataDir     string
}

func NewService(repo Repository, assetStore RevisionAssetStore, genAdapter GenerationAdapter, procAdapter ProcessingAdapter, qualAdapter QualityAdapter, db *gorm.DB, dataDir string) Service {
	return &service{
		repo:        repo,
		assetStore:  assetStore,
		genAdapter:  genAdapter,
		procAdapter: procAdapter,
		qualAdapter: qualAdapter,
		db:          db,
		dataDir:     dataDir,
	}
}

type draftFrame struct {
	FrameID           string
	AssetID           string
	LogicalIndex      int
	DurationMS        int
	AnchorX           float64
	AnchorY           float64
	AnchorSpace       string
	SourceFrameID     string
	SourceRevisionID  string
	SourceAttemptID   string
	LineageType       string
	MaskAssetID       string
	TransformJSON     string
	MetadataJSON      string
	CopiedFromFrameID string
}

type draftState struct {
	Frames             []draftFrame
	DeletedFrames      map[string]draftFrame
	DefaultFPS         int
	LoopType           string
	ReturnAction       string
	Interruptible      bool
	PriorityOverride   *int
	CooldownMSOverride *int
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func readFileBytes(path string) []byte {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func (ds *draftState) findFrameIndex(frameID string) int {
	for i, f := range ds.Frames {
		if f.FrameID == frameID {
			return i
		}
	}
	return -1
}

func (ds *draftState) reindex() {
	for i := range ds.Frames {
		ds.Frames[i].LogicalIndex = i
	}
}

func (ds *draftState) insertAt(idx int, frame draftFrame) {
	ds.Frames = append(ds.Frames, draftFrame{})
	copy(ds.Frames[idx+1:], ds.Frames[idx:])
	ds.Frames[idx] = frame
}

func (ds *draftState) removeAt(idx int) draftFrame {
	frame := ds.Frames[idx]
	ds.Frames = append(ds.Frames[:idx], ds.Frames[idx+1:]...)
	return frame
}

func (ds *draftState) resolveInsertIndex(beforeFrameID, afterFrameID string) (int, error) {
	if beforeFrameID != "" {
		idx := ds.findFrameIndex(beforeFrameID)
		if idx == -1 {
			return 0, ErrFrameNotFound
		}
		return idx, nil
	}
	if afterFrameID != "" {
		idx := ds.findFrameIndex(afterFrameID)
		if idx == -1 {
			return 0, ErrFrameNotFound
		}
		return idx + 1, nil
	}
	return len(ds.Frames), nil
}

func (ds *draftState) neighborIDs(idx int) (beforeID, afterID string) {
	if idx > 0 {
		beforeID = ds.Frames[idx-1].FrameID
	}
	if idx < len(ds.Frames)-1 {
		afterID = ds.Frames[idx+1].FrameID
	}
	return
}

func (ds *draftState) applyReorder(p FrameReorderPayload) (FrameReorderPayload, error) {
	idx := ds.findFrameIndex(p.FrameID)
	if idx == -1 {
		return FrameReorderPayload{}, ErrFrameNotFound
	}
	beforeID, afterID := ds.neighborIDs(idx)
	frame := ds.removeAt(idx)
	insertIdx, err := ds.resolveInsertIndex(p.BeforeFrameID, p.AfterFrameID)
	if err != nil {
		ds.insertAt(idx, frame)
		return FrameReorderPayload{}, err
	}
	ds.insertAt(insertIdx, frame)
	ds.reindex()
	inverse := FrameReorderPayload{FrameID: p.FrameID}
	if beforeID != "" {
		inverse.AfterFrameID = beforeID
	} else if afterID != "" {
		inverse.BeforeFrameID = afterID
	}
	return inverse, nil
}

func (ds *draftState) applyDelete(p FrameDeletePayload) (FrameRestorePayload, error) {
	idx := ds.findFrameIndex(p.FrameID)
	if idx == -1 {
		return FrameRestorePayload{}, ErrFrameNotFound
	}
	beforeID, afterID := ds.neighborIDs(idx)
	frame := ds.removeAt(idx)
	if ds.DeletedFrames == nil {
		ds.DeletedFrames = make(map[string]draftFrame)
	}
	ds.DeletedFrames[p.FrameID] = frame
	ds.reindex()
	inverse := FrameRestorePayload{FrameID: p.FrameID}
	if beforeID != "" {
		inverse.AfterFrameID = beforeID
	} else if afterID != "" {
		inverse.BeforeFrameID = afterID
	}
	return inverse, nil
}

func (ds *draftState) applyRestore(p FrameRestorePayload) (FrameDeletePayload, error) {
	frame, ok := ds.DeletedFrames[p.FrameID]
	if !ok {
		return FrameDeletePayload{}, ErrFrameNotFound
	}
	insertIdx, err := ds.resolveInsertIndex(p.BeforeFrameID, p.AfterFrameID)
	if err != nil {
		return FrameDeletePayload{}, err
	}
	ds.insertAt(insertIdx, frame)
	delete(ds.DeletedFrames, p.FrameID)
	ds.reindex()
	return FrameDeletePayload{FrameID: p.FrameID}, nil
}

func (ds *draftState) applyDuplicate(p FrameDuplicatePayload) (FrameDeletePayload, error) {
	idx := ds.findFrameIndex(p.FrameID)
	if idx == -1 {
		return FrameDeletePayload{}, ErrFrameNotFound
	}
	src := ds.Frames[idx]
	newFrame := draftFrame{
		FrameID:           generateID("frame"),
		AssetID:           src.AssetID,
		DurationMS:        src.DurationMS,
		AnchorX:           src.AnchorX,
		AnchorY:           src.AnchorY,
		AnchorSpace:       src.AnchorSpace,
		SourceFrameID:     src.FrameID,
		LineageType:       LineageDuplicated,
		CopiedFromFrameID: src.FrameID,
	}
	insertIdx := idx + 1
	if p.AfterFrameID != "" {
		afterIdx := ds.findFrameIndex(p.AfterFrameID)
		if afterIdx != -1 {
			insertIdx = afterIdx + 1
		}
	}
	ds.insertAt(insertIdx, newFrame)
	ds.reindex()
	return FrameDeletePayload{FrameID: newFrame.FrameID}, nil
}

func (ds *draftState) applyInsertAsset(p FrameInsertAssetPayload) (FrameDeletePayload, error) {
	newFrame := draftFrame{
		FrameID:     generateID("frame"),
		AssetID:     p.AssetID,
		DurationMS:  p.DurationMS,
		AnchorX:     DefaultAnchorX,
		AnchorY:     DefaultAnchorY,
		AnchorSpace: AnchorSpaceNormalizedCanvas,
		LineageType: LineageInserted,
	}
	if newFrame.DurationMS == 0 {
		newFrame.DurationMS = DefaultFrameDurationMS
	}
	insertIdx, err := ds.resolveInsertIndex(p.BeforeFrameID, p.AfterFrameID)
	if err != nil {
		return FrameDeletePayload{}, err
	}
	ds.insertAt(insertIdx, newFrame)
	ds.reindex()
	return FrameDeletePayload{FrameID: newFrame.FrameID}, nil
}

func (ds *draftState) applyReplaceAsset(p FrameReplaceAssetPayload) (FrameReplaceAssetPayload, error) {
	idx := ds.findFrameIndex(p.FrameID)
	if idx == -1 {
		return FrameReplaceAssetPayload{}, ErrFrameNotFound
	}
	oldAssetID := ds.Frames[idx].AssetID
	inverse := FrameReplaceAssetPayload{
		FrameID:    p.FrameID,
		AssetID:    oldAssetID,
		KeepAnchor: true,
	}
	ds.Frames[idx].AssetID = p.AssetID
	if !p.KeepAnchor {
		ds.Frames[idx].AnchorX = DefaultAnchorX
		ds.Frames[idx].AnchorY = DefaultAnchorY
		ds.Frames[idx].AnchorSpace = AnchorSpaceNormalizedCanvas
	}
	ds.Frames[idx].LineageType = LineageReplaced
	return inverse, nil
}

func (ds *draftState) applySetDuration(p FrameSetDurationPayload) (FrameSetDurationPayload, error) {
	idx := ds.findFrameIndex(p.FrameID)
	if idx == -1 {
		return FrameSetDurationPayload{}, ErrFrameNotFound
	}
	if p.DurationMS < MinFrameDurationMS || p.DurationMS > MaxFrameDurationMS {
		return FrameSetDurationPayload{}, ErrFrameDurationInvalid
	}
	oldDuration := ds.Frames[idx].DurationMS
	ds.Frames[idx].DurationMS = p.DurationMS
	return FrameSetDurationPayload{FrameID: p.FrameID, DurationMS: oldDuration}, nil
}

func (ds *draftState) applyBatchSetDuration(p FrameBatchSetDurationPayload) (map[string]int, error) {
	if p.DurationMS < MinFrameDurationMS || p.DurationMS > MaxFrameDurationMS {
		return nil, ErrFrameDurationInvalid
	}
	oldDurations := make(map[string]int)
	for _, frameID := range p.FrameIDs {
		idx := ds.findFrameIndex(frameID)
		if idx == -1 {
			return nil, ErrFrameNotFound
		}
		oldDurations[frameID] = ds.Frames[idx].DurationMS
		ds.Frames[idx].DurationMS = p.DurationMS
	}
	return oldDurations, nil
}

func (ds *draftState) applySetDefaultFPS(p ActionSetDefaultFPSPayload) (ActionSetDefaultFPSPayload, error) {
	old := ds.DefaultFPS
	ds.DefaultFPS = p.DefaultFPS
	return ActionSetDefaultFPSPayload{DefaultFPS: old, Recalculate: false}, nil
}

func (ds *draftState) applySetLoopType(p ActionSetLoopTypePayload) (ActionSetLoopTypePayload, error) {
	old := ds.LoopType
	ds.LoopType = p.LoopType
	return ActionSetLoopTypePayload{LoopType: old}, nil
}

func (ds *draftState) applySetReturnAction(p ActionSetReturnActionPayload) (ActionSetReturnActionPayload, error) {
	old := ds.ReturnAction
	ds.ReturnAction = p.ReturnAction
	return ActionSetReturnActionPayload{ReturnAction: old}, nil
}

func (ds *draftState) applySetInterruptible(p ActionSetInterruptiblePayload) (ActionSetInterruptiblePayload, error) {
	old := ds.Interruptible
	ds.Interruptible = p.Interruptible
	return ActionSetInterruptiblePayload{Interruptible: old}, nil
}

func (ds *draftState) applySetPriority(p ActionSetPriorityOverridePayload) (ActionSetPriorityOverridePayload, error) {
	old := ds.PriorityOverride
	ds.PriorityOverride = p.Priority
	return ActionSetPriorityOverridePayload{Priority: old}, nil
}

func (ds *draftState) applySetCooldown(p ActionSetCooldownOverridePayload) (ActionSetCooldownOverridePayload, error) {
	old := ds.CooldownMSOverride
	ds.CooldownMSOverride = p.CooldownMS
	return ActionSetCooldownOverridePayload{CooldownMS: old}, nil
}

func (ds *draftState) applySetFrameAnchor(p AnchorSetFramePayload) (AnchorSetFramePayload, error) {
	idx := ds.findFrameIndex(p.FrameID)
	if idx == -1 {
		return AnchorSetFramePayload{}, ErrFrameNotFound
	}
	old := AnchorSetFramePayload{
		FrameID: p.FrameID,
		AnchorX: ds.Frames[idx].AnchorX,
		AnchorY: ds.Frames[idx].AnchorY,
		Space:   ds.Frames[idx].AnchorSpace,
	}
	ds.Frames[idx].AnchorX = p.AnchorX
	ds.Frames[idx].AnchorY = p.AnchorY
	ds.Frames[idx].AnchorSpace = p.Space
	return old, nil
}

func (ds *draftState) applyBatchOffsetAnchors(p AnchorBatchOffsetPayload) (AnchorBatchOffsetPayload, error) {
	for _, frameID := range p.FrameIDs {
		idx := ds.findFrameIndex(frameID)
		if idx == -1 {
			return AnchorBatchOffsetPayload{}, ErrFrameNotFound
		}
	}
	for _, frameID := range p.FrameIDs {
		idx := ds.findFrameIndex(frameID)
		ds.Frames[idx].AnchorX += p.DeltaX
		ds.Frames[idx].AnchorY += p.DeltaY
	}
	return AnchorBatchOffsetPayload{FrameIDs: p.FrameIDs, DeltaX: -p.DeltaX, DeltaY: -p.DeltaY}, nil
}

func (ds *draftState) applyResetAnchors(p AnchorResetPayload) ([]map[string]any, error) {
	frameIDs := p.FrameIDs
	if len(frameIDs) == 0 {
		frameIDs = make([]string, 0, len(ds.Frames))
		for _, f := range ds.Frames {
			frameIDs = append(frameIDs, f.FrameID)
		}
	}
	oldAnchors := make([]map[string]any, 0, len(frameIDs))
	for _, frameID := range frameIDs {
		idx := ds.findFrameIndex(frameID)
		if idx == -1 {
			continue
		}
		oldAnchors = append(oldAnchors, map[string]any{
			"frameId": frameID,
			"anchorX": ds.Frames[idx].AnchorX,
			"anchorY": ds.Frames[idx].AnchorY,
			"space":   ds.Frames[idx].AnchorSpace,
		})
		ds.Frames[idx].AnchorX = DefaultAnchorX
		ds.Frames[idx].AnchorY = DefaultAnchorY
		ds.Frames[idx].AnchorSpace = AnchorSpaceNormalizedCanvas
	}
	return oldAnchors, nil
}

func (ds *draftState) applyOp(opType string, payloadJSON string) (string, error) {
	switch opType {
	case OpFrameReorder:
		var p FrameReorderPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyReorder(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameDelete:
		var p FrameDeletePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyDelete(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameRestore:
		var p FrameRestorePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyRestore(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameDuplicate:
		var p FrameDuplicatePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyDuplicate(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameInsertAsset:
		var p FrameInsertAssetPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyInsertAsset(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameReplaceAsset:
		var p FrameReplaceAssetPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyReplaceAsset(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameSetDuration:
		var p FrameSetDurationPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetDuration(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpFrameBatchSetDuration:
		var p FrameBatchSetDurationPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyBatchSetDuration(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpActionSetDefaultFPS:
		var p ActionSetDefaultFPSPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetDefaultFPS(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpActionSetLoopType:
		var p ActionSetLoopTypePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetLoopType(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpActionSetReturnAction:
		var p ActionSetReturnActionPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetReturnAction(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpActionSetInterruptible:
		var p ActionSetInterruptiblePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetInterruptible(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpActionSetPriority:
		var p ActionSetPriorityOverridePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetPriority(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpActionSetCooldown:
		var p ActionSetCooldownOverridePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetCooldown(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpAnchorSetFrame:
		var p AnchorSetFramePayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applySetFrameAnchor(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpAnchorBatchOffset:
		var p AnchorBatchOffsetPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyBatchOffsetAnchors(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	case OpAnchorReset:
		var p AnchorResetPayload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return "", err
		}
		inv, err := ds.applyResetAnchors(p)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(inv)
		return string(b), nil
	default:
		return "", NewEditError(ErrCodeEditOperationInvalid, fmt.Sprintf("未知操作类型: %s", opType))
	}
}

func (s *service) rebuildDraftState(ctx context.Context, session *EditSession) (*draftState, error) {
	baseRev, err := s.repo.GetActionRevision(session.BaseRevisionID)
	if err != nil {
		return nil, err
	}
	baseFrames, err := s.repo.ListRevisionFrames(session.BaseRevisionID)
	if err != nil {
		return nil, err
	}
	ds := &draftState{
		Frames:             make([]draftFrame, 0, len(baseFrames)),
		DeletedFrames:      make(map[string]draftFrame),
		DefaultFPS:         baseRev.DefaultFPS,
		LoopType:           baseRev.LoopType,
		ReturnAction:       baseRev.ReturnAction,
		Interruptible:      baseRev.Interruptible != 0,
		PriorityOverride:   baseRev.PriorityOverride,
		CooldownMSOverride: baseRev.CooldownMSOverride,
	}
	for _, f := range baseFrames {
		ds.Frames = append(ds.Frames, draftFrame{
			FrameID:          f.FrameID,
			AssetID:          f.AssetID,
			LogicalIndex:     f.LogicalIndex,
			DurationMS:       f.DurationMS,
			AnchorX:          f.AnchorX,
			AnchorY:          f.AnchorY,
			AnchorSpace:      f.AnchorSpace,
			SourceFrameID:    f.SourceFrameID,
			SourceRevisionID: f.SourceRevisionID,
			SourceAttemptID:  f.SourceAttemptID,
			LineageType:      LineageOriginal,
			MaskAssetID:      f.MaskAssetID,
			TransformJSON:    f.TransformJSON,
			MetadataJSON:     f.MetadataJSON,
		})
	}
	ops, err := s.repo.ListOperations(session.ID)
	if err != nil {
		return nil, err
	}
	for _, op := range ops {
		if op.Status != OperationStatusApplied {
			continue
		}
		if _, err := ds.applyOp(op.OperationType, op.PayloadJSON); err != nil {
			return nil, err
		}
	}
	return ds, nil
}

func (s *service) buildManifestFromDraft(revID, parentRevID, processingTaskID, actionKey string, ds *draftState) (*RevisionManifest, error) {
	manifest := &RevisionManifest{
		SchemaVersion:    ManifestSchemaVersion,
		RevisionID:       revID,
		ParentRevisionID: parentRevID,
		ProcessingTaskID: processingTaskID,
		ActionKey:        actionKey,
		Playback: ManifestPlayback{
			LoopType:      ds.LoopType,
			DefaultFPS:    ds.DefaultFPS,
			ReturnAction:  ds.ReturnAction,
			Interruptible: ds.Interruptible,
		},
		Frames:    make([]ManifestFrame, 0, len(ds.Frames)),
		Quality:   ManifestQuality{},
		CreatedAt: nowUTC(),
	}
	assetCache := make(map[string]*FrameAsset)
	for _, f := range ds.Frames {
		var contentHash string
		if asset, ok := assetCache[f.AssetID]; ok {
			contentHash = asset.ContentHash
		} else {
			asset, err := s.repo.GetFrameAsset(f.AssetID)
			if err == nil && asset != nil {
				contentHash = asset.ContentHash
				assetCache[f.AssetID] = asset
			}
		}
		manifest.Frames = append(manifest.Frames, ManifestFrame{
			FrameID:      f.FrameID,
			LogicalIndex: f.LogicalIndex,
			AssetID:      f.AssetID,
			ContentHash:  contentHash,
			DurationMS:   f.DurationMS,
			Anchor: ManifestAnchor{
				X:     f.AnchorX,
				Y:     f.AnchorY,
				Space: f.AnchorSpace,
			},
			Lineage: ManifestLineage{
				Type:             f.LineageType,
				SourceRevisionID: f.SourceRevisionID,
				SourceAttemptID:  f.SourceAttemptID,
				SourceFrameID:    f.SourceFrameID,
			},
		})
	}
	return manifest, nil
}

func (s *service) totalDuration(ds *draftState) int {
	total := 0
	for _, f := range ds.Frames {
		total += f.DurationMS
	}
	return total
}

func (s *service) validateSession(session *EditSession) error {
	if session == nil {
		return ErrSessionNotFound
	}
	if session.Status == SessionStatusCommitted || session.Status == SessionStatusAbandoned {
		return ErrSessionAlreadyCommitted
	}
	if session.Status == SessionStatusExpired {
		return ErrSessionExpired
	}
	if session.ExpiresAt != "" {
		now := nowUTC()
		if session.ExpiresAt < now {
			return ErrSessionExpired
		}
	}
	return nil
}

func (s *service) applySessionOperation(ctx context.Context, sessionID, userID, opType string, payload any) error {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return err
	}
	if err := s.validateSession(session); err != nil {
		return err
	}
	baseVersion := session.SessionVersion
	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		return err
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	payloadJSON := string(payloadBytes)
	inverseJSON, err := ds.applyOp(opType, payloadJSON)
	if err != nil {
		return err
	}
	if session.Cursor < session.LastOperationSeq {
		supersededOps, err := s.repo.ListOperationsSince(sessionID, session.Cursor)
		if err != nil {
			return err
		}
		for _, op := range supersededOps {
			if op.Status == OperationStatusReverted {
				s.repo.UpdateOperationStatus(op.ID, OperationStatusSuperseded)
			}
		}
	}
	newSeq := session.LastOperationSeq + 1
	now := nowUTC()
	op := &EditOperation{
		ID:            generateID("op"),
		SessionID:     sessionID,
		Sequence:      newSeq,
		OperationType: opType,
		PayloadJSON:   payloadJSON,
		InverseJSON:   inverseJSON,
		BaseVersion:   baseVersion,
		ResultVersion: baseVersion + 1,
		Status:        OperationStatusApplied,
		CreatedBy:     userID,
		CreatedAt:     now,
	}
	if err := s.repo.CreateOperation(op); err != nil {
		return err
	}
	if err := s.repo.UpdateSessionCursor(sessionID, newSeq, newSeq); err != nil {
		return err
	}
	_, err = s.repo.UpdateSessionVersion(sessionID, baseVersion)
	return err
}

func (s *service) CheckProcessingTaskOwnership(ctx context.Context, processingTaskID, userID string) error {
	var genTaskID string
	err := s.db.Table("desktop_pet_processing_tasks").
		Where("id = ?", processingTaskID).
		Select("generation_task_id").
		Row().Scan(&genTaskID)
	if err != nil {
		return ErrTaskNotFound
	}
	var ownerUserID string
	err = s.db.Table("desktop_pet_generation_tasks").
		Where("id = ?", genTaskID).
		Select("user_id").
		Row().Scan(&ownerUserID)
	if err != nil {
		return ErrTaskNotFound
	}
	if ownerUserID != userID {
		return ErrPermissionDenied
	}
	return nil
}

func (s *service) ListRevisions(ctx context.Context, processingTaskID, actionKey string) ([]RevisionSummary, error) {
	revs, err := s.repo.ListActionRevisions(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	activeRevisionID := ""

	oldBinding, err := s.repo.GetActiveRevisionBinding(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if oldBinding != nil {
		activeRevisionID = oldBinding.RevisionID
	}

	if activeRevisionID == "" {
		newBinding, err := s.repo.GetActiveActionRevisionBindingByTask(processingTaskID, actionKey)
		if err != nil {
			return nil, err
		}
		if newBinding != nil {
			activeRevisionID = newBinding.ActiveActionRevisionID
		}
	}

	summaries := make([]RevisionSummary, 0, len(revs))
	for _, rev := range revs {
		summaries = append(summaries, RevisionSummary{
			ID:               rev.ID,
			RevisionNumber:   rev.RevisionNumber,
			RevisionType:     rev.RevisionType,
			Status:           rev.Status,
			FrameCount:       rev.FrameCount,
			DurationMS:       rev.DurationMS,
			DefaultFPS:       rev.DefaultFPS,
			LoopType:         rev.LoopType,
			QualityVerdict:   rev.QualityVerdict,
			ChangeSummary:    rev.ChangeSummary,
			ParentRevisionID: rev.ParentRevisionID,
			IsActive:         rev.ID == activeRevisionID,
			CreatedAt:        rev.CreatedAt,
		})
	}
	return summaries, nil
}

func (s *service) GetRevision(ctx context.Context, revisionID string) (*RevisionDetail, error) {
	rev, err := s.repo.GetActionRevision(revisionID)
	if err != nil {
		return nil, err
	}
	frames, err := s.repo.ListRevisionFrames(revisionID)
	if err != nil {
		return nil, err
	}
	assetMap := make(map[string]*FrameAsset)
	for _, f := range frames {
		if _, ok := assetMap[f.AssetID]; ok {
			continue
		}
		asset, err := s.repo.GetFrameAsset(f.AssetID)
		if err != nil {
			continue
		}
		assetMap[f.AssetID] = asset
	}
	assets := make([]FrameAsset, 0, len(assetMap))
	for _, a := range assetMap {
		assets = append(assets, *a)
	}
	var manifest *RevisionManifest
	if rev.ManifestPath != "" {
		m, err := s.assetStore.ReadManifest(revisionID)
		if err == nil {
			manifest = m
		}
	}
	return &RevisionDetail{
		Revision: *rev,
		Frames:   frames,
		Assets:   assets,
		Manifest: manifest,
	}, nil
}

func (s *service) GetActiveRevision(ctx context.Context, processingTaskID, actionKey string) (*RevisionDetail, error) {
	binding, err := s.repo.GetActiveRevisionBinding(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if binding != nil {
		return s.GetRevision(ctx, binding.RevisionID)
	}

	newBinding, err := s.repo.GetActiveActionRevisionBindingByTask(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if newBinding != nil {
		return s.GetRevision(ctx, newBinding.ActiveActionRevisionID)
	}

	return nil, ErrRevisionNotFound
}

func (s *service) ActivateRevision(ctx context.Context, processingTaskID, actionKey, revisionID string, expectedVersion int64, reason, userID string) error {
	rev, err := s.repo.GetActionRevision(revisionID)
	if err != nil {
		return err
	}
	if rev.Status != RevisionStatusReady && rev.Status != RevisionStatusQualityReady {
		return ErrRevisionNotReady
	}
	if rev.ProcessingTaskID != processingTaskID || rev.ActionKey != actionKey {
		return ErrRevisionNotFound
	}
	now := nowUTC()
	existing, err := s.repo.GetActiveRevisionBinding(processingTaskID, actionKey)
	if err != nil {
		return err
	}
	if existing == nil {
		if expectedVersion != 0 {
			return ErrActiveBindingConflict
		}
		binding := &ActiveRevisionBinding{
			ProcessingTaskID: processingTaskID,
			ActionKey:        actionKey,
			RevisionID:       revisionID,
			BindingVersion:   1,
			ActivatedBy:      userID,
			Reason:           reason,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := s.db.Create(binding).Error; err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "duplicate key") {
				return ErrActiveBindingConflict
			}
			return err
		}
		return nil
	}
	if existing.BindingVersion != expectedVersion {
		return ErrActiveBindingConflict
	}
	result := s.db.Model(&ActiveRevisionBinding{}).
		Where("processing_task_id = ? AND action_key = ? AND binding_version = ?", processingTaskID, actionKey, expectedVersion).
		Updates(map[string]any{
			"revision_id":     revisionID,
			"binding_version": expectedVersion + 1,
			"activated_by":    userID,
			"reason":          reason,
			"updated_at":      now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrActiveBindingConflict
	}
	return nil
}

func (s *service) GetPreviewManifest(ctx context.Context, revisionID string) (*RevisionManifest, error) {
	rev, err := s.repo.GetActionRevision(revisionID)
	if err != nil {
		return nil, err
	}
	if rev.ManifestPath == "" {
		return nil, ErrRevisionNotReady
	}
	return s.assetStore.ReadManifest(revisionID)
}

func (s *service) GetFrameImage(ctx context.Context, revisionID, frameID string) (string, string, error) {
	frames, err := s.repo.ListRevisionFrames(revisionID)
	if err != nil {
		return "", "", err
	}
	var assetID string
	for _, f := range frames {
		if f.FrameID == frameID {
			assetID = f.AssetID
			break
		}
	}
	if assetID == "" {
		return "", "", ErrFrameNotFound
	}
	asset, err := s.repo.GetFrameAsset(assetID)
	if err != nil {
		return "", "", err
	}
	path, err := s.assetStore.GetAssetPath(assetID)
	if err != nil {
		return "", "", err
	}
	return path, asset.MimeType, nil
}

func (s *service) GetFrameThumbnail(ctx context.Context, revisionID, frameID string) (string, string, error) {
	return s.GetFrameImage(ctx, revisionID, frameID)
}

func (s *service) GetActionEditSummary(ctx context.Context, processingTaskID, actionKey string) (*ActionEditSummary, error) {
	binding, err := s.repo.GetActiveRevisionBinding(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrRevisionNotFound
	}
	rev, err := s.repo.GetActionRevision(binding.RevisionID)
	if err != nil {
		return nil, err
	}
	frames, err := s.repo.ListRevisionFrames(binding.RevisionID)
	if err != nil {
		return nil, err
	}
	assetMap := make(map[string]*FrameAsset)
	for _, f := range frames {
		if _, ok := assetMap[f.AssetID]; ok {
			continue
		}
		asset, err := s.repo.GetFrameAsset(f.AssetID)
		if err == nil {
			assetMap[f.AssetID] = asset
		}
	}
	findingFrameSet := make(map[string]bool)
	findings, ferr := s.qualAdapter.GetFindings(ctx, binding.RevisionID)
	if ferr == nil {
		for _, fnd := range findings {
			if fnd.FrameID != "" && !fnd.Stale {
				findingFrameSet[fnd.FrameID] = true
			}
		}
	}
	timeline := make([]FrameTimelineItem, 0, len(frames))
	for _, f := range frames {
		item := FrameTimelineItem{
			FrameID:         f.FrameID,
			LogicalIndex:    f.LogicalIndex,
			AssetID:         f.AssetID,
			DurationMS:      f.DurationMS,
			AnchorX:         f.AnchorX,
			AnchorY:         f.AnchorY,
			HasQualityIssue: findingFrameSet[f.FrameID],
		}
		if asset, ok := assetMap[f.AssetID]; ok {
			item.ContentHash = asset.ContentHash
			item.SourceType = asset.SourceType
			item.Width = asset.Width
			item.Height = asset.Height
		}
		timeline = append(timeline, item)
	}
	openSessions, err := s.repo.ListSessionsByTaskAction(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	allRevs, err := s.repo.ListActionRevisions(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	return &ActionEditSummary{
		ActionKey:         actionKey,
		ActiveRevisionID:  binding.RevisionID,
		ActiveRevisionNum: rev.RevisionNumber,
		BindingVersion:    binding.BindingVersion,
		FrameCount:        rev.FrameCount,
		DurationMS:        rev.DurationMS,
		QualityVerdict:    rev.QualityVerdict,
		HasOpenSession:    len(openSessions) > 0,
		RevisionCount:     len(allRevs),
		Timeline:          timeline,
	}, nil
}

func (s *service) CreateSession(ctx context.Context, processingTaskID, actionKey, userID string, req CreateSessionRequest) (*CreateSessionResponse, error) {
	baseRev, err := s.repo.GetActionRevision(req.BaseRevisionID)
	if err != nil {
		return nil, err
	}
	if baseRev.ProcessingTaskID != processingTaskID || baseRev.ActionKey != actionKey {
		return nil, ErrRevisionNotFound
	}
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetIdempotencyRecord(userID, "session_create", req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.Status == "completed" {
			var result CreateSessionResponse
			if err := json.Unmarshal([]byte(existing.ResultJSON), &result); err == nil {
				return &result, nil
			}
		}
	}
	now := nowUTC()
	expiresAt := time.Now().UTC().Add(SessionDefaultTTLHours * time.Hour).Format(time.RFC3339)
	sessionID := generateID("sess")
	session := &EditSession{
		ID:               sessionID,
		UserID:           userID,
		ProcessingTaskID: processingTaskID,
		ActionKey:        actionKey,
		BaseRevisionID:   req.BaseRevisionID,
		SessionVersion:   1,
		Status:           SessionStatusOpen,
		Cursor:           0,
		LastOperationSeq: 0,
		ClientInstanceID: req.ClientInstanceID,
		ExpiresAt:        expiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateEditSession(session); err != nil {
		return nil, err
	}
	resp := &CreateSessionResponse{
		SessionID:      sessionID,
		SessionVersion: 1,
		BaseRevisionID: req.BaseRevisionID,
	}
	if req.IdempotencyKey != "" {
		resultJSON, _ := json.Marshal(resp)
		record := &EditIdempotencyRecord{
			ID:             generateID("idem"),
			UserID:         userID,
			SessionID:      "session_create",
			IdempotencyKey: req.IdempotencyKey,
			Endpoint:       "create_session",
			ResultJSON:     string(resultJSON),
			Status:         "completed",
			CreatedAt:      now,
		}
		s.repo.CreateIdempotencyRecord(record)
	}
	return resp, nil
}

func (s *service) GetSession(ctx context.Context, sessionID string) (*EditSession, error) {
	return s.repo.GetEditSession(sessionID)
}

func (s *service) ApplyOperation(ctx context.Context, sessionID, userID string, req ApplyOperationRequest) (*ApplyOperationResponse, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetOperationByIdempotencyKey(sessionID, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return &ApplyOperationResponse{
				SessionVersion: existing.ResultVersion,
				Sequence:       existing.Sequence,
				Status:         existing.Status,
			}, nil
		}
	}
	if session.SessionVersion != req.BaseSessionVersion {
		return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
			CurrentVersion:  session.SessionVersion,
			ExpectedVersion: req.BaseSessionVersion,
		})
	}
	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		return nil, err
	}
	payloadBytes, err := json.Marshal(req.Operation.Payload)
	if err != nil {
		return nil, err
	}
	payloadJSON := string(payloadBytes)
	inverseJSON, err := ds.applyOp(req.Operation.Type, payloadJSON)
	if err != nil {
		return nil, err
	}
	if session.Cursor < session.LastOperationSeq {
		supersededOps, err := s.repo.ListOperationsSince(sessionID, session.Cursor)
		if err != nil {
			return nil, err
		}
		for _, op := range supersededOps {
			if op.Status == OperationStatusReverted {
				s.repo.UpdateOperationStatus(op.ID, OperationStatusSuperseded)
			}
		}
	}
	newSeq := session.LastOperationSeq + 1
	now := nowUTC()
	op := &EditOperation{
		ID:             generateID("op"),
		SessionID:      sessionID,
		Sequence:       newSeq,
		OperationType:  req.Operation.Type,
		PayloadJSON:    payloadJSON,
		InverseJSON:    inverseJSON,
		IdempotencyKey: req.IdempotencyKey,
		BaseVersion:    req.BaseSessionVersion,
		ResultVersion:  req.BaseSessionVersion + 1,
		Status:         OperationStatusApplied,
		CreatedBy:      userID,
		CreatedAt:      now,
	}
	if err := s.repo.CreateOperation(op); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSessionCursor(sessionID, newSeq, newSeq); err != nil {
		return nil, err
	}
	newVersion, err := s.repo.UpdateSessionVersion(sessionID, req.BaseSessionVersion)
	if err != nil {
		if editErr, ok := err.(*EditError); ok && editErr.Code == ErrCodeEditSessionNotFound {
			return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
				CurrentVersion:  newVersion,
				ExpectedVersion: req.BaseSessionVersion,
			})
		}
		return nil, err
	}
	return &ApplyOperationResponse{
		SessionVersion: newVersion,
		Sequence:       newSeq,
		Status:         OperationStatusApplied,
	}, nil
}

func (s *service) Undo(ctx context.Context, sessionID, userID string, baseVersion int64) (*ApplyOperationResponse, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	if session.SessionVersion != baseVersion {
		return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
			CurrentVersion:  session.SessionVersion,
			ExpectedVersion: baseVersion,
		})
	}
	if session.Cursor == 0 {
		return nil, NewEditError(ErrCodeEditOperationInvalid, "没有可撤销的操作")
	}
	targetSeq := session.Cursor
	targetOp, err := s.repo.GetOperation(sessionID, targetSeq)
	if err != nil {
		return nil, err
	}
	for targetOp == nil || targetOp.Status != OperationStatusApplied {
		targetSeq--
		if targetSeq <= 0 {
			return nil, NewEditError(ErrCodeEditOperationInvalid, "没有可撤销的操作")
		}
		targetOp, err = s.repo.GetOperation(sessionID, targetSeq)
		if err != nil {
			return nil, err
		}
	}
	if err := s.repo.UpdateOperationStatus(targetOp.ID, OperationStatusReverted); err != nil {
		return nil, err
	}
	newCursor := 0
	for seq := targetSeq - 1; seq > 0; seq-- {
		prevOp, err := s.repo.GetOperation(sessionID, seq)
		if err != nil {
			break
		}
		if prevOp != nil && prevOp.Status == OperationStatusApplied {
			newCursor = seq
			break
		}
	}
	if err := s.repo.UpdateSessionCursor(sessionID, newCursor, session.LastOperationSeq); err != nil {
		return nil, err
	}
	newVersion, err := s.repo.UpdateSessionVersion(sessionID, baseVersion)
	if err != nil {
		if editErr, ok := err.(*EditError); ok && editErr.Code == ErrCodeEditSessionNotFound {
			return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
				CurrentVersion:  newVersion,
				ExpectedVersion: baseVersion,
			})
		}
		return nil, err
	}
	return &ApplyOperationResponse{
		SessionVersion: newVersion,
		Sequence:       targetOp.Sequence,
		Status:         OperationStatusReverted,
	}, nil
}

func (s *service) Redo(ctx context.Context, sessionID, userID string, baseVersion int64) (*ApplyOperationResponse, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	if session.SessionVersion != baseVersion {
		return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
			CurrentVersion:  session.SessionVersion,
			ExpectedVersion: baseVersion,
		})
	}
	var redoOp *EditOperation
	for seq := session.Cursor + 1; seq <= session.LastOperationSeq; seq++ {
		op, err := s.repo.GetOperation(sessionID, seq)
		if err != nil {
			break
		}
		if op != nil && op.Status == OperationStatusReverted {
			redoOp = op
			break
		}
	}
	if redoOp == nil {
		return nil, NewEditError(ErrCodeEditOperationInvalid, "没有可重做的操作")
	}
	if err := s.repo.UpdateOperationStatus(redoOp.ID, OperationStatusApplied); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSessionCursor(sessionID, redoOp.Sequence, session.LastOperationSeq); err != nil {
		return nil, err
	}
	newVersion, err := s.repo.UpdateSessionVersion(sessionID, baseVersion)
	if err != nil {
		if editErr, ok := err.(*EditError); ok && editErr.Code == ErrCodeEditSessionNotFound {
			return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
				CurrentVersion:  newVersion,
				ExpectedVersion: baseVersion,
			})
		}
		return nil, err
	}
	return &ApplyOperationResponse{
		SessionVersion: newVersion,
		Sequence:       redoOp.Sequence,
		Status:         OperationStatusApplied,
	}, nil
}

func (s *service) CreateCheckpoint(ctx context.Context, sessionID string) error {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return err
	}
	if err := s.validateSession(session); err != nil {
		return err
	}
	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		return err
	}
	manifest, err := s.buildManifestFromDraft(session.ID, session.BaseRevisionID, session.ProcessingTaskID, session.ActionKey, ds)
	if err != nil {
		return err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	checkpoint := &EditCheckpoint{
		ID:           generateID("ckpt"),
		SessionID:    sessionID,
		Sequence:     session.Cursor,
		ManifestJSON: string(manifestJSON),
		ManifestHash: s.assetStore.ComputeHash(manifestJSON),
		FrameCount:   len(ds.Frames),
		CreatedAt:    nowUTC(),
	}
	if err := s.repo.CreateCheckpoint(checkpoint); err != nil {
		return err
	}
	if err := s.repo.UpdateSessionCheckpoint(sessionID, checkpoint.ID); err != nil {
		return err
	}
	return s.repo.DeleteOldCheckpoints(sessionID, MaxCheckpointKeep)
}

func (s *service) CommitSession(ctx context.Context, sessionID, userID string, req CommitSessionRequest) (*CommitSessionResponse, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	if session.SessionVersion != req.ExpectedSessionVersion {
		return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
			CurrentVersion:  session.SessionVersion,
			ExpectedVersion: req.ExpectedSessionVersion,
		})
	}
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetIdempotencyRecord(userID, sessionID, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.Status == "completed" {
			var result CommitSessionResponse
			if err := json.Unmarshal([]byte(existing.ResultJSON), &result); err == nil {
				return &result, nil
			}
		}
	}
	if err := s.repo.UpdateSessionStatus(sessionID, SessionStatusCommitting); err != nil {
		return nil, err
	}
	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	if len(ds.Frames) < MinFrameCount {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, ErrFrameCountInvalid
	}
	baseRev, err := s.repo.GetActionRevision(session.BaseRevisionID)
	if err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	revNum, err := s.repo.GetNextRevisionNumber(session.ProcessingTaskID, session.ActionKey)
	if err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	rootRevID := baseRev.RootRevisionID
	if rootRevID == "" {
		rootRevID = baseRev.ID
	}
	now := nowUTC()
	revID := generateID("rev")
	interruptible := 0
	if ds.Interruptible {
		interruptible = 1
	}
	newRev := &ActionRevision{
		ID:                   revID,
		ProcessingTaskID:     session.ProcessingTaskID,
		GenerationTaskID:     baseRev.GenerationTaskID,
		ActionKey:            session.ActionKey,
		ParentRevisionID:     session.BaseRevisionID,
		RootRevisionID:       rootRevID,
		RevisionNumber:       revNum,
		RevisionType:         RevisionTypeEdit,
		Status:               RevisionStatusBuilding,
		FrameCount:           len(ds.Frames),
		DurationMS:           s.totalDuration(ds),
		DefaultFPS:           ds.DefaultFPS,
		LoopType:             ds.LoopType,
		ReturnAction:         ds.ReturnAction,
		Interruptible:        interruptible,
		PriorityOverride:     ds.PriorityOverride,
		CooldownMSOverride:   ds.CooldownMSOverride,
		CreatedByUserID:      userID,
		CreatedFromSessionID: sessionID,
		ChangeSummary:        req.ChangeSummary,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.repo.CreateActionRevision(newRev); err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	manifest, err := s.buildManifestFromDraft(revID, session.BaseRevisionID, session.ProcessingTaskID, session.ActionKey, ds)
	if err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	manifestPath, manifestHash, err := s.assetStore.WriteManifest(revID, manifest)
	if err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	revFrames := make([]ActionRevisionFrame, 0, len(ds.Frames))
	for _, f := range ds.Frames {
		revFrames = append(revFrames, ActionRevisionFrame{
			ID:                generateID("rf"),
			RevisionID:        revID,
			FrameID:           f.FrameID,
			AssetID:           f.AssetID,
			LogicalIndex:      f.LogicalIndex,
			DurationMS:        f.DurationMS,
			SourceFrameID:     f.SourceFrameID,
			SourceRevisionID:  f.SourceRevisionID,
			SourceAttemptID:   f.SourceAttemptID,
			AnchorX:           f.AnchorX,
			AnchorY:           f.AnchorY,
			AnchorSpace:       f.AnchorSpace,
			MaskAssetID:       f.MaskAssetID,
			TransformJSON:     f.TransformJSON,
			MetadataJSON:      f.MetadataJSON,
			CopiedFromFrameID: f.CopiedFromFrameID,
			CreatedAt:         now,
		})
	}
	if err := s.repo.CreateRevisionFrames(revFrames); err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	if err := s.repo.UpdateActionRevisionManifest(revID, manifestPath, manifestHash, len(ds.Frames), s.totalDuration(ds)); err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	journal := &PublishJournal{
		ID:           generateID("jrn"),
		RevisionID:   revID,
		SessionID:    sessionID,
		Action:       JournalActionPublish,
		Status:       JournalStatusCompleted,
		ManifestPath: manifestPath,
		CreatedAt:    now,
		CompletedAt:  now,
	}
	if err := s.repo.CreatePublishJournal(journal); err != nil {
		s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		return nil, err
	}
	if err := s.repo.UpdateSessionCommitted(sessionID, revID); err != nil {
		return nil, err
	}
	var qualityJobID string
	switch req.ActivationPolicy {
	case ActivationPolicyAfterQualityPass:
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusQualityPending)
		evalID, qErr := s.qualAdapter.EvaluateRevision(ctx, revID)
		if qErr == nil {
			qualityJobID = evalID
		}
	case ActivationPolicyManual:
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusReady)
	case ActivationPolicyKeepCurrent:
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusReady)
	default:
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusReady)
	}
	resp := &CommitSessionResponse{
		RevisionID:   revID,
		QualityJobID: qualityJobID,
		Status:       SessionStatusCommitted,
	}
	if req.IdempotencyKey != "" {
		resultJSON, _ := json.Marshal(resp)
		record := &EditIdempotencyRecord{
			ID:             generateID("idem"),
			UserID:         userID,
			SessionID:      sessionID,
			IdempotencyKey: req.IdempotencyKey,
			Endpoint:       "commit_session",
			ResultJSON:     string(resultJSON),
			Status:         "completed",
			CreatedAt:      now,
		}
		s.repo.CreateIdempotencyRecord(record)
	}
	return resp, nil
}

func (s *service) AbandonSession(ctx context.Context, sessionID, userID string) error {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	if session.Status == SessionStatusCommitted {
		return ErrSessionAlreadyCommitted
	}
	return s.repo.UpdateSessionStatus(sessionID, SessionStatusAbandoned)
}

func (s *service) GetSessionEvents(ctx context.Context, sessionID string) ([]SessionEvent, error) {
	ops, err := s.repo.ListOperations(sessionID)
	if err != nil {
		return nil, err
	}
	events := make([]SessionEvent, 0, len(ops))
	for _, op := range ops {
		eventType := "operation.applied"
		if op.Status == OperationStatusReverted {
			eventType = "operation.reverted"
		} else if op.Status == OperationStatusSuperseded {
			eventType = "operation.superseded"
		} else if op.Status == OperationStatusFailed {
			eventType = "operation.failed"
		}
		events = append(events, SessionEvent{
			EventType: eventType,
			SessionID: sessionID,
			Data: map[string]any{
				"sequence":      op.Sequence,
				"operationType": op.OperationType,
				"status":        op.Status,
				"createdBy":     op.CreatedBy,
			},
			Timestamp: op.CreatedAt,
		})
	}
	checkpoints, err := s.repo.ListCheckpoints(sessionID)
	if err != nil {
		return events, nil
	}
	for _, cp := range checkpoints {
		events = append(events, SessionEvent{
			EventType: "checkpoint.created",
			SessionID: sessionID,
			Data: map[string]any{
				"checkpointId": cp.ID,
				"sequence":     cp.Sequence,
				"frameCount":   cp.FrameCount,
			},
			Timestamp: cp.CreatedAt,
		})
	}
	return events, nil
}

func (s *service) CreateRegenerationJob(ctx context.Context, sessionID, userID string, req CreateRegenerationJobRequest) (*CreateRegenerationJobResponse, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetJobByIdempotencyKey(sessionID, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return &CreateRegenerationJobResponse{
				JobID:        existing.ID,
				Status:       existing.Status,
				CostEstimate: existing.CostEstimateJSON,
			}, nil
		}
	}
	now := nowUTC()
	jobID := generateID("regen")
	job := &RegenerationJob{
		ID:               jobID,
		SessionID:        sessionID,
		ProcessingTaskID: session.ProcessingTaskID,
		ActionKey:        session.ActionKey,
		TargetFrameID:    req.TargetFrameID,
		JobType:          req.JobType,
		Status:           JobStatusCreated,
		IdempotencyKey:   req.IdempotencyKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateRegenerationJob(job); err != nil {
		return nil, err
	}
	return &CreateRegenerationJobResponse{
		JobID:  jobID,
		Status: JobStatusCreated,
	}, nil
}

func (s *service) GetRegenerationJob(ctx context.Context, jobID string) (*RegenerationJob, error) {
	return s.repo.GetRegenerationJob(jobID)
}

func (s *service) ListRegenerationJobs(ctx context.Context, userID string, limit, offset int) ([]RegenerationJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var jobs []RegenerationJob
	if err := s.repo.DB().Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *service) CancelRegenerationJob(ctx context.Context, jobID, userID string) error {
	job, err := s.repo.GetRegenerationJob(jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return ErrCandidateNotFound
	}
	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
		return ErrJobNotCancelable
	}
	return s.repo.UpdateJobStatus(jobID, JobStatusCancelled)
}

func (s *service) AcceptCandidate(ctx context.Context, candidateID, userID string, req AcceptCandidateRequest) error {
	candidate, err := s.repo.GetCandidate(candidateID)
	if err != nil {
		return err
	}
	if candidate.Status != CandidateStatusPending {
		return ErrCandidateAlreadyDecided
	}
	if err := s.repo.UpdateCandidateStatus(candidateID, CandidateStatusAccepted, userID); err != nil {
		return err
	}
	return s.applySessionOperation(ctx, candidate.SessionID, userID, OpFrameReplaceAsset, FrameReplaceAssetPayload{
		FrameID:    candidate.TargetFrameID,
		AssetID:    candidate.AssetID,
		KeepAnchor: true,
	})
}

func (s *service) RejectCandidate(ctx context.Context, candidateID, userID string, req RejectCandidateRequest) error {
	candidate, err := s.repo.GetCandidate(candidateID)
	if err != nil {
		return err
	}
	if candidate.Status != CandidateStatusPending {
		return ErrCandidateAlreadyDecided
	}
	return s.repo.UpdateCandidateStatus(candidateID, CandidateStatusRejected, userID)
}

func (s *service) ListCandidates(ctx context.Context, sessionID string) ([]EditCandidate, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	candidates, err := s.repo.ListCandidatesBySession(sessionID)
	if err != nil {
		return nil, err
	}
	if candidates == nil {
		candidates = []EditCandidate{}
	}
	return candidates, nil
}

func (s *service) UploadCandidate(ctx context.Context, sessionID, userID string, data []byte, mimeType string, targetFrameID string) (*UploadCandidateResponse, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSession(session); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrUploadInvalid
	}
	asset, err := s.assetStore.WriteAsset(ctx, data, mimeType, AssetSourceUploaded, userID)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	candidateID := generateID("cand")
	candidate := &EditCandidate{
		ID:            candidateID,
		SessionID:     sessionID,
		TargetFrameID: targetFrameID,
		CandidateType: AssetSourceUploaded,
		AssetID:       asset.ID,
		Status:        CandidateStatusPending,
		CreatedAt:     now,
	}
	if err := s.repo.CreateCandidate(candidate); err != nil {
		return nil, err
	}
	return &UploadCandidateResponse{
		CandidateID: candidateID,
		AssetID:     asset.ID,
		Status:      CandidateStatusPending,
	}, nil
}

func (s *service) ApplyBackgroundPatch(ctx context.Context, sessionID, frameID, userID string, req BackgroundApplyPatchPayload) error {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return err
	}
	if err := s.validateSession(session); err != nil {
		return err
	}
	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		return err
	}
	idx := ds.findFrameIndex(frameID)
	if idx == -1 {
		return ErrFrameNotFound
	}
	brushData := req.BrushData
	if len(brushData) == 0 && req.BrushDataBase64 != "" {
		brushData = []byte(req.BrushDataBase64)
	}
	maskPath, err := s.assetStore.WriteMaskData(ctx, sessionID, brushData)
	if err != nil {
		return err
	}
	sourceAsset, err := s.repo.GetFrameAsset(ds.Frames[idx].AssetID)
	if err != nil {
		return err
	}
	now := nowUTC()
	maskPatch := &MaskPatch{
		ID:               generateID("mask"),
		SessionID:        sessionID,
		FrameID:          frameID,
		SourceAssetHash:  sourceAsset.ContentHash,
		PatchType:        req.PatchType,
		BrushDataPath:    maskPath,
		BrushSize:        req.BrushSize,
		BrushHardness:    req.BrushHardness,
		BrushOpacity:     req.BrushOpacity,
		CoordinateSpace:  AnchorSpacePixel,
		CanvasWidth:      req.CanvasWidth,
		CanvasHeight:     req.CanvasHeight,
		AlgorithmVersion: "1",
		OperationSeq:     session.LastOperationSeq + 1,
		CreatedAt:        now,
	}
	if err := s.repo.CreateMaskPatch(maskPatch); err != nil {
		return err
	}
	return s.applySessionOperation(ctx, sessionID, userID, OpBackgroundApplyPatch, req)
}

func (s *service) ResetBackgroundPatch(ctx context.Context, sessionID, frameID, userID string) error {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return err
	}
	if err := s.validateSession(session); err != nil {
		return err
	}
	return s.applySessionOperation(ctx, sessionID, userID, OpBackgroundResetPatch, BackgroundResetPatchPayload{FrameID: frameID})
}

func (s *service) SetFrameAnchor(ctx context.Context, sessionID, userID string, req AnchorSetFramePayload) error {
	return s.applySessionOperation(ctx, sessionID, userID, OpAnchorSetFrame, req)
}

func (s *service) BatchOffsetAnchors(ctx context.Context, sessionID, userID string, req AnchorBatchOffsetPayload) error {
	return s.applySessionOperation(ctx, sessionID, userID, OpAnchorBatchOffset, req)
}

func (s *service) ResetAnchors(ctx context.Context, sessionID, userID string, req AnchorResetPayload) error {
	return s.applySessionOperation(ctx, sessionID, userID, OpAnchorReset, req)
}

func (s *service) TriggerQualityEvaluation(ctx context.Context, revisionID string) (string, error) {
	rev, err := s.repo.GetActionRevision(revisionID)
	if err != nil {
		return "", err
	}
	if rev.Status == RevisionStatusBuilding {
		s.repo.UpdateActionRevisionStatus(revisionID, RevisionStatusQualityPending)
	}
	evalID, err := s.qualAdapter.EvaluateRevision(ctx, revisionID)
	if err != nil {
		return "", err
	}
	return evalID, nil
}

func (s *service) GetLatestQualityEvaluation(ctx context.Context, revisionID string) (*QualityEvaluationInfo, error) {
	return s.qualAdapter.GetLatestEvaluation(ctx, revisionID)
}

func (s *service) RecoverPendingJournals(ctx context.Context) error {
	journals, err := s.repo.ListPendingJournals()
	if err != nil {
		return err
	}
	for _, journal := range journals {
		rev, err := s.repo.GetActionRevision(journal.RevisionID)
		if err != nil {
			s.repo.UpdateJournalStatus(journal.ID, JournalStatusFailed, fmt.Sprintf("revision not found: %v", err))
			continue
		}
		if rev.ManifestPath == "" {
			s.repo.UpdateJournalStatus(journal.ID, JournalStatusFailed, "manifest path is empty")
			continue
		}
		frames, err := s.repo.ListRevisionFrames(journal.RevisionID)
		if err != nil {
			s.repo.UpdateJournalStatus(journal.ID, JournalStatusFailed, fmt.Sprintf("failed to list frames: %v", err))
			continue
		}
		if len(frames) == 0 {
			s.repo.UpdateJournalStatus(journal.ID, JournalStatusFailed, "no revision frames found")
			continue
		}
		if rev.Status == RevisionStatusBuilding {
			s.repo.UpdateActionRevisionStatus(journal.RevisionID, RevisionStatusReady)
		}
		s.repo.UpdateJournalStatus(journal.ID, JournalStatusCompleted, "")
	}
	return nil
}

func (s *service) ExpireSessions(ctx context.Context) error {
	sessions, err := s.repo.ListExpiredSessions()
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err := s.repo.UpdateSessionStatus(session.ID, SessionStatusExpired); err != nil {
			continue
		}
	}
	return nil
}

func (s *service) ImportLegacyRevision(ctx context.Context, processingTaskID, actionKey, userID string) (*CommitSessionResponse, error) {
	existing, err := s.repo.GetActiveRevisionBinding(processingTaskID, actionKey)
	if err == nil && existing != nil {
		return nil, NewEditError(ErrCodeEditOperationInvalid, "该动作已有活跃 Revision，无需导入")
	}
	importData, err := s.procAdapter.ImportAsBaselineRevision(ctx, processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if len(importData.Frames) == 0 {
		return nil, NewEditError(ErrCodeEditFrameCountInvalid, "未找到可导入的处理帧")
	}
	now := nowUTC()
	revID := generateID("rev")
	fps := importData.FPS
	if fps <= 0 {
		fps = 12
	}
	frameDuration := importData.FrameDurationMS
	if frameDuration <= 0 {
		frameDuration = DefaultFrameDurationMS
	}
	loopType := importData.LoopType
	if loopType == "" {
		loopType = "loop"
	}
	totalDuration := frameDuration * len(importData.Frames)
	newRev := &ActionRevision{
		ID:                   revID,
		ProcessingTaskID:     processingTaskID,
		GenerationTaskID:     "",
		ActionKey:            actionKey,
		ParentRevisionID:     "",
		RootRevisionID:       revID,
		RevisionNumber:       1,
		RevisionType:         RevisionTypeLegacyImport,
		Status:               RevisionStatusBuilding,
		FrameCount:           len(importData.Frames),
		DurationMS:           totalDuration,
		DefaultFPS:           fps,
		LoopType:             loopType,
		ReturnAction:         "",
		Interruptible:        1,
		CreatedByUserID:      userID,
		CreatedFromSessionID: "",
		ChangeSummary:        "从处理结果导入为基线 Revision",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.repo.CreateActionRevision(newRev); err != nil {
		return nil, err
	}
	revFrames := make([]ActionRevisionFrame, 0, len(importData.Frames))
	ds := &draftState{
		Frames:        make([]draftFrame, 0, len(importData.Frames)),
		DeletedFrames: make(map[string]draftFrame),
		DefaultFPS:    fps,
		LoopType:      loopType,
		Interruptible: true,
	}
	for i, frameInfo := range importData.Frames {
		asset, assetErr := s.assetStore.WriteAsset(ctx, readFileBytes(filepath.Join(s.dataDir, frameInfo.ProcessedPath)), "image/png", AssetSourceLegacy, importData.ProcessingActionID)
		if assetErr != nil {
			s.repo.UpdateActionRevisionStatus(revID, RevisionStatusFailed)
			return nil, assetErr
		}
		frameID := fmt.Sprintf("frame-legacy-%d-%d", time.Now().UnixNano(), i)
		anchorX := frameInfo.AnchorX
		if anchorX == 0 {
			anchorX = DefaultAnchorX
		}
		anchorY := frameInfo.AnchorY
		if anchorY == 0 {
			anchorY = DefaultAnchorY
		}
		rf := ActionRevisionFrame{
			ID:               generateID("rf"),
			RevisionID:       revID,
			FrameID:          frameID,
			AssetID:          asset.ID,
			LogicalIndex:     i,
			DurationMS:       frameDuration,
			SourceFrameID:    "",
			SourceRevisionID: "",
			AnchorX:          anchorX,
			AnchorY:          anchorY,
			AnchorSpace:      AnchorSpaceNormalizedCanvas,
			CreatedAt:        now,
		}
		revFrames = append(revFrames, rf)
		ds.Frames = append(ds.Frames, draftFrame{
			FrameID:      frameID,
			AssetID:      asset.ID,
			LogicalIndex: i,
			DurationMS:   frameDuration,
			AnchorX:      anchorX,
			AnchorY:      anchorY,
			AnchorSpace:  AnchorSpaceNormalizedCanvas,
			LineageType:  LineageOriginal,
		})
	}
	if err := s.repo.CreateRevisionFrames(revFrames); err != nil {
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusFailed)
		return nil, err
	}
	manifest, err := s.buildManifestFromDraft(revID, "", processingTaskID, actionKey, ds)
	if err != nil {
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusFailed)
		return nil, err
	}
	manifestPath, manifestHash, err := s.assetStore.WriteManifest(revID, manifest)
	if err != nil {
		s.repo.UpdateActionRevisionStatus(revID, RevisionStatusFailed)
		return nil, err
	}
	if err := s.repo.UpdateActionRevisionManifest(revID, manifestPath, manifestHash, len(revFrames), totalDuration); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateActionRevisionStatus(revID, RevisionStatusReady); err != nil {
		return nil, err
	}
	journal := &PublishJournal{
		ID:           generateID("jrn"),
		RevisionID:   revID,
		Action:       JournalActionPublish,
		Status:       JournalStatusCompleted,
		ManifestPath: manifestPath,
		CreatedAt:    now,
		CompletedAt:  now,
	}
	if err := s.repo.CreatePublishJournal(journal); err != nil {
		return nil, err
	}
	binding := &ActiveRevisionBinding{
		ProcessingTaskID: processingTaskID,
		ActionKey:        actionKey,
		RevisionID:       revID,
		BindingVersion:   1,
		ActivatedBy:      userID,
		Reason:           "legacy_import",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.db.Create(binding).Error; err != nil {
		return nil, err
	}
	return &CommitSessionResponse{
		RevisionID: revID,
		Status:     "ready",
	}, nil
}

func (s *service) ListActionStreams(ctx context.Context, userID string) ([]ActionStreamSummary, error) {
	streams, err := s.repo.ListAllActionStreams(userID)
	if err != nil {
		return nil, err
	}
	summaries := make([]ActionStreamSummary, 0, len(streams))
	for _, stream := range streams {
		summary := ActionStreamSummary{
			ID:                   stream.ID,
			UserID:               stream.UserID,
			CharacterID:          stream.CharacterID,
			ActionKey:            stream.ActionKey,
			RootProcessingTaskID: stream.RootProcessingTaskID,
			StreamKey:            stream.StreamKey,
			NextRevisionNumber:   stream.NextRevisionNumber,
			CreatedAt:            stream.CreatedAt,
			UpdatedAt:            stream.UpdatedAt,
		}

		binding, err := s.repo.GetActiveActionRevisionBindingByStream(stream.UserID, stream.ID)
		if err == nil && binding != nil {
			summary.ActiveRevisionID = binding.ActiveActionRevisionID
			summary.BindingRevision = binding.BindingRevision
		}

		revs, err := s.repo.ListActionRevisionsByStream(stream.UserID, stream.ID)
		if err == nil {
			summary.RevisionCount = len(revs)
		}

		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *service) ListRevisionsByStream(ctx context.Context, userID string, streamID string) ([]RevisionSummary, error) {
	revs, err := s.repo.ListActionRevisionsByStream(userID, streamID)
	if err != nil {
		return nil, err
	}
	activeRevisionID := ""
	if activeBinding, err := s.repo.GetActiveActionRevisionBindingByStream(userID, streamID); err == nil && activeBinding != nil {
		activeRevisionID = activeBinding.ActiveActionRevisionID
	}
	summaries := make([]RevisionSummary, 0, len(revs))
	for _, rev := range revs {
		summaries = append(summaries, RevisionSummary{
			ID:               rev.ID,
			RevisionNumber:   rev.RevisionNumber,
			RevisionType:     rev.RevisionType,
			Status:           rev.Status,
			FrameCount:       rev.FrameCount,
			DurationMS:       rev.DurationMS,
			DefaultFPS:       rev.DefaultFPS,
			LoopType:         rev.LoopType,
			QualityVerdict:   rev.QualityVerdict,
			ChangeSummary:    rev.ChangeSummary,
			ParentRevisionID: rev.ParentRevisionID,
			IsActive:         rev.ID == activeRevisionID,
			CreatedAt:        rev.CreatedAt,
		})
	}
	return summaries, nil
}

func (s *service) GetActiveRevisionByStream(ctx context.Context, userID string, streamID string) (*RevisionDetail, error) {
	binding, err := s.repo.GetActiveActionRevisionBindingByStream(userID, streamID)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrRevisionNotFound
	}
	return s.GetRevision(ctx, binding.ActiveActionRevisionID)
}
