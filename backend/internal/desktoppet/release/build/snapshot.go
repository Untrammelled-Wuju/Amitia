package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type SnapshotCreator struct {
	source     release.SnapshotSource
	gateReader release.ReleaseQualityGateReader
	repo       release.ReleaseRepository
}

func NewSnapshotCreator(
	source release.SnapshotSource,
	gateReader release.ReleaseQualityGateReader,
	repo release.ReleaseRepository,
) *SnapshotCreator {
	return &SnapshotCreator{
		source:     source,
		gateReader: gateReader,
		repo:       repo,
	}
}

type CreateSnapshotRequest struct {
	UserID           string
	PetID            string
	ProcessingTaskID string
	CharacterID      string
	DefaultAction    string
	IncludedActions  []string
}

type SnapshotResult struct {
	Snapshot          *release.ReleaseBuildSnapshot
	ActionSnapshots   []release.ReleaseActionSnapshot
	GateResult        *release.QualityGateResult
	TaskInfo          *release.TaskInfo
	Identity          *release.PetIdentityData
	PreviewArtifactID string
}

func (sc *SnapshotCreator) Create(ctx context.Context, req *CreateSnapshotRequest) (*SnapshotResult, error) {
	taskInfo, err := sc.source.GetProcessingTaskInfo(req.ProcessingTaskID)
	if err != nil {
		return nil, NewBuildError("PROCESSING_TASK_NOT_FOUND", "处理任务不存在", err)
	}

	identity, err := sc.resolveAndValidateIdentity(req, taskInfo)
	if err != nil {
		return nil, err
	}

	if err := sc.validateOwnership(req, taskInfo, identity); err != nil {
		return nil, err
	}

	activeRevisionSetHash, err := sc.source.GetActiveRevisionSetHash(req.ProcessingTaskID)
	if err != nil {
		return nil, NewBuildError("REVISION_SET_HASH_FAILED", "获取活跃修订集合哈希失败", err)
	}

	gateResult, err := sc.gateReader.GetValidGateForRelease(ctx, req.UserID, req.ProcessingTaskID, activeRevisionSetHash)
	if err != nil {
		return nil, NewBuildError("QUALITY_GATE_READ_FAILED", "读取质量门禁失败", err)
	}
	if gateResult == nil {
		return nil, NewBuildError("quality_gate_missing", "质量门禁结果为空", nil)
	}
	if !gateResult.GateStatus.IsAllowed() {
		return nil, NewBuildError(gateResult.GateStatus.ErrorCode(),
			fmt.Sprintf("质量门禁未通过: %s", gateResult.GateStatus), nil)
	}

	includedActions, err := sc.resolveIncludedActions(req, gateResult)
	if err != nil {
		return nil, err
	}

	actionSnapshots, err := sc.buildActionSnapshots(req.ProcessingTaskID, includedActions)
	if err != nil {
		return nil, err
	}

	defaultActionKey, err := sc.resolveDefaultAction(req, identity, actionSnapshots, gateResult)
	if err != nil {
		return nil, err
	}

	previewArtifactID, err := sc.source.ResolvePreviewArtifactID(req.ProcessingTaskID, identity.ID)
	if err != nil {
		previewArtifactID = ""
	}

	buildConfigHash := sc.computeBuildConfigHash(req, includedActions, defaultActionKey)
	inputHash := computeInputHash(req.UserID, identity.ID, activeRevisionSetHash, gateResult.GateID, buildConfigHash)

	actionsJSON, _ := json.Marshal(includedActions)

	snapshot := &release.ReleaseBuildSnapshot{
		ID:                     uuid.NewString(),
		UserID:                 req.UserID,
		PetID:                  identity.ID,
		CharacterID:            taskInfo.CharacterID,
		ProcessingTaskID:       req.ProcessingTaskID,
		ActiveRevisionSetHash:  activeRevisionSetHash,
		QualityGateID:          gateResult.GateID,
		QualityGateHash:        gateResult.GateHash,
		DefaultActionKey:       defaultActionKey,
		IncludedActionsJSON:    string(actionsJSON),
		PackageSchemaVersion:   2,
		RuntimeContractVersion: "1.0.0",
		BuildConfigHash:        buildConfigHash,
		InputHash:              inputHash,
		CreatedAt:              formatTimestamp(time.Now()),
	}

	if err := sc.repo.CreateBuildSnapshot(snapshot); err != nil {
		return nil, NewBuildError("SNAPSHOT_PERSIST_FAILED", "快照持久化失败", err)
	}

	return &SnapshotResult{
		Snapshot:          snapshot,
		ActionSnapshots:   actionSnapshots,
		GateResult:        gateResult,
		TaskInfo:          taskInfo,
		Identity:          identity,
		PreviewArtifactID: previewArtifactID,
	}, nil
}

func (sc *SnapshotCreator) resolveAndValidateIdentity(req *CreateSnapshotRequest, taskInfo *release.TaskInfo) (*release.PetIdentityData, error) {
	if req.PetID != "" {
		identity, err := sc.repo.GetPetIdentity(req.PetID)
		if err != nil {
			return nil, NewBuildError(ErrCodeReleaseOwnershipDenied, "桌宠身份不存在", err)
		}
		return identity, nil
	}

	identity, err := sc.repo.GetPetIdentityByCharacter(req.UserID, taskInfo.CharacterID)
	if err == nil {
		return identity, nil
	}

	now := formatTimestamp(time.Now())
	name := taskInfo.PackageName
	if name == "" {
		name = taskInfo.CharacterID
	}
	identity = &release.PetIdentityData{
		ID:                uuid.NewString(),
		OwnerUserID:       req.UserID,
		SourceCharacterID: taskInfo.CharacterID,
		Name:              name,
		Slug:              makeSlug(name),
		BindingPolicy:     "character_locked",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := sc.repo.CreatePetIdentity(identity); err != nil {
		return nil, NewBuildError("PET_IDENTITY_CREATE_FAILED", "创建桌宠身份失败", err)
	}
	return identity, nil
}

func (sc *SnapshotCreator) validateOwnership(req *CreateSnapshotRequest, taskInfo *release.TaskInfo, identity *release.PetIdentityData) error {
	if identity.OwnerUserID != req.UserID {
		return NewBuildError(ErrCodeReleaseOwnershipDenied, "桌宠身份不属于当前用户", nil)
	}
	if req.CharacterID != "" && identity.SourceCharacterID != req.CharacterID {
		return NewBuildError(ErrCodeReleaseSourceMismatch, "角色 ID 不匹配", nil)
	}
	if taskInfo.UserID != "" && taskInfo.UserID != req.UserID {
		return NewBuildError(ErrCodeReleaseOwnershipDenied, "处理任务不属于当前用户", nil)
	}
	if taskInfo.CharacterID != "" && identity.SourceCharacterID != taskInfo.CharacterID {
		return NewBuildError(ErrCodeReleaseSourceMismatch, "处理任务角色与桌宠角色不匹配", nil)
	}
	return nil
}

func (sc *SnapshotCreator) resolveIncludedActions(req *CreateSnapshotRequest, gateResult *release.QualityGateResult) ([]string, error) {
	gateActions := gateResult.IncludedActionKeys
	if len(gateActions) == 0 {
		actions, err := sc.source.ListProcessingActions(req.ProcessingTaskID)
		if err != nil {
			return nil, NewBuildError("LIST_ACTIONS_FAILED", "列出处理动作失败", err)
		}
		for _, a := range actions {
			if a.Excluded || a.Status != "succeeded" {
				continue
			}
			gateActions = append(gateActions, a.ActionKey)
		}
	}

	sort.Strings(gateActions)

	if len(req.IncludedActions) == 0 {
		if err := sc.validateRequiredActions(gateActions, gateResult.RequiredActionKeys); err != nil {
			return nil, err
		}
		return gateActions, nil
	}

	requestedSet := make(map[string]bool, len(req.IncludedActions))
	for _, k := range req.IncludedActions {
		requestedSet[k] = true
	}

	gateSet := make(map[string]bool, len(gateActions))
	for _, k := range gateActions {
		gateSet[k] = true
	}

	result := make([]string, 0, len(gateActions))
	for _, k := range gateActions {
		if requestedSet[k] {
			result = append(result, k)
		}
	}

	if err := sc.validateRequiredActions(result, gateResult.RequiredActionKeys); err != nil {
		return nil, err
	}

	return result, nil
}

func (sc *SnapshotCreator) validateRequiredActions(included []string, required []string) error {
	if len(required) == 0 {
		return nil
	}
	includedSet := make(map[string]bool, len(included))
	for _, k := range included {
		includedSet[k] = true
	}
	for _, r := range required {
		if !includedSet[r] {
			return NewBuildError(ErrCodeReleaseDefaultActionInvalid,
				fmt.Sprintf("必需动作 %s 不能被排除", r), nil)
		}
	}
	return nil
}

func (sc *SnapshotCreator) buildActionSnapshots(processingTaskID string, actionKeys []string) ([]release.ReleaseActionSnapshot, error) {
	snapshots := make([]release.ReleaseActionSnapshot, 0, len(actionKeys))
	for _, actionKey := range actionKeys {
		detail, err := sc.source.GetActiveRevisionDetail(processingTaskID, actionKey)
		if err != nil {
			return nil, NewBuildError("REVISION_DETAIL_FAILED",
				fmt.Sprintf("获取动作 %s 的活跃修订失败", actionKey), err)
		}
		if detail == nil {
			return nil, NewBuildError("REVISION_DETAIL_FAILED",
				fmt.Sprintf("动作 %s 无活跃修订", actionKey), nil)
		}

		if err := sc.validateFrames(actionKey, detail); err != nil {
			return nil, err
		}

		frameArtifactIDs := make([]string, 0, len(detail.Frames))
		frameHashes := make([]string, 0, len(detail.Frames))
		for _, f := range detail.Frames {
			frameArtifactIDs = append(frameArtifactIDs, f.AssetID)
			frameHashes = append(frameHashes, f.ContentHash)
		}

		frameSetHash := hashStrings(frameHashes)
		contentHash := hashActionContent(detail.RevisionID, detail.FrameCount, detail.LoopType, detail.Interruptible, frameHashes)

		snapshots = append(snapshots, release.ReleaseActionSnapshot{
			ActionKey:        actionKey,
			ActionRevisionID: detail.RevisionID,
			ContentHash:      contentHash,
			ActionConfigHash: fmt.Sprintf("%s|%d|%s|%t", detail.RevisionID, detail.FrameCount, detail.LoopType, detail.Interruptible),
			FrameSetHash:     frameSetHash,
			FrameArtifactIDs: frameArtifactIDs,
			QualityVerdict:   detail.QualityVerdict,
		})
	}
	return snapshots, nil
}

func (sc *SnapshotCreator) validateFrames(actionKey string, detail *release.RevisionDetail) error {
	if detail.FrameCount != len(detail.Frames) {
		return NewBuildError(ErrCodeReleaseFrameSetIncomplete,
			fmt.Sprintf("动作 %s 帧数不一致: 声明 %d, 实际 %d", actionKey, detail.FrameCount, len(detail.Frames)), nil)
	}

	for i, frame := range detail.Frames {
		if frame.AssetID == "" {
			return NewBuildError(ErrCodeReleaseFrameAssetMissing,
				fmt.Sprintf("动作 %s 第 %d 帧缺少 AssetID", actionKey, i), nil)
		}
		if frame.ContentHash == "" {
			return NewBuildError(ErrCodeReleaseFrameHashMismatch,
				fmt.Sprintf("动作 %s 第 %d 帧缺少 ContentHash", actionKey, i), nil)
		}
	}

	for i := 1; i < len(detail.Frames); i++ {
		if detail.Frames[i].LogicalIndex != detail.Frames[i-1].LogicalIndex+1 {
			return NewBuildError(ErrCodeReleaseFrameSetIncomplete,
				fmt.Sprintf("动作 %s 帧索引不连续", actionKey), nil)
		}
	}

	return nil
}

func (sc *SnapshotCreator) resolveDefaultAction(req *CreateSnapshotRequest, identity *release.PetIdentityData, snapshots []release.ReleaseActionSnapshot, gateResult *release.QualityGateResult) (string, error) {
	if req.DefaultAction != "" {
		if !containsAction(snapshots, req.DefaultAction) {
			return "", NewBuildError(ErrCodeReleaseDefaultActionInvalid,
				fmt.Sprintf("请求的默认动作 %s 不在发布集合中", req.DefaultAction), nil)
		}
		return req.DefaultAction, nil
	}

	if identity.DefaultActionKey != "" && containsAction(snapshots, identity.DefaultActionKey) {
		return identity.DefaultActionKey, nil
	}

	for _, s := range snapshots {
		actions, err := sc.source.ListProcessingActions(req.ProcessingTaskID)
		if err != nil {
			break
		}
		for _, a := range actions {
			if a.ActionKey == s.ActionKey && a.SupportsDefaultIdle {
				return s.ActionKey, nil
			}
		}
		break
	}

	if len(snapshots) > 0 {
		return snapshots[0].ActionKey, nil
	}

	return "", NewBuildError(ErrCodeReleaseDefaultActionInvalid, "无法确定默认动作", nil)
}

func (sc *SnapshotCreator) computeBuildConfigHash(req *CreateSnapshotRequest, includedActions []string, defaultAction string) string {
	sortedActions := make([]string, len(includedActions))
	copy(sortedActions, includedActions)
	sort.Strings(sortedActions)
	data := fmt.Sprintf("%s|%s|%s", strings.Join(sortedActions, ","), defaultAction, "2")
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func hashStrings(items []string) string {
	sorted := make([]string, len(items))
	copy(sorted, items)
	sort.Strings(sorted)
	h := sha256.New()
	for _, s := range sorted {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashActionContent(revisionID string, frameCount int, loopType string, interruptible bool, frameHashes []string) string {
	h := sha256.New()
	h.Write([]byte(revisionID))
	h.Write([]byte{0})
	h.Write([]byte(fmt.Sprintf("%d", frameCount)))
	h.Write([]byte{0})
	h.Write([]byte(loopType))
	h.Write([]byte{0})
	h.Write([]byte(fmt.Sprintf("%t", interruptible)))
	h.Write([]byte{0})
	for _, fh := range frameHashes {
		h.Write([]byte(fh))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func containsAction(snapshots []release.ReleaseActionSnapshot, actionKey string) bool {
	for _, s := range snapshots {
		if s.ActionKey == actionKey {
			return true
		}
	}
	return false
}

func makeSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = "pet"
	}
	return result
}
