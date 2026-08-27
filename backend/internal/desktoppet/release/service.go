package release

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"gorm.io/gorm"
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
	GetReleaseArchivePath(ctx context.Context, releaseID, userID string) (string, *ReleaseData, error)
}

type GeneratedPackageSourceResult struct {
	PackageID    string
	PackageHash  string
	PackageDir   string
	ManifestData []byte
	Ephemeral    bool
}

type GeneratedPackageSource interface {
	BuildGeneratedPackage(ctx context.Context, userID, processingTaskID, defaultAction string, includedActions []string) (*GeneratedPackageSourceResult, error)
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
	Operation *ReleaseBuildOperation
	Release   *ReleaseData
	Snapshot  *ReleaseBuildSequenceInfo
}

type ReleaseBuildSequenceInfo struct {
	Sequence  int
	Version   string
	ReleaseID string
}

type service struct {
	repo           ReleaseRepository
	identitySvc    *PetIdentityResolver
	gateReader     ReleaseQualityGateReader
	storage        ReleaseStoragePort
	eventPublisher EventPublisher
	packageSource  GeneratedPackageSource
}

func NewReleaseService(
	repo ReleaseRepository,
	gateReader ReleaseQualityGateReader,
	storage ReleaseStoragePort,
	eventPublisher EventPublisher,
	packageSource GeneratedPackageSource,
) ReleaseService {
	return &service{
		repo:           repo,
		identitySvc:    &PetIdentityResolver{repo: repo},
		gateReader:     gateReader,
		storage:        storage,
		eventPublisher: eventPublisher,
		packageSource:  packageSource,
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
	if s.packageSource == nil {
		return nil, NewReleaseError("PACKAGE_SOURCE_UNAVAILABLE", "Release 构建源未配置", nil)
	}

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

	op.State = BuildOpStateSnapshotting
	_ = s.repo.UpdateBuildOperation(op)

	gate, err := s.loadReleaseQualityGate(ctx, req.UserID, req.ProcessingTaskID)
	if err != nil {
		s.failOperation(op, "QUALITY_GATE_READ_FAILED", err)
		return nil, err
	}
	selectedActions, err := validateReleaseActionSelection(gate, req.IncludedActionKeys, req.DefaultAction)
	if err != nil {
		s.failOperation(op, "QUALITY_GATE_REJECTED", err)
		return nil, err
	}
	buildReq := *req
	buildReq.IncludedActionKeys = selectedActions
	req = &buildReq

	generated, err := s.packageSource.BuildGeneratedPackage(ctx, req.UserID, req.ProcessingTaskID, req.DefaultAction, req.IncludedActionKeys)
	if err != nil {
		s.failOperation(op, "PACKAGE_BUILD_FAILED", err)
		return nil, NewReleaseError("PACKAGE_BUILD_FAILED", "生成 Release 源包失败", err)
	}
	if generated.Ephemeral && strings.TrimSpace(generated.PackageDir) != "" {
		defer os.RemoveAll(generated.PackageDir)
	}

	legacyManifest, err := (&packageformat.V1Reader{}).ReadManifest(generated.ManifestData)
	if err != nil {
		s.failOperation(op, "SOURCE_MANIFEST_INVALID", err)
		return nil, NewReleaseError("SOURCE_MANIFEST_INVALID", "源包 manifest 无法转换为 V2", err)
	}
	characterID := strings.TrimSpace(legacyManifest.Binding.SourceCharacterID)
	if characterID == "" {
		characterID = strings.TrimSpace(req.CharacterID)
	}
	if characterID == "" {
		err := errors.New("source character id is empty")
		s.failOperation(op, "CHARACTER_ID_MISSING", err)
		return nil, NewReleaseError("CHARACTER_ID_MISSING", "无法确定桌宠绑定角色", err)
	}
	if req.CharacterID != "" && req.CharacterID != characterID {
		err := fmt.Errorf("request character %s does not match source character %s", req.CharacterID, characterID)
		s.failOperation(op, "CHARACTER_ID_MISMATCH", err)
		return nil, NewReleaseError("CHARACTER_ID_MISMATCH", "桌宠角色绑定与处理任务不一致", err)
	}

	var identity *PetIdentityData
	if req.PetID != "" {
		identity, err = s.repo.GetPetIdentity(req.PetID)
		if err != nil {
			s.failOperation(op, "PET_IDENTITY_NOT_FOUND", err)
			return nil, NewReleaseError("PET_IDENTITY_NOT_FOUND", "指定桌宠身份不存在", err)
		}
		if identity.OwnerUserID != req.UserID || (identity.SourceCharacterID != "" && identity.SourceCharacterID != characterID) {
			err := errors.New("pet identity ownership or binding mismatch")
			s.failOperation(op, "OWNERSHIP_DENIED", err)
			return nil, NewReleaseError("OWNERSHIP_DENIED", "桌宠身份与当前用户或角色不匹配", err)
		}
	} else {
		identity, err = s.identitySvc.ResolveOrCreate(ctx, req.UserID, characterID, legacyManifest.Name)
		if err != nil {
			s.failOperation(op, "IDENTITY_CREATE_FAILED", err)
			return nil, err
		}
	}

	snapshot, err := s.createSnapshot(ctx, req, op, identity, legacyManifest, gate)
	if err != nil {
		s.failOperation(op, "SNAPSHOT_FAILED", err)
		return nil, err
	}
	op.SnapshotID = snapshot.ID
	op.PetID = identity.ID
	op.ReleaseID = releaseID
	op.State = BuildOpStateBuilding
	if err := s.repo.UpdateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_UPDATE_FAILED", "更新操作状态失败", err)
	}

	releaseSequence, err := s.allocateSequence(ctx, identity.ID)
	if err != nil {
		s.failOperation(op, "SEQUENCE_FAILED", err)
		return nil, err
	}
	version := fmt.Sprintf("1.0.%d", releaseSequence)

	record := &ReleaseData{
		ID:                    releaseID,
		PetID:                 identity.ID,
		OwnerUserID:           req.UserID,
		Version:               version,
		ReleaseSequence:       releaseSequence,
		SchemaVersion:         packageformat.ManifestSchemaVersion,
		Lifecycle:             string(ReleaseLifecycleBuilding),
		SourceType:            "generated",
		SourceProcessingTask:  req.ProcessingTaskID,
		SourceGenerationTask:  legacyManifest.Provenance.GenerationTaskID,
		ActiveRevisionSetHash: gate.ActiveRevisionSetHash,
		QualityGateID:         gate.GateID,
		QualityGateHash:       gate.GateHash,
		EvaluationSetHash:     gate.EvaluationSetHash,
		DefaultActionKey:      legacyManifest.DefaultAction,
		BuildSnapshotID:       snapshot.ID,
		IntegrityStatus:       string(ReleaseIntegrityUnknown),
		CompatibilityStatus:   string(ReleaseCompatUnknown),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.repo.CreateRelease(record); err != nil {
		s.failOperation(op, "RELEASE_CREATE_FAILED", err)
		return nil, NewReleaseError("RELEASE_CREATE_FAILED", "创建 Release 记录失败", err)
	}

	op.State = BuildOpStatePublishing
	if err := s.repo.UpdateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_UPDATE_FAILED", "更新操作状态失败", err)
	}

	finalRelease, err := s.finalizeRelease(ctx, op, snapshot, record, version, identity, legacyManifest, generated)
	if err != nil {
		record.Lifecycle = string(ReleaseLifecycleFailed)
		record.UpdatedAt = formatReleaseTimestamp(time.Now())
		_ = s.repo.UpdateRelease(record)
		s.failOperation(op, "FINALIZE_FAILED", err)
		return nil, err
	}

	op.State = BuildOpStateCompleted
	op.CompletedAt = formatReleaseTimestamp(time.Now())
	op.PublishedPathKey = finalRelease.StorageKey
	resultData, _ := json.Marshal(map[string]any{"releaseId": finalRelease.ID, "version": finalRelease.Version, "contentRootHash": finalRelease.ContentRootHash})
	op.ResultJSON = string(resultData)
	if err := s.repo.UpdateBuildOperation(op); err != nil {
		return nil, NewReleaseError("OPERATION_COMPLETE_FAILED", "更新操作完成状态失败", err)
	}

	return &BuildReleaseResult{
		Operation: op,
		Release:   finalRelease,
		Snapshot:  &ReleaseBuildSequenceInfo{Sequence: releaseSequence, Version: version, ReleaseID: releaseID},
	}, nil
}

func (s *service) loadReleaseQualityGate(ctx context.Context, userID, processingTaskID string) (*QualityGateResult, error) {
	if s.gateReader == nil {
		return nil, NewReleaseError("QUALITY_GATE_UNAVAILABLE", "Release 质量门禁未配置", nil)
	}
	var activeRevisionSetHash string
	var gateSnapshotHash string
	if err := s.repo.Transaction(func(tx *gorm.DB) error {
		var row struct {
			ActiveRevisionSetHash string `gorm:"column:active_revision_set_hash"`
			SnapshotHash          string `gorm:"column:snapshot_hash"`
		}
		if err := tx.WithContext(ctx).Table("desktop_pet_quality_gate_results").
			Select("active_revision_set_hash, snapshot_hash").
			Where("processing_task_id = ? AND invalidated_at = ''", processingTaskID).
			Order("updated_at DESC").Take(&row).Error; err != nil {
			return err
		}
		activeRevisionSetHash = row.ActiveRevisionSetHash
		gateSnapshotHash = row.SnapshotHash
		return nil
	}); err != nil {
		return nil, NewReleaseError("QUALITY_GATE_MISSING", "没有可用于 Release 的质量门禁", err)
	}
	if strings.TrimSpace(activeRevisionSetHash) == "" {
		return nil, NewReleaseError("QUALITY_GATE_STALE", "质量门禁缺少 active revision set", nil)
	}
	gate, err := s.gateReader.GetValidGateForRelease(ctx, userID, processingTaskID, activeRevisionSetHash)
	if err != nil {
		return nil, NewReleaseError("QUALITY_GATE_READ_FAILED", "读取 Release 质量门禁失败", err)
	}
	if gate == nil {
		return nil, NewReleaseError("QUALITY_GATE_STALE", "质量门禁已失效或与当前修订不匹配", nil)
	}
	if !gate.GateStatus.IsAllowed() {
		return nil, NewReleaseError(gate.GateStatus.ErrorCode(), "质量门禁未通过: "+string(gate.GateStatus), nil)
	}
	if gate.GateHash == "" {
		gate.GateHash = gateSnapshotHash
	}
	return gate, nil
}

func validateReleaseActionSelection(gate *QualityGateResult, requested []string, defaultAction string) ([]string, error) {
	if gate == nil {
		return nil, NewReleaseError("QUALITY_GATE_MISSING", "质量门禁为空", nil)
	}
	allowed := make(map[string]struct{}, len(gate.IncludedActionKeys))
	for _, key := range gate.IncludedActionKeys {
		allowed[key] = struct{}{}
	}
	selected := append([]string(nil), requested...)
	if len(selected) == 0 {
		selected = append(selected, gate.IncludedActionKeys...)
	}
	selectedSet := make(map[string]struct{}, len(selected))
	for _, key := range selected {
		if _, ok := allowed[key]; !ok {
			return nil, NewReleaseError("QUALITY_GATE_ACTION_REJECTED", "动作未通过质量门禁: "+key, nil)
		}
		selectedSet[key] = struct{}{}
	}
	for _, key := range gate.RequiredActionKeys {
		if _, ok := selectedSet[key]; !ok {
			return nil, NewReleaseError("QUALITY_GATE_REQUIRED_ACTION_MISSING", "不能排除必需动作: "+key, nil)
		}
	}
	if defaultAction != "" {
		if _, ok := selectedSet[defaultAction]; !ok {
			return nil, NewReleaseError("RELEASE_DEFAULT_ACTION_INVALID", "默认动作不在 Release 动作集合中: "+defaultAction, nil)
		}
	}
	sort.Strings(selected)
	return selected, nil
}

func (s *service) createSnapshot(ctx context.Context, req *BuildReleaseRequest, op *ReleaseBuildOperation, identity *PetIdentityData, source *packageformat.Manifest, gate *QualityGateResult) (*ReleaseBuildSnapshot, error) {
	now := formatReleaseTimestamp(time.Now())
	included := make([]string, 0, len(source.Actions))
	for _, action := range source.Actions {
		included = append(included, action.Key)
	}
	includedJSON, _ := json.Marshal(included)
	gateJSON, _ := json.Marshal(gate)
	snapshot := &ReleaseBuildSnapshot{
		ID:                     uuid.NewString(),
		UserID:                 req.UserID,
		PetID:                  identity.ID,
		CharacterID:            identity.SourceCharacterID,
		ProcessingTaskID:       req.ProcessingTaskID,
		ActiveRevisionSetHash:  gate.ActiveRevisionSetHash,
		QualityGateID:          gate.GateID,
		QualityGateHash:        gate.GateHash,
		QualityGateJSON:        string(gateJSON),
		DefaultActionKey:       source.DefaultAction,
		IncludedActionsJSON:    string(includedJSON),
		RequiredActionsJSON:    "[]",
		ExcludedActionsJSON:    "[]",
		ActionSnapshotsJSON:    "[]",
		PackageSchemaVersion:   packageformat.ManifestSchemaVersion,
		RuntimeContractVersion: contracts.RuntimeContractVersion,
		InputHash:              s.computeInputHash(req, op.IdempotencyKey),
		CreatedAt:              now,
	}
	snapshot.SnapshotHash = snapshot.computeSnapshotHash()
	if err := s.repo.CreateBuildSnapshot(snapshot); err != nil {
		return nil, NewReleaseError("SNAPSHOT_CREATE_FAILED", "创建快照失败", err)
	}
	return snapshot, nil
}

func (s *service) allocateSequence(ctx context.Context, petID string) (int, error) {
	var sequence int
	err := s.repo.Transaction(func(tx *gorm.DB) error {
		var current int
		row := tx.WithContext(ctx).Table("desktop_pet_identities").
			Where("id = ?", petID).
			Select("next_release_sequence").Row()
		if err := row.Scan(&current); err != nil {
			return err
		}
		sequence = current
		if sequence < 1 {
			sequence = 1
		}
		result := tx.WithContext(ctx).Table("desktop_pet_identities").
			Where("id = ? AND next_release_sequence = ?", petID, current).
			Update("next_release_sequence", sequence+1)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("release sequence allocation conflict")
		}
		return nil
	})
	if err != nil {
		return 0, NewReleaseError("SEQUENCE_FAILED", "分配 Release 序号失败", err)
	}
	return sequence, nil
}

func (s *service) finalizeRelease(ctx context.Context, op *ReleaseBuildOperation, snapshot *ReleaseBuildSnapshot, record *ReleaseData, version string, identity *PetIdentityData, source *packageformat.Manifest, generated *GeneratedPackageSourceResult) (*ReleaseData, error) {
	if err := s.storage.RemoveStagingDir(record.ID); err != nil {
		return nil, NewReleaseError("STAGING_CLEANUP_FAILED", "清理 Release 暂存目录失败", err)
	}
	if err := s.storage.EnsureStagingDir(record.ID); err != nil {
		return nil, NewReleaseError("STAGING_CREATE_FAILED", "创建 Release 暂存目录失败", err)
	}
	stagingDir, err := s.storage.StagingDir(record.ID)
	if err != nil {
		return nil, NewReleaseError("STAGING_PATH_FAILED", "解析 Release 暂存目录失败", err)
	}
	if err := copyPackageTree(generated.PackageDir, stagingDir); err != nil {
		_ = s.storage.RemoveStagingDir(record.ID)
		return nil, NewReleaseError("PACKAGE_COPY_FAILED", "复制 Release 文件失败", err)
	}
	_ = os.Remove(filepath.Join(stagingDir, "manifest.json"))

	manifest := *source
	manifest.SchemaVersion = packageformat.ManifestSchemaVersion
	manifest.ManifestFormat = packageformat.ManifestFormatCanonical
	manifest.PetID = identity.ID
	manifest.ReleaseID = record.ID
	manifest.Version = version
	manifest.Name = identity.Name
	if manifest.Name == "" {
		manifest.Name = source.Name
	}
	manifest.Author = packageformat.ManifestAuthor{Name: "Amitia User", ID: record.OwnerUserID}
	manifest.License = packageformat.ManifestLicense{SPDX: "AGPL-3.0-only"}
	manifest.Compatibility = packageformat.ManifestCompatibility{MinRuntimeVersion: contracts.RuntimeVersion, RenderMode: packageformat.RenderModeSprite}
	manifest.Binding = packageformat.ManifestBinding{Policy: packageformat.BindingPolicyBound, SourceCharacterID: identity.SourceCharacterID}
	manifest.Canvas.CoordinateSystem = packageformat.CoordinateSystemTopLeft
	manifest.Capabilities = packageformat.ManifestCapabilities{TransparentBackground: true, FrameSequence: true, PerFrameDuration: true, Audio: false}
	manifest.Provenance.SourceType = "generated"
	manifest.Provenance.ProcessingTaskID = snapshot.ProcessingTaskID
	manifest.Provenance.BuiltAt = time.Now().UTC().Format(time.RFC3339)
	manifest.Provenance.Builder = "amitia-release-v2"

	for i := range manifest.Actions {
		a := &manifest.Actions[i]
		cfgPath := filepath.Join(stagingDir, filepath.FromSlash(a.Config))
		meta, err := convertLegacyActionConfig(cfgPath, a.Key, a.Name)
		if err != nil {
			_ = s.storage.RemoveStagingDir(record.ID)
			return nil, NewReleaseError("ACTION_CONFIG_CONVERT_FAILED", "转换动作配置失败: "+a.Key, err)
		}
		a.RevisionID = "generated:" + generated.PackageID
		a.QualityVerdict = packageformat.QualityVerdictAccepted
		a.PlaybackMode = meta.PlaybackMode
		a.FPS = meta.FPS
		a.FrameCount = meta.FrameCount
		a.SupportsDefaultIdle = a.Key == manifest.DefaultAction
		a.IsStableStateCandidate = a.SupportsDefaultIdle || strings.HasPrefix(a.Key, "idle_") || strings.HasPrefix(a.Key, "sleep_")
	}

	fileManifest, err := packageformat.BuildFileManifestFromDir(stagingDir)
	if err != nil {
		_ = s.storage.RemoveStagingDir(record.ID)
		return nil, NewReleaseError("FILE_MANIFEST_FAILED", "计算 Release 文件清单失败", err)
	}
	manifest.Integrity = packageformat.ManifestIntegrity{Algorithm: packageformat.IntegrityAlgorithmV2, Files: fileManifest.Entries}
	for _, f := range fileManifest.Entries {
		manifest.Integrity.TotalBytes += f.Bytes
	}
	manifest.Integrity.FileCount = len(fileManifest.Entries)
	finalManifest, manifestData, err := (&packageformat.V2Writer{}).FinalizeManifest(&manifest)
	if err != nil {
		_ = s.storage.RemoveStagingDir(record.ID)
		return nil, NewReleaseError("MANIFEST_FINALIZE_FAILED", "生成 V2 manifest 失败", err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "manifest.json"), manifestData, 0o644); err != nil {
		_ = s.storage.RemoveStagingDir(record.ID)
		return nil, NewReleaseError("MANIFEST_WRITE_FAILED", "写入 V2 manifest 失败", err)
	}
	validation := packageformat.NewValidator().ValidateDirectory(stagingDir, finalManifest)
	if validation.Verdict != "valid" || validation.ErrorCount > 0 {
		_ = s.storage.RemoveStagingDir(record.ID)
		return nil, NewReleaseError("RELEASE_VALIDATION_FAILED", fmt.Sprintf("V2 Release 校验失败: %d errors", validation.ErrorCount), nil)
	}
	if err := s.storage.MoveStagingToPublished(identity.ID, record.ID); err != nil {
		_ = s.storage.RemoveStagingDir(record.ID)
		return nil, NewReleaseError("PUBLISH_FAILED", "发布 Release 文件失败", err)
	}
	publishedDir, err := s.storage.PublishedDir(identity.ID, record.ID)
	if err != nil {
		return nil, NewReleaseError("PUBLISHED_PATH_FAILED", "解析发布目录失败", err)
	}
	archivePath, err := s.storage.ArchivePath(identity.ID, record.ID)
	if err != nil {
		return nil, NewReleaseError("ARCHIVE_PATH_FAILED", "解析 Release 归档路径失败", err)
	}
	archiveHash, archiveBytes, err := buildZipArchive(publishedDir, archivePath)
	if err != nil {
		_ = s.storage.RemovePublishedDir(identity.ID, record.ID)
		return nil, NewReleaseError("ARCHIVE_BUILD_FAILED", "生成 Release 下载归档失败", err)
	}
	storageKey, err := s.storage.PublishedStorageKey(identity.ID, record.ID)
	if err != nil {
		return nil, NewReleaseError("STORAGE_KEY_FAILED", "生成 Release 存储键失败", err)
	}
	archiveKey, err := s.storage.ArchiveStorageKey(identity.ID, record.ID)
	if err != nil {
		return nil, NewReleaseError("ARCHIVE_KEY_FAILED", "生成归档存储键失败", err)
	}

	now := formatReleaseTimestamp(time.Now())
	record.Lifecycle = string(ReleaseLifecycleReady)
	record.IntegrityStatus = string(ReleaseIntegrityVerified)
	record.CompatibilityStatus = string(ReleaseCompatCompatible)
	record.ContentRootHash = finalManifest.Integrity.ContentRootHash
	record.ManifestHash = finalManifest.Integrity.ManifestHash
	record.StorageKey = storageKey
	record.ArchiveStorageKey = archiveKey
	record.ArchiveHash = archiveHash
	record.ArchiveBytes = archiveBytes
	record.TotalBytes = finalManifest.Integrity.TotalBytes
	record.FileCount = finalManifest.Integrity.FileCount
	record.ActionCount = len(finalManifest.Actions)
	record.DefaultActionKey = finalManifest.DefaultAction
	record.MinRuntimeVersion = finalManifest.Compatibility.MinRuntimeVersion
	record.ManifestJSON = string(manifestData)
	record.PublishedAt = now
	record.UpdatedAt = now

	files := make([]ReleaseFileData, 0, len(finalManifest.Integrity.Files))
	for _, f := range finalManifest.Integrity.Files {
		files = append(files, ReleaseFileData{ID: uuid.NewString(), ReleaseID: record.ID, Path: f.Path, SHA256: f.SHA256, Bytes: f.Bytes, MediaType: f.MediaType, Role: f.Role, ActionKey: f.ActionKey, FrameID: f.FrameID, CreatedAt: now})
	}
	if err := s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.UpdateReleaseTx(tx, record); err != nil {
			return err
		}
		if len(files) > 0 {
			return s.repo.CreateReleaseFilesTx(tx, files)
		}
		return nil
	}); err != nil {
		_ = s.storage.RemovePublishedDir(identity.ID, record.ID)
		_ = os.Remove(archivePath)
		return nil, NewReleaseError("RELEASE_FINALIZE_FAILED", "提交 Release 元数据失败", err)
	}
	identity.DefaultActionKey = record.DefaultActionKey
	identity.UpdatedAt = now
	_ = s.repo.UpdatePetIdentity(identity)
	return record, nil
}

type convertedActionMetadata struct {
	PlaybackMode    string
	FPS, FrameCount int
}

type legacyActionConfig struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	LoopType        string `json:"loopType"`
	PlaybackMode    string `json:"playbackMode"`
	Fps             int    `json:"fps"`
	FrameDurationMs int    `json:"frameDurationMs"`
	Frames          []struct {
		Index      int    `json:"index"`
		File       string `json:"file"`
		DurationMs int    `json:"durationMs"`
	} `json:"frames"`
	Anchor struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"anchor"`
	ReturnAction string `json:"returnAction"`
	ReturnPolicy string `json:"returnPolicy"`
}

func convertLegacyActionConfig(path, actionKey, displayName string) (*convertedActionMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var legacy legacyActionConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	fps := legacy.Fps
	if fps <= 0 {
		fps = 8
	}
	mode := packageformat.NormalizePlaybackMode(legacy.PlaybackMode)
	if !packageformat.IsValidPlaybackMode(mode) {
		mode = packageformat.MapLegacyLoopType(legacy.LoopType)
	}
	if !packageformat.IsValidPlaybackMode(mode) {
		mode = packageformat.PlaybackModeLoop
	}
	frames := make([]map[string]any, 0, len(legacy.Frames))
	for idx, frame := range legacy.Frames {
		frameFile := frame.File
		if frameFile == "" {
			frameFile = fmt.Sprintf("frames/frame-%04d.png", idx+1)
		}
		abs := filepath.Join(filepath.Dir(path), filepath.FromSlash(frameFile))
		hash, err := hashFileSHA256(abs)
		if err != nil {
			return nil, err
		}
		duration := frame.DurationMs
		if duration < 8 {
			duration = legacy.FrameDurationMs
		}
		if duration < 8 {
			duration = maxInt(8, 1000/fps)
		}
		frames = append(frames, map[string]any{"index": idx, "frameId": fmt.Sprintf("%s-%04d", actionKey, idx+1), "file": frameFile, "durationMs": duration, "assetId": "generated:" + hash[:16], "contentHash": hash})
	}
	returnTo := map[string]any{"type": "previous"}
	if legacy.ReturnPolicy == "none" {
		returnTo = map[string]any{"type": "none"}
	}
	if legacy.ReturnAction != "" {
		returnTo = map[string]any{"type": "action", "actionKey": legacy.ReturnAction}
	}
	out := map[string]any{"schemaVersion": packageformat.ActionConfigSchemaVersion, "actionKey": actionKey, "displayName": displayName, "fps": fps, "playbackMode": mode, "frames": frames, "returnTo": returnTo, "anchor": map[string]any{"x": clamp01(legacy.Anchor.X), "y": clamp01(legacy.Anchor.Y), "coordinateSpace": "normalized_canvas"}}
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return nil, err
	}
	return &convertedActionMetadata{PlaybackMode: mode, FPS: fps, FrameCount: len(frames)}, nil
}

func copyPackageTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if filepath.Base(path) == "manifest.json" && filepath.Dir(path) == src {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, cpErr := io.Copy(out, in)
		closeErr := out.Close()
		if cpErr != nil {
			return cpErr
		}
		return closeErr
	})
}

func buildZipArchive(root, archivePath string) (string, int64, error) {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		return "", 0, err
	}
	tmp := archivePath + ".tmp"
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, err
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	if _, statErr := os.Stat(archivePath); statErr == nil {
		_ = os.Remove(tmp)
		return "", 0, fmt.Errorf("archive already exists")
	}
	if err := os.Rename(tmp, archivePath); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	hash, err := hashFileSHA256(archivePath)
	if err != nil {
		return "", 0, err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return "", 0, err
	}
	return hash, info.Size(), nil
}

func hashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func (s *service) GetReleaseArchivePath(ctx context.Context, releaseID, userID string) (string, *ReleaseData, error) {
	releaseData, err := s.GetRelease(ctx, releaseID, userID)
	if err != nil {
		return "", nil, err
	}
	if releaseData.ArchiveStorageKey == "" {
		return "", nil, NewReleaseError("ARCHIVE_NOT_READY", "Release 归档尚未生成", nil)
	}
	path, err := s.storage.ArchivePath(releaseData.PetID, releaseID)
	if err != nil {
		return "", nil, NewReleaseError("ARCHIVE_PATH_FAILED", "解析归档路径失败", err)
	}
	if _, err := os.Stat(path); err != nil {
		return "", nil, NewReleaseError("ARCHIVE_NOT_FOUND", "Release 归档不存在", err)
	}
	return path, releaseData, nil
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
