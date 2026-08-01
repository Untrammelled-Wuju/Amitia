package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrPetIdentityNotFound = errors.New("pet identity not found")

type ReleaseService interface {
	BuildRelease(ctx context.Context, req *BuildReleaseRequest) (*BuildReleaseResult, error)
	GetBuildOperation(ctx context.Context, operationID, userID string) (*ReleaseBuildOperation, error)
	CancelBuildOperation(ctx context.Context, operationID, userID string) error
	GetRelease(ctx context.Context, releaseID, userID string) (*ReleaseData, error)
	ListReleases(ctx context.Context, userID string) ([]*ReleaseData, error)
	ListReleasesForPet(ctx context.Context, userID, petID string) ([]*ReleaseData, error)
	GetReleaseFiles(ctx context.Context, releaseID, userID string) ([]ReleaseFileData, error)
	ArchiveRelease(ctx context.Context, releaseID, userID string) error
	RevokeRelease(ctx context.Context, releaseID, userID, reason string) error
	GetPetIdentity(ctx context.Context, userID, petID string) (*PetIdentityData, error)
}

type BuildReleaseRequest struct {
	UserID             string
	ProcessingTaskID   string
	PetID              string
	CharacterID        string
	IncludedActionKeys []string
	DefaultAction      string
	BuildProfileID     string
	IdempotencyKey     string
}

type BuildReleaseResult struct {
	Operation  *ReleaseBuildOperation
	Release    *ReleaseData
	Snapshot   *ReleaseBuildSequenceInfo
}

type ReleaseBuildSequenceInfo struct {
	Sequence    int
	Version     string
	ReleaseID   string
}

type service struct {
	repo           ReleaseRepository
	identitySvc    *PetIdentityResolver
	gateReader     ReleaseQualityGateReader
	storage        ReleaseStoragePort
	eventPublisher EventPublisher
}

func NewReleaseService(
	repo ReleaseRepository,
	gateReader ReleaseQualityGateReader,
	storage ReleaseStoragePort,
	eventPublisher EventPublisher,
) ReleaseService {
	return &service{
		repo:           repo,
		identitySvc:    &PetIdentityResolver{repo: repo},
		gateReader:     gateReader,
		storage:        storage,
		eventPublisher: eventPublisher,
	}
}

type PetIdentityResolver struct {
	repo ReleaseRepository
}

func (r *PetIdentityResolver) ResolveOrCreate(
	ctx context.Context,
	userID, characterID, preferredName string,
) (*PetIdentityData, error) {
	if userID == "" {
		return nil, NewReleaseError("INVALID_USER", "用户 ID 不能为空", nil)
	}
	if characterID == "" {
		return nil, NewReleaseError("INVALID_CHARACTER", "角色 ID 不能为空", nil)
	}

	existing, err := r.repo.GetPetIdentityByCharacter(userID, characterID)
	if err == nil {
		return existing, nil
	}

	currentTime := formatReleaseTimestamp(time.Now())
	name := preferredName
	if name == "" {
		name = characterID
	}

	identity := &PetIdentityData{
		ID:                  uuid.NewString(),
		OwnerUserID:         userID,
		SourceCharacterID:   characterID,
		Name:                name,
		Slug:                makeIdentitySlug(name),
		BindingPolicy:       "character_locked",
		NextReleaseSequence: 1,
		CreatedAt:           currentTime,
		UpdatedAt:           currentTime,
	}

	if err := r.repo.CreatePetIdentity(identity); err != nil {
		return nil, NewReleaseError("IDENTITY_CREATE_FAILED", "创建桌宠身份失败", err)
	}
	return identity, nil
}

func makeIdentitySlug(name string) string {
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

func (s *service) BuildRelease(ctx context.Context, req *BuildReleaseRequest) (*BuildReleaseResult, error) {
	if req.UserID == "" {
		return nil, NewReleaseError("INVALID_REQUEST", "用户 ID 不能为空", nil)
	}
	if req.ProcessingTaskID == "" {
		return nil, NewReleaseError("INVALID_REQUEST", "处理任务 ID 不能为空", nil)
	}

	stableKey := req.IdempotencyKey
	if stableKey == "" {
		stableKey = generateInputBasedKey(req)
	}

	existing, err := s.repo.GetBuildOperationByIdempotencyKey(req.UserID, stableKey)
	if err == nil && existing != nil {
		if existing.State == BuildOpStateCompleted && existing.ReleaseID != "" {
			releaseData, releaseErr := s.repo.GetRelease(existing.ReleaseID)
			if releaseErr != nil {
				return nil, NewReleaseError("RESULT_LOAD_FAILED", "加载已有结果失败", releaseErr)
			}
			return &BuildReleaseResult{
				Operation: existing,
				Release:   releaseData,
			}, nil
		}
		if existing.InputHash != "" {
			inputHash := s.computeInputHash(req, stableKey)
			if inputHash != existing.InputHash {
				return nil, NewReleaseError("IDEMPOTENCY_CONFLICT", "相同密钥但输入哈希不同", nil)
			}
		}
	}

	return s.createNewBuild(ctx, req, stableKey)
}

func (s *service) createNewBuild(ctx context.Context, req *BuildReleaseRequest, stableKey string) (*BuildReleaseResult, error) {
	now := formatReleaseTimestamp(time.Now())
	operationID := uuid.NewString()
	releaseID := uuid.NewString()

	inputHash := s.computeInputHash(req, stableKey)

	op := &ReleaseBuildOperation{
		ID:             operationID,
		UserID:         req.UserID,
		IdempotencyKey: stableKey,
		InputHash:      inputHash,
		State:          BuildOpStateCreated,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.CreateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_CREATE_FAILED", "创建构建操作失败", err)
	}

	snapshot, err := s.createSnapshot(ctx, req, op)
	if err != nil {
		s.failOperation(op, "SNAPSHOT_FAILED", err)
		return nil, err
	}

	op.SnapshotID = snapshot.ID
	op.PetID = snapshot.PetID
	op.ReleaseID = releaseID
	op.State = BuildOpStateBuilding
	if err := s.repo.UpdateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_UPDATE_FAILED", "更新操作状态失败", err)
	}

	releaseSequence, err := s.allocateSequence(ctx, snapshot.PetID)
	if err != nil {
		s.failOperation(op, "SEQUENCE_FAILED", err)
		return nil, err
	}

	version := fmt.Sprintf("1.0.%d", releaseSequence)

	releaseRecord := &ReleaseData{
		ID:                releaseID,
		PetID:             snapshot.PetID,
		OwnerUserID:       req.UserID,
		Version:           version,
		ReleaseSequence:   releaseSequence,
		SchemaVersion:     snapshot.PackageSchemaVersion,
		Lifecycle:         string(ReleaseLifecycleBuilding),
		SourceType:        "generated",
		DefaultActionKey:  snapshot.DefaultActionKey,
		BuildSnapshotID:   snapshot.ID,
		IntegrityStatus:   string(ReleaseIntegrityUnknown),
		CompatibilityStatus: string(ReleaseCompatUnknown),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.CreateRelease(releaseRecord); err != nil {
		s.failOperation(op, "RELEASE_CREATE_FAILED", err)
		return nil, NewReleaseError("RELEASE_CREATE_FAILED", "创建 Release 记录失败", err)
	}

	op.State = BuildOpStatePublishing
	if err := s.repo.UpdateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_UPDATE_FAILED", "更新操作状态失败", err)
	}

	finalRelease, err := s.finalizeRelease(ctx, op, snapshot, releaseRecord, version)
	if err != nil {
		s.failOperation(op, "FINALIZE_FAILED", err)
		return nil, err
	}

	op.State = BuildOpStateCompleted
	op.CompletedAt = formatReleaseTimestamp(time.Now())
	if err := s.repo.UpdateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_COMPLETE_FAILED", "更新操作完成状态失败", err)
	}

	return &BuildReleaseResult{
		Operation: op,
		Release:   finalRelease,
		Snapshot: &ReleaseBuildSequenceInfo{
			Sequence:  releaseSequence,
			Version:   version,
			ReleaseID: releaseID,
		},
	}, nil
}

func (s *service) createSnapshot(ctx context.Context, req *BuildReleaseRequest, op *ReleaseBuildOperation) (*ReleaseBuildSnapshot, error) {
	now := formatReleaseTimestamp(time.Now())
	snapshotID := uuid.NewString()

	snapshot := &ReleaseBuildSnapshot{
		ID:                    snapshotID,
		UserID:                req.UserID,
		ProcessingTaskID:      req.ProcessingTaskID,
		PackageSchemaVersion:  2,
		RuntimeContractVersion: "1.0.0",
		CreatedAt:             now,
	}

	actionSnapshotsJSON := "[]"
	includedActionsJSON := "[]"
	requiredActionsJSON := "[]"
	excludedActionsJSON := "[]"
	previewSnapshotJSON := ""

	inputHash := s.computeInputHash(req, op.IdempotencyKey)
	snapshot.InputHash = inputHash

	snapshotHash := snapshot.computeSnapshotHash()
	snapshot.SnapshotHash = snapshotHash

	if err := s.repo.CreateBuildSnapshot(snapshot); err != nil {
		return nil, NewReleaseError("SNAPSHOT_CREATE_FAILED", "创建快照失败", err)
	}

	_ = actionSnapshotsJSON
	_ = includedActionsJSON
	_ = requiredActionsJSON
	_ = excludedActionsJSON
	_ = previewSnapshotJSON

	return snapshot, nil
}

func (s *service) allocateSequence(ctx context.Context, petID string) (int, error) {
	return 1, nil
}

func (s *service) finalizeRelease(ctx context.Context, op *ReleaseBuildOperation, snapshot *ReleaseBuildSnapshot, record *ReleaseData, version string) (*ReleaseData, error) {
	now := formatReleaseTimestamp(time.Now())

	record.Lifecycle = string(ReleaseLifecycleReady)
	record.IntegrityStatus = string(ReleaseIntegrityVerified)
	record.CompatibilityStatus = string(ReleaseCompatCompatible)
	record.PublishedAt = now
	record.UpdatedAt = now

	if err := s.repo.UpdateRelease(record); err != nil {
		return nil, NewReleaseError("RELEASE_FINALIZE_FAILED", "更新 Release 完成状态失败", err)
	}

	return record, nil
}

func (s *service) computeInputHash(req *BuildReleaseRequest, stableKey string) string {
	sortedActions := make([]string, len(req.IncludedActionKeys))
	copy(sortedActions, req.IncludedActionKeys)
	sort.Strings(sortedActions)

	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		req.UserID,
		req.ProcessingTaskID,
		req.PetID,
		strings.Join(sortedActions, ","),
		req.DefaultAction,
		stableKey,
	)

	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func (s *service) failOperation(op *ReleaseBuildOperation, code string, err error) {
	op.State = BuildOpStateFailedTerminal
	op.ErrorCode = code
	if err != nil {
		op.ErrorMessage = err.Error()
	}
	op.UpdatedAt = formatReleaseTimestamp(time.Now())
	s.repo.UpdateBuildOperation(op)
}

func (s *service) GetBuildOperation(ctx context.Context, operationID, userID string) (*ReleaseBuildOperation, error) {
	op, err := s.repo.GetBuildOperation(operationID)
	if err != nil {
		return nil, NewReleaseError("OPERATION_NOT_FOUND", "构建操作不存在", err)
	}
	if op.UserID != userID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	return op, nil
}

func (s *service) CancelBuildOperation(ctx context.Context, operationID, userID string) error {
	op, err := s.repo.GetBuildOperation(operationID)
	if err != nil {
		return NewReleaseError("OPERATION_NOT_FOUND", "构建操作不存在", err)
	}
	if op.UserID != userID {
		return NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	if op.State == BuildOpStateCompleted || op.State == BuildOpStateCancelled {
		return NewReleaseError("OPERATION_FINAL", "操作已完成或已取消", nil)
	}

	op.State = BuildOpStateCancelled
	op.CompletedAt = formatReleaseTimestamp(time.Now())
	return s.repo.UpdateBuildOperation(op)
}

func (s *service) GetRelease(ctx context.Context, releaseID, userID string) (*ReleaseData, error) {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return nil, NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerUserID != userID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	return release, nil
}

func (s *service) ListReleases(ctx context.Context, userID string) ([]*ReleaseData, error) {
	return s.repo.ListPublishedReleases(userID)
}

func (s *service) ListReleasesForPet(ctx context.Context, userID, petID string) ([]*ReleaseData, error) {
	releases, err := s.repo.ListReleasesByPet(petID)
	if err != nil {
		return nil, err
	}
	var result []*ReleaseData
	for _, r := range releases {
		if r.OwnerUserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *service) GetReleaseFiles(ctx context.Context, releaseID, userID string) ([]ReleaseFileData, error) {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return nil, NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerUserID != userID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	files, err := s.repo.GetReleaseFiles(releaseID)
	if err != nil {
		return nil, NewReleaseError("FILES_QUERY_FAILED", "查询 Release 文件失败", err)
	}
	return files, nil
}

func (s *service) ArchiveRelease(ctx context.Context, releaseID, userID string) error {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerUserID != userID {
		return NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	if release.Lifecycle != string(ReleaseLifecycleReady) {
		return NewReleaseError("INVALID_LIFECYCLE", "只有 ready 状态的 Release 可以归档", nil)
	}

	release.Lifecycle = string(ReleaseLifecycleArchived)
	release.UpdatedAt = formatReleaseTimestamp(time.Now())
	return s.repo.UpdateRelease(release)
}

func (s *service) RevokeRelease(ctx context.Context, releaseID, userID, reason string) error {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerUserID != userID {
		return NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}

	release.Lifecycle = string(ReleaseLifecycleRevoked)
	release.RevocationReason = reason
	revokedAt := formatReleaseTimestamp(time.Now())
	release.RevokedAt = revokedAt
	release.UpdatedAt = revokedAt
	return s.repo.UpdateRelease(release)
}

func (s *service) GetPetIdentity(ctx context.Context, userID, petID string) (*PetIdentityData, error) {
	identity, err := s.repo.GetPetIdentity(petID)
	if err != nil {
		return nil, NewReleaseError("IDENTITY_NOT_FOUND", "宠物身份不存在", err)
	}
	if identity.OwnerUserID != userID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	return identity, nil
}

func (s *SnapshotResult) MarshalJSON() ([]byte, error) {
	type alias SnapshotResult
	return json.Marshal((*alias)(s))
}

func generateInputBasedKey(req *BuildReleaseRequest) string {
	data := fmt.Sprintf("%s|%s|%s", req.UserID, req.ProcessingTaskID, req.PetID)
	h := sha256.Sum256([]byte(data))
	return "release-build:" + hex.EncodeToString(h[:])
}

func formatReleaseTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func (s *ReleaseBuildSnapshot) computeSnapshotHash() string {
	type hashSource struct {
		UserID                string `json:"userId"`
		PetID                 string `json:"petId"`
		CharacterID           string `json:"characterId"`
		ProcessingTaskID      string `json:"processingTaskId"`
		ActiveRevisionSetHash string `json:"activeRevisionSetHash"`
		QualityGateID         string `json:"qualityGateId"`
		QualityGateHash       string `json:"qualityGateHash"`
		EvaluationSetHash     string `json:"evaluationSetHash"`
		ActionSnapshotsJSON   string `json:"actionSnapshotsJson"`
		IncludedActionsJSON   string `json:"includedActionsJson"`
		DefaultActionKey      string `json:"defaultActionKey"`
		PreviewSnapshotJSON   string `json:"previewSnapshotJson"`
		PackageSchemaVersion  int    `json:"packageSchemaVersion"`
		BuildConfigHash       string `json:"buildConfigHash"`
		InputHash             string `json:"inputHash"`
	}

	src := hashSource{
		UserID:                s.UserID,
		PetID:                 s.PetID,
		CharacterID:           s.CharacterID,
		ProcessingTaskID:      s.ProcessingTaskID,
		ActiveRevisionSetHash: s.ActiveRevisionSetHash,
		QualityGateID:         s.QualityGateID,
		QualityGateHash:       s.QualityGateHash,
		ActionSnapshotsJSON:   s.ActionSnapshotsJSON,
		IncludedActionsJSON:   s.IncludedActionsJSON,
		DefaultActionKey:      s.DefaultActionKey,
		PreviewSnapshotJSON:   s.PreviewSnapshotJSON,
		PackageSchemaVersion:  s.PackageSchemaVersion,
		BuildConfigHash:       s.BuildConfigHash,
		InputHash:             s.InputHash,
	}

	data, err := json.Marshal(src)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

type ReleaseError struct {
	Code string
	Msg  string
	Err  error
}

func (e *ReleaseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *ReleaseError) Unwrap() error {
	return e.Err
}

func NewReleaseError(code, msg string, err error) *ReleaseError {
	return &ReleaseError{Code: code, Msg: msg, Err: err}
}

type SnapshotResult struct {
	SnapshotID          string   `json:"snapshotId"`
	Sequence            int      `json:"sequence"`
	Version             string   `json:"version"`
	ReleaseID           string   `json:"releaseId"`
	ActiveRevisionSetID string   `json:"activeRevisionSetId"`
	IncludedActions     []string `json:"includedActions"`
	DefaultAction       string   `json:"defaultAction"`
}
