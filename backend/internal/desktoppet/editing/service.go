package editing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
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

	ListActionStreams(ctx context.Context, userID string) ([]ActionStreamSummary, error)
	ListRevisionsByStream(ctx context.Context, userID string, streamID string) ([]RevisionSummary, error)
	GetActiveRevisionByStream(ctx context.Context, userID string, streamID string) (*RevisionDetail, error)
}

type service struct {
	repo                Repository
	assetStore          RevisionAssetStore
	genAdapter          GenerationAdapter
	procAdapter         ProcessingAdapter
	qualAdapter         QualityAdapter
	candidateAcceptance *CandidateAcceptanceService
	db                  *gorm.DB
	dataDir             string
}

func NewService(repo Repository, assetStore RevisionAssetStore, genAdapter GenerationAdapter, procAdapter ProcessingAdapter, qualAdapter QualityAdapter, db *gorm.DB, dataDir string) Service {
	svc := &service{
		repo:        repo,
		assetStore:  assetStore,
		genAdapter:  genAdapter,
		procAdapter: procAdapter,
		qualAdapter: qualAdapter,
		db:          db,
		dataDir:     dataDir,
	}
	svc.candidateAcceptance = NewCandidateAcceptanceService(repo, qualAdapter, NewAuditOutbox(repo), svc.ApplyOperation)
	return svc
}

type draftFrame struct {
	FrameID           string  `json:"frameId"`
	AssetID           string  `json:"assetId"`
	LogicalIndex      int     `json:"logicalIndex"`
	DurationMS        int     `json:"durationMs"`
	AnchorX           float64 `json:"anchorX"`
	AnchorY           float64 `json:"anchorY"`
	AnchorSpace       string  `json:"anchorSpace"`
	SourceFrameID     string  `json:"sourceFrameId,omitempty"`
	SourceRevisionID  string  `json:"sourceRevisionId,omitempty"`
	SourceAttemptID   string  `json:"sourceAttemptId,omitempty"`
	LineageType       string  `json:"lineageType,omitempty"`
	MaskAssetID       string  `json:"maskAssetId,omitempty"`
	TransformJSON     string  `json:"transformJson,omitempty"`
	MetadataJSON      string  `json:"metadataJson,omitempty"`
	CopiedFromFrameID string  `json:"copiedFromFrameId,omitempty"`
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
	return prefix + "-" + uuid.NewString()
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
	if session.Status == SessionStatusCommitting || session.Status == SessionStatusConflicted {
		return ErrSessionStale
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
	binding, err := resolveActiveRevisionBinding(s.repo, processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	activeRevisionID := ""
	var revs []ActionRevision
	if binding != nil {
		activeRevisionID = binding.RevisionID
	}
	if binding != nil && binding.Canonical && binding.ActionStreamID != "" {
		revs, err = s.repo.ListActionRevisionsByStream(binding.UserID, binding.ActionStreamID)
	} else {
		revs, err = s.repo.ListActionRevisions(processingTaskID, actionKey)
	}
	if err != nil {
		return nil, err
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
	binding, err := resolveActiveRevisionBinding(s.repo, processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if binding == nil || binding.RevisionID == "" {
		return nil, ErrRevisionNotFound
	}
	return s.GetRevision(ctx, binding.RevisionID)
}

func (s *service) ActivateRevision(ctx context.Context, processingTaskID, actionKey, revisionID string, expectedVersion int64, reason, userID string) error {
	rev, err := s.repo.GetActionRevision(revisionID)
	if err != nil {
		return err
	}
	if rev.Status != RevisionStatusReady && rev.Status != RevisionStatusQualityReady {
		return ErrRevisionNotReady
	}
	if rev.ActionKey != actionKey {
		return ErrRevisionNotFound
	}
	if rev.ActionStreamID == "" {
		if rev.ProcessingTaskID != processingTaskID {
			return ErrRevisionNotFound
		}
	} else {
		binding, bindErr := resolveActiveRevisionBinding(s.repo, processingTaskID, actionKey)
		if bindErr != nil {
			return bindErr
		}
		if binding == nil || !binding.Canonical || binding.ActionStreamID != rev.ActionStreamID {
			return ErrRevisionNotFound
		}
	}
	if rev.UserID != "" && rev.UserID != userID {
		return ErrPermissionDenied
	}
	_, _, err = bindActiveRevision(s.repo, processingTaskID, actionKey, revisionID, expectedVersion, userID, reason)
	return err
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
	binding, err := resolveActiveRevisionBinding(s.repo, processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if binding == nil || binding.RevisionID == "" {
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
	var allRevs []ActionRevision
	if binding.Canonical && binding.ActionStreamID != "" {
		allRevs, err = s.repo.ListActionRevisionsByStream(binding.UserID, binding.ActionStreamID)
	} else {
		allRevs, err = s.repo.ListActionRevisions(processingTaskID, actionKey)
	}
	if err != nil {
		return nil, err
	}
	return &ActionEditSummary{
		ActionKey:         actionKey,
		ActiveRevisionID:  binding.RevisionID,
		ActiveRevisionNum: rev.RevisionNumber,
		BindingVersion:    binding.BindingRevision,
		FrameCount:        rev.FrameCount,
		DurationMS:        rev.DurationMS,
		QualityVerdict:    rev.QualityVerdict,
		HasOpenSession:    len(openSessions) > 0,
		RevisionCount:     len(allRevs),
		Timeline:          timeline,
	}, nil
}

func (s *service) CreateSession(ctx context.Context, processingTaskID, actionKey, userID string, req CreateSessionRequest) (*CreateSessionResponse, error) {
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

	baseRev, err := s.repo.GetActionRevision(req.BaseRevisionID)
	if err != nil {
		return nil, err
	}
	if baseRev.ActionKey != actionKey {
		return nil, ErrRevisionNotFound
	}
	if baseRev.UserID != "" && baseRev.UserID != userID {
		return nil, ErrPermissionDenied
	}
	binding, err := resolveActiveRevisionBinding(s.repo, processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if binding == nil || binding.RevisionID != req.BaseRevisionID {
		return nil, ErrSessionStale
	}
	if baseRev.ActionStreamID == "" {
		if baseRev.ProcessingTaskID != processingTaskID {
			return nil, ErrRevisionNotFound
		}
	} else if !binding.Canonical || binding.ActionStreamID != baseRev.ActionStreamID {
		return nil, ErrRevisionNotFound
	}
	if binding.UserID != "" && binding.UserID != userID {
		return nil, ErrPermissionDenied
	}

	characterID := baseRev.CharacterID
	actionStreamID := baseRev.ActionStreamID
	if actionStreamID != "" {
		stream, err := s.repo.GetActionStreamByID(actionStreamID)
		if err != nil {
			return nil, err
		}
		if stream == nil || (stream.UserID != "" && stream.UserID != userID) {
			return nil, ErrPermissionDenied
		}
		if characterID == "" {
			characterID = stream.CharacterID
		}
	}

	now := nowUTC()
	expiresAt := time.Now().UTC().Add(SessionDefaultTTLHours * time.Hour).Format(time.RFC3339)
	sessionID := generateID("sess")
	session := &EditSession{
		ID:                    sessionID,
		UserID:                userID,
		CharacterID:           characterID,
		ActionStreamID:        actionStreamID,
		ProcessingTaskID:      processingTaskID,
		ActionKey:             actionKey,
		BaseRevisionID:        req.BaseRevisionID,
		BaseActionContentHash: baseRev.ContentHash,
		BaseBindingRevision:   binding.BindingRevision,
		SessionVersion:        1,
		Status:                SessionStatusOpen,
		Cursor:                0,
		LastOperationSeq:      0,
		ClientInstanceID:      req.ClientInstanceID,
		ExpiresAt:             expiresAt,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.repo.CreateEditSession(session); err != nil {
		return nil, err
	}
	if _, err := s.ensureDraftSnapshot(ctx, session); err != nil {
		_ = s.db.Delete(&EditSession{}, "id = ?", sessionID).Error
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
		if err := s.repo.CreateIdempotencyRecord(record); err != nil {
			// The session is already durable. A duplicate idempotency insert can
			// only happen under a concurrent retry; return the persisted result.
			if existing, readErr := s.repo.GetIdempotencyRecord(userID, "session_create", req.IdempotencyKey); readErr == nil && existing != nil {
				var result CreateSessionResponse
				if json.Unmarshal([]byte(existing.ResultJSON), &result) == nil {
					return &result, nil
				}
			}
			return nil, err
		}
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
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if session.UserID != userID {
		return nil, ErrPermissionDenied
	}

	activationPolicy := req.ActivationPolicy
	if activationPolicy == "" {
		activationPolicy = ActivationPolicyImmediate
	}
	switch activationPolicy {
	case ActivationPolicyImmediate, ActivationPolicyManual, ActivationPolicyKeepCurrent:
	case ActivationPolicyAfterQualityPass:
		return nil, NewEditError(ErrCodeEditOperationInvalid, "after_quality_pass requires a deferred activation coordinator and is disabled fail-closed")
	default:
		return nil, NewEditError(ErrCodeEditOperationInvalid, "未知的Revision激活策略")
	}

	effectiveIdempotencyKey := req.IdempotencyKey
	if effectiveIdempotencyKey == "" {
		effectiveIdempotencyKey = fmt.Sprintf("commit:%s:%d", session.ID, req.ExpectedSessionVersion)
	}
	idem, err := s.repo.GetIdempotencyRecord(userID, sessionID, effectiveIdempotencyKey)
	if err != nil {
		return nil, err
	}
	if idem != nil && idem.Status == "completed" {
		var result CommitSessionResponse
		if err := json.Unmarshal([]byte(idem.ResultJSON), &result); err == nil {
			return &result, nil
		}
	}

	// A retry can arrive after the durable session state was committed but
	// before the idempotency record was finalized. Reconstruct the response
	// from the committed revision instead of rejecting the retry.
	if session.Status == SessionStatusCommitted && session.CommittedRevisionID != "" {
		committed, getErr := s.repo.GetActionRevision(session.CommittedRevisionID)
		if getErr != nil {
			return nil, getErr
		}
		resp := &CommitSessionResponse{RevisionID: committed.ID, QualityJobID: committed.QualityEvaluationID, Status: SessionStatusCommitted}
		resultJSON, _ := json.Marshal(resp)
		if idem != nil {
			_ = s.repo.UpdateIdempotencyRecord(idem.ID, "completed", string(resultJSON))
		} else {
			_ = s.repo.CreateIdempotencyRecord(&EditIdempotencyRecord{
				ID:             generateID("idem"),
				UserID:         userID,
				SessionID:      sessionID,
				IdempotencyKey: effectiveIdempotencyKey,
				Endpoint:       "commit_session",
				ResultJSON:     string(resultJSON),
				Status:         "completed",
				CreatedAt:      nowUTC(),
			})
		}
		return resp, nil
	}
	if session.Status != SessionStatusCommitting {
		if err := s.validateSession(session); err != nil {
			return nil, err
		}
	}
	if session.SessionVersion != req.ExpectedSessionVersion {
		return nil, NewEditErrorWithDetail(ErrCodeEditSessionVersionConflict, "会话版本冲突", SessionConflictDetail{
			CurrentVersion:  session.SessionVersion,
			ExpectedVersion: req.ExpectedSessionVersion,
		})
	}

	// Use a deterministic revision ID for this idempotent commit. That makes a
	// crash between file materialization and DB finalization recoverable.
	revID := "rev-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(session.ID+"|"+effectiveIdempotencyKey)).String()
	recoveredRevisionNumber := 0
	var recovered ActionRevision
	recoverErr := s.db.Where("id = ?", revID).First(&recovered).Error
	if recoverErr == nil {
		recoveredRevisionNumber = recovered.RevisionNumber
		binding, bindErr := resolveActiveRevisionBinding(s.repo, session.ProcessingTaskID, session.ActionKey)
		if bindErr != nil {
			return nil, bindErr
		}
		if recovered.ManifestHash != "" && recovered.Status != RevisionStatusBuilding &&
			(activationPolicy != ActivationPolicyImmediate || (binding != nil && binding.RevisionID == recovered.ID)) {
			journalID := "jrn-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(sessionID+"|"+revID+"|publish-journal")).String()
			var existingJournal PublishJournal
			journalErr := s.db.Where("id = ?", journalID).First(&existingJournal).Error
			if errors.Is(journalErr, gorm.ErrRecordNotFound) {
				journal := &PublishJournal{ID: journalID, RevisionID: revID, SessionID: sessionID, Action: JournalActionPublish, Status: JournalStatusCompleted, ManifestPath: recovered.ManifestPath, CreatedAt: nowUTC(), CompletedAt: nowUTC()}
				if err := s.repo.CreatePublishJournal(journal); err != nil {
					return nil, err
				}
			} else if journalErr != nil {
				return nil, journalErr
			}
			if err := s.repo.UpdateSessionCommitted(session.ID, recovered.ID); err != nil {
				return nil, err
			}
			resp := &CommitSessionResponse{RevisionID: recovered.ID, QualityJobID: recovered.QualityEvaluationID, Status: SessionStatusCommitted}
			resultJSON, _ := json.Marshal(resp)
			if idem == nil {
				idem = &EditIdempotencyRecord{ID: generateID("idem"), UserID: userID, SessionID: sessionID, IdempotencyKey: effectiveIdempotencyKey, Endpoint: "commit_session", CreatedAt: nowUTC()}
				if err := s.repo.CreateIdempotencyRecord(idem); err != nil {
					if existing, e := s.repo.GetIdempotencyRecord(userID, sessionID, effectiveIdempotencyKey); e == nil && existing != nil {
						idem = existing
					} else {
						return nil, err
					}
				}
			}
			_ = s.repo.UpdateIdempotencyRecord(idem.ID, "completed", string(resultJSON))
			return resp, nil
		}
		// The deterministic revision is only partially materialized. Remove its
		// DB rows and rebuild the same ID. Asset files are content-addressed and
		// safe to reuse; the manifest path is overwritten atomically by storage.
		_ = s.repo.DeleteRevisionFrames(revID)
		if err := s.db.Delete(&ActionRevision{}, "id = ?", revID).Error; err != nil {
			return nil, err
		}
	} else if !errors.Is(recoverErr, gorm.ErrRecordNotFound) {
		return nil, recoverErr
	}

	if _, err := s.validateSessionBaseBinding(session); err != nil {
		_ = s.repo.UpdateSessionStatus(session.ID, SessionStatusConflicted)
		return nil, err
	}
	if idem == nil {
		idem = &EditIdempotencyRecord{
			ID:             generateID("idem"),
			UserID:         userID,
			SessionID:      sessionID,
			IdempotencyKey: effectiveIdempotencyKey,
			Endpoint:       "commit_session",
			Status:         "pending",
			CreatedAt:      nowUTC(),
		}
		if err := s.repo.CreateIdempotencyRecord(idem); err != nil {
			existing, readErr := s.repo.GetIdempotencyRecord(userID, sessionID, effectiveIdempotencyKey)
			if readErr != nil || existing == nil {
				return nil, err
			}
			idem = existing
			if idem.Status == "completed" {
				var result CommitSessionResponse
				if json.Unmarshal([]byte(idem.ResultJSON), &result) == nil {
					return &result, nil
				}
			}
		}
	}

	if err := s.repo.UpdateSessionStatus(sessionID, SessionStatusCommitting); err != nil {
		_ = s.repo.UpdateIdempotencyRecord(idem.ID, "failed", "")
		return nil, err
	}
	failCommitOpen := func(commitErr error) (*CommitSessionResponse, error) {
		_ = s.repo.UpdateSessionStatus(sessionID, SessionStatusOpen)
		_ = s.repo.UpdateIdempotencyRecord(idem.ID, "failed", "")
		return nil, commitErr
	}
	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		return failCommitOpen(err)
	}
	if len(ds.Frames) < MinFrameCount {
		return failCommitOpen(ErrFrameCountInvalid)
	}
	baseRev, err := s.repo.GetActionRevision(session.BaseRevisionID)
	if err != nil {
		return failCommitOpen(err)
	}

	var revNum int
	if recoveredRevisionNumber > 0 {
		revNum = recoveredRevisionNumber
	} else if session.ActionStreamID != "" {
		revNum, err = allocateActionStreamRevisionNumber(s.repo, session.ActionStreamID)
	} else {
		revNum, err = s.repo.GetNextRevisionNumber(session.ProcessingTaskID, session.ActionKey)
	}
	if err != nil {
		return failCommitOpen(err)
	}

	rootRevID := baseRev.RootRevisionID
	if rootRevID == "" {
		rootRevID = baseRev.ID
	}
	rootActionRevID := baseRev.RootActionRevisionID
	if rootActionRevID == "" {
		rootActionRevID = baseRev.ID
	}
	actionConfigJSON, err := marshalCanonicalJSON(draftActionConfig(ds))
	if err != nil {
		return failCommitOpen(err)
	}
	now := nowUTC()
	interruptible := 0
	if ds.Interruptible {
		interruptible = 1
	}
	newRev := &ActionRevision{
		ID:                         revID,
		UserID:                     session.UserID,
		CharacterID:                session.CharacterID,
		ProcessingTaskID:           session.ProcessingTaskID,
		ProcessingActionID:         baseRev.ProcessingActionID,
		GenerationTaskID:           baseRev.GenerationTaskID,
		ActionKey:                  session.ActionKey,
		ParentRevisionID:           session.BaseRevisionID,
		RootRevisionID:             rootRevID,
		RevisionNumber:             revNum,
		RevisionType:               RevisionTypeEdit,
		Status:                     RevisionStatusBuilding,
		FrameCount:                 len(ds.Frames),
		DurationMS:                 s.totalDuration(ds),
		DefaultFPS:                 ds.DefaultFPS,
		LoopType:                   ds.LoopType,
		ReturnAction:               ds.ReturnAction,
		Interruptible:              interruptible,
		PriorityOverride:           ds.PriorityOverride,
		CooldownMSOverride:         ds.CooldownMSOverride,
		CreatedByUserID:            userID,
		CreatedFromSessionID:       sessionID,
		ChangeSummary:              req.ChangeSummary,
		CreatedAt:                  now,
		UpdatedAt:                  now,
		SourceType:                 canonicalSourceManualEdit,
		ContentHashVersion:         canonicalContentHashVersionManifestV1,
		Origin:                     canonicalOriginUser,
		PlaybackMode:               baseRev.PlaybackMode,
		ActionStreamID:             session.ActionStreamID,
		SourceProcessingRevisionID: baseRev.SourceProcessingRevisionID,
		SourceProcessingTaskID:     baseRev.SourceProcessingTaskID,
		SourceProcessingActionID:   baseRev.SourceProcessingActionID,
		SourceProcessingAttemptID:  baseRev.SourceProcessingAttemptID,
		ParentActionRevisionID:     baseRev.ID,
		RootActionRevisionID:       rootActionRevID,
		ActionConfigSnapshotJSON:   actionConfigJSON,
		ActionSpecHash:             baseRev.ActionSpecHash,
	}
	if newRev.SourceProcessingTaskID == "" {
		newRev.SourceProcessingTaskID = baseRev.ProcessingTaskID
	}
	if newRev.SourceProcessingActionID == "" {
		newRev.SourceProcessingActionID = baseRev.ProcessingActionID
	}
	if err := s.repo.CreateActionRevision(newRev); err != nil {
		return failCommitOpen(err)
	}

	manifest, err := s.buildManifestFromDraft(revID, session.BaseRevisionID, session.ProcessingTaskID, session.ActionKey, ds)
	if err != nil {
		return failCommitOpen(err)
	}
	manifestPath, manifestHash, err := s.assetStore.WriteManifest(revID, manifest)
	if err != nil {
		return failCommitOpen(err)
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
		return failCommitOpen(err)
	}
	if err := s.repo.UpdateActionRevisionManifest(revID, manifestPath, manifestHash, len(ds.Frames), s.totalDuration(ds)); err != nil {
		return failCommitOpen(err)
	}
	configJSON, configHash, frameSetHash, revisionSnapshotJSON, revisionSnapshotHash, err := s.revisionHashesFromDraft(ds, manifestHash)
	if err != nil {
		return failCommitOpen(err)
	}
	if err := s.db.Model(&ActionRevision{}).Where("id = ?", revID).Updates(map[string]any{
		"content_hash":                manifestHash,
		"content_hash_version":        canonicalContentHashVersionManifestV1,
		"action_config_snapshot_json": configJSON,
		"action_config_hash":          configHash,
		"frame_set_hash":              frameSetHash,
		"revision_snapshot_json":      revisionSnapshotJSON,
		"revision_snapshot_hash":      revisionSnapshotHash,
		"updated_at":                  nowUTC(),
	}).Error; err != nil {
		return failCommitOpen(err)
	}
	if err := s.repo.UpdateActionRevisionStatus(revID, RevisionStatusReady); err != nil {
		return failCommitOpen(err)
	}

	if activationPolicy == ActivationPolicyImmediate {
		if _, _, err := bindActiveRevision(s.repo, session.ProcessingTaskID, session.ActionKey, revID, session.BaseBindingRevision, userID, "editor.commit"); err != nil {
			_ = s.repo.UpdateSessionStatus(sessionID, SessionStatusConflicted)
			_ = s.repo.UpdateIdempotencyRecord(idem.ID, "failed", "")
			return nil, err
		}
	}
	journal := &PublishJournal{
		ID:           "jrn-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(sessionID+"|"+revID+"|publish-journal")).String(),
		RevisionID:   revID,
		SessionID:    sessionID,
		Action:       JournalActionPublish,
		Status:       JournalStatusCompleted,
		ManifestPath: manifestPath,
		CreatedAt:    now,
		CompletedAt:  nowUTC(),
	}
	if err := s.repo.CreatePublishJournal(journal); err != nil {
		var existing PublishJournal
		if readErr := s.db.Where("id = ?", journal.ID).First(&existing).Error; readErr != nil || existing.RevisionID != revID || existing.SessionID != sessionID {
			_ = s.repo.UpdateIdempotencyRecord(idem.ID, "failed", "")
			return nil, err
		}
	}
	if err := s.repo.UpdateSessionCommitted(sessionID, revID); err != nil {
		_ = s.repo.UpdateIdempotencyRecord(idem.ID, "failed", "")
		return nil, err
	}

	resp := &CommitSessionResponse{RevisionID: revID, Status: SessionStatusCommitted}
	resultJSON, _ := json.Marshal(resp)
	if err := s.repo.UpdateIdempotencyRecord(idem.ID, "completed", string(resultJSON)); err != nil {
		return nil, err
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
	if session.UserID != userID {
		return nil, ErrPermissionDenied
	}
	if _, err := s.validateSessionBaseBinding(session); err != nil {
		_ = s.repo.UpdateSessionStatus(session.ID, SessionStatusConflicted)
		return nil, err
	}
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetJobByIdempotencyKey(sessionID, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return &CreateRegenerationJobResponse{JobID: existing.ID, Status: existing.Status, CostEstimate: existing.CostEstimateJSON}, nil
		}
	}

	mode := ""
	switch req.JobType {
	case JobTypeSingleFrame:
		mode = RegenModeSingleFrame
	case JobTypeFullAction:
		mode = RegenModeFullAction
	case JobTypeBackgroundReprocess:
		mode = RegenModeBgReprocess
	case JobTypeNormalizeUpload:
		mode = RegenModeNormalize
	default:
		return nil, NewEditError(ErrCodeEditOperationInvalid, "未知的重生成任务类型")
	}
	if req.JobType != JobTypeFullAction && req.TargetFrameID == "" {
		return nil, ErrFrameNotFound
	}

	draftSnapshot, err := s.ensureDraftSnapshot(ctx, session)
	if err != nil {
		return nil, err
	}
	var draftFrames []draftFrame
	if err := json.Unmarshal([]byte(draftSnapshot.FramesJSON), &draftFrames); err != nil {
		return nil, fmt.Errorf("decode immutable draft snapshot: %w", err)
	}
	targetContentHash := ""
	if req.TargetFrameID != "" {
		found := false
		for _, frame := range draftFrames {
			if frame.FrameID != req.TargetFrameID {
				continue
			}
			asset, err := s.repo.GetFrameAsset(frame.AssetID)
			if err != nil {
				return nil, err
			}
			targetContentHash = asset.ContentHash
			found = true
			break
		}
		if !found {
			return nil, ErrFrameNotFound
		}
	}

	requestSnapshot := map[string]any{
		"targetFrameId":     req.TargetFrameID,
		"jobType":           req.JobType,
		"fixIntent":         req.FixIntent,
		"useAdjacentFrames": req.UseAdjacentFrames,
	}
	requestJSON, err := marshalCanonicalJSON(requestSnapshot)
	if err != nil {
		return nil, err
	}
	requestHash := s.assetStore.ComputeHash([]byte(requestJSON))
	effectiveIdempotencyKey := req.IdempotencyKey
	if effectiveIdempotencyKey == "" {
		effectiveIdempotencyKey = "regen:" + requestHash
	}
	if req.IdempotencyKey == "" {
		existing, err := s.repo.GetJobByIdempotencyKey(sessionID, effectiveIdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return &CreateRegenerationJobResponse{JobID: existing.ID, Status: existing.Status, CostEstimate: existing.CostEstimateJSON}, nil
		}
	}
	confirmationID := ""
	if req.CostConfirmationToken != "" {
		confirmationID = "cost-confirm-" + s.assetStore.ComputeHash([]byte(req.CostConfirmationToken))
	}

	now := nowUTC()
	jobID := generateID("regen")
	job := &RegenerationJob{
		ID:                   jobID,
		SessionID:            sessionID,
		UserID:               session.UserID,
		CharacterID:          session.CharacterID,
		ActionStreamID:       session.ActionStreamID,
		DraftSnapshotID:      draftSnapshot.ID,
		DraftSnapshotHash:    draftSnapshot.SnapshotHash,
		ProcessingTaskID:     session.ProcessingTaskID,
		ActionKey:            session.ActionKey,
		TargetFrameID:        req.TargetFrameID,
		JobType:              req.JobType,
		Mode:                 mode,
		Status:               JobStatusCreated,
		Stage:                JobStatusCreated,
		IdempotencyKey:       effectiveIdempotencyKey,
		RequestHash:          requestHash,
		RequestSnapshotJSON:  requestJSON,
		CostEstimateJSON:     "{}",
		BaseActionRevisionID: session.BaseRevisionID,
		BaseContentHash:      draftSnapshot.BaseContentHash,
		BaseBindingRevision:  draftSnapshot.BaseBindingRevision,
		InstanceID:           session.ClientInstanceID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	inputSnapshot := &RegenerationJobInputSnapshot{
		JobID:                  jobID,
		SessionID:              sessionID,
		DraftSnapshotID:        draftSnapshot.ID,
		DraftSnapshotHash:      draftSnapshot.SnapshotHash,
		RequestJSON:            requestJSON,
		RequestHash:            requestHash,
		BaseRevisionID:         session.BaseRevisionID,
		BaseContentHash:        draftSnapshot.BaseContentHash,
		BaseBindingRevision:    draftSnapshot.BaseBindingRevision,
		TargetFrameID:          req.TargetFrameID,
		TargetFrameContentHash: targetContentHash,
		CostEstimateJSON:       "{}",
		CostEstimateHash:       s.assetStore.ComputeHash([]byte("{}")),
		CostConfirmationID:     confirmationID,
		CreatedAt:              now,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(inputSnapshot).Error
	}); err != nil {
		if existing, readErr := s.repo.GetJobByIdempotencyKey(sessionID, effectiveIdempotencyKey); readErr == nil && existing != nil {
			return &CreateRegenerationJobResponse{JobID: existing.ID, Status: existing.Status, CostEstimate: existing.CostEstimateJSON}, nil
		}
		return nil, err
	}
	return &CreateRegenerationJobResponse{JobID: jobID, Status: JobStatusCreated}, nil
}

func (s *service) GetRegenerationJob(ctx context.Context, jobID string) (*RegenerationJob, error) {
	job, err := s.repo.GetRegenerationJob(jobID)
	if err != nil || job == nil {
		return job, err
	}
	if job.Status != JobStatusQualityPending && job.Status != JobStatusQualityRunning {
		return job, nil
	}
	var candidate EditCandidate
	if err := s.db.Where("job_id = ?", job.ID).Order("created_at DESC").Limit(1).Find(&candidate).Error; err != nil || candidate.ID == "" || candidate.CandidateRevisionID == "" {
		return job, err
	}
	passed, reason, gateErr := s.qualAdapter.IsGatePassed(ctx, candidate.CandidateRevisionID)
	if gateErr != nil || (!passed && reason == "quality_pending") {
		return job, gateErr
	}
	rev, revErr := s.repo.GetActionRevision(candidate.CandidateRevisionID)
	if revErr != nil {
		return job, revErr
	}
	effectiveVerdict := rev.QualityVerdict
	if effectiveVerdict == "" {
		effectiveVerdict = reason
	}
	if err := s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
		"status":            CandidateStatusReadyForReview,
		"quality_status":    reason,
		"effective_verdict": effectiveVerdict,
	}); err != nil {
		return job, err
	}
	if err := s.repo.UpdateJobFields(job.ID, map[string]any{"status": JobStatusReadyForReview, "stage": JobStatusReadyForReview}); err != nil {
		return job, err
	}
	job.Status = JobStatusReadyForReview
	job.Stage = JobStatusReadyForReview
	return job, nil
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
	if job.UserID != "" && job.UserID != userID {
		return ErrPermissionDenied
	}
	if IsTerminalJobStatus(job.Status) || job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
		return ErrJobNotCancelable
	}
	return s.repo.UpdateJobFields(jobID, map[string]any{
		"status":              JobStatusCancelled,
		"cancel_requested_at": nowUTC(),
		"completed_at":        nowUTC(),
		"lease_owner":         "",
		"lease_expires_at":    "",
		"heartbeat_at":        "",
		"execution_id":        "",
	})
}

func (s *service) AcceptCandidate(ctx context.Context, candidateID, userID string, req AcceptCandidateRequest) error {
	return s.candidateAcceptance.AcceptCandidate(ctx, candidateID, userID, req.IdempotencyKey)
}

func (s *service) RejectCandidate(ctx context.Context, candidateID, userID string, req RejectCandidateRequest) error {
	return s.candidateAcceptance.RejectCandidate(ctx, candidateID, userID, "user_rejected", req.IdempotencyKey)
}

func (s *service) ListCandidates(ctx context.Context, sessionID string) ([]EditCandidate, error) {
	session, err := s.repo.GetEditSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	candidates, err := s.repo.ListCandidatesBySession(sessionID)
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.CandidateRevisionID == "" ||
			(candidate.Status != CandidateStatusQualityPending && candidate.Status != CandidateStatusQualityRunning) {
			continue
		}
		passed, reason, gateErr := s.qualAdapter.IsGatePassed(ctx, candidate.CandidateRevisionID)
		if gateErr != nil || (!passed && reason == "quality_pending") {
			continue
		}
		rev, revErr := s.repo.GetActionRevision(candidate.CandidateRevisionID)
		if revErr != nil {
			continue
		}
		effectiveVerdict := rev.QualityVerdict
		if effectiveVerdict == "" {
			effectiveVerdict = reason
		}
		updates := map[string]any{
			"status":            CandidateStatusReadyForReview,
			"quality_status":    reason,
			"effective_verdict": effectiveVerdict,
		}
		if s.repo.UpdateCandidateFields(candidate.ID, updates) == nil {
			candidate.Status = CandidateStatusReadyForReview
			candidate.QualityStatus = reason
			candidate.EffectiveVerdict = effectiveVerdict
			_ = s.repo.UpdateCandidateRevisionMetadataStatus(candidate.CandidateRevisionID, CandidateStatusReadyForReview)
			_ = s.repo.UpdateJobFields(candidate.JobID, map[string]any{"status": JobStatusReadyForReview, "stage": JobStatusReadyForReview})
		}
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
	if session.UserID != userID {
		return nil, ErrPermissionDenied
	}
	if _, err := s.validateSessionBaseBinding(session); err != nil {
		return nil, err
	}
	if len(data) == 0 || targetFrameID == "" {
		return nil, ErrUploadInvalid
	}
	snapshot, err := s.ensureDraftSnapshot(ctx, session)
	if err != nil {
		return nil, err
	}
	var frames []draftFrame
	if err := json.Unmarshal([]byte(snapshot.FramesJSON), &frames); err != nil {
		return nil, err
	}
	found := false
	for _, frame := range frames {
		if frame.FrameID == targetFrameID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrFrameNotFound
	}
	asset, err := s.assetStore.WriteAsset(ctx, data, mimeType, AssetSourceUploaded, userID)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	candidateID := generateID("cand")
	candidate := &EditCandidate{
		ID:                  candidateID,
		SessionID:           sessionID,
		UserID:              session.UserID,
		CharacterID:         session.CharacterID,
		ActionStreamID:      session.ActionStreamID,
		CandidateVersion:    snapshot.SessionVersion,
		DraftSnapshotID:     snapshot.ID,
		DraftSnapshotHash:   snapshot.SnapshotHash,
		TargetFrameID:       targetFrameID,
		CandidateType:       AssetSourceUploaded,
		AssetID:             asset.ID,
		Status:              CandidateStatusReadyForReview,
		SourceType:          AssetSourceUploaded,
		ParentRevisionID:    session.BaseRevisionID,
		ParentContentHash:   session.BaseActionContentHash,
		BaseBindingRevision: session.BaseBindingRevision,
		ActivationPolicy:    ActivationPolicyManual,
		CreatedAt:           now,
	}
	if err := s.repo.CreateCandidate(candidate); err != nil {
		return nil, err
	}
	return &UploadCandidateResponse{CandidateID: candidateID, AssetID: asset.ID, Status: CandidateStatusReadyForReview}, nil
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
	if rev.QualityEvaluationID != "" && (rev.Status == RevisionStatusQualityPending || rev.Status == RevisionStatusQualityReady) {
		return rev.QualityEvaluationID, nil
	}
	if rev.Status == RevisionStatusBuilding {
		if err := s.repo.UpdateActionRevisionStatus(revisionID, RevisionStatusReady); err != nil {
			return "", err
		}
		rev.Status = RevisionStatusReady
	}
	if rev.Status != RevisionStatusReady {
		return "", ErrRevisionNotReady
	}
	return s.qualAdapter.EvaluateRevision(ctx, revisionID)
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
