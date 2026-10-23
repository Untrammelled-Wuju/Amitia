package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"gorm.io/gorm"
)

type LegacyPackageBuildRequest struct {
	SpaceID         string
	LegacyPackageID string
	PackageName     string
	PackageDir      string
	ManifestPath    string
	Version         int
	PetID           string
}

type LegacyPackageBuildResult struct {
	PetID   string
	Release *ReleaseData
}

type LegacyPackageBuildPort interface {
	MigrateLegacyPackage(context.Context, LegacyPackageBuildRequest) (*LegacyPackageBuildResult, error)
}

type legacyProcessingManifest struct {
	Name          string `json:"name"`
	DefaultAction string `json:"defaultAction"`
	Preview       string `json:"preview"`
	Canvas        struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"canvas"`
	Actions []struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		Config string `json:"config"`
	} `json:"actions"`
}

func (s *service) MigrateLegacyPackage(ctx context.Context, req LegacyPackageBuildRequest) (*LegacyPackageBuildResult, error) {
	if strings.TrimSpace(req.SpaceID) == "" || strings.TrimSpace(req.LegacyPackageID) == "" {
		return nil, NewReleaseError("INVALID_REQUEST", "旧桌宠包迁移缺少用户或包编号", nil)
	}
	sourceDir, err := filepath.Abs(strings.TrimSpace(req.PackageDir))
	if err != nil || sourceDir == "" {
		return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "旧桌宠包目录无效", err)
	}
	manifestPath, err := filepath.Abs(strings.TrimSpace(req.ManifestPath))
	if err != nil || !strings.HasPrefix(manifestPath, sourceDir+string(filepath.Separator)) {
		return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "旧桌宠包 manifest 路径无效", err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "读取旧桌宠包 manifest 失败", err)
	}
	var legacy legacyProcessingManifest
	if err := json.Unmarshal(manifestData, &legacy); err != nil {
		return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "解析旧桌宠包 manifest 失败", err)
	}
	if strings.TrimSpace(legacy.DefaultAction) == "" || len(legacy.Actions) == 0 {
		return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "旧桌宠包缺少默认动作或动作列表", nil)
	}

	actionKeys := make([]string, 0, len(legacy.Actions))
	manifestActions := make([]packageformat.ManifestActionEntry, 0, len(legacy.Actions))
	for _, action := range legacy.Actions {
		key := strings.TrimSpace(action.Key)
		config := strings.TrimSpace(action.Config)
		if key == "" || config == "" {
			return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "旧桌宠包含空动作 Key 或配置路径", nil)
		}
		actionKeys = append(actionKeys, key)
		manifestActions = append(manifestActions, packageformat.ManifestActionEntry{
			Key:    key,
			Name:   firstNonEmptyLegacy(action.Name, key),
			Config: config,
		})
	}

	identityID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("legacy-pet:"+req.SpaceID+":"+req.LegacyPackageID)).String()
	identity, err := s.repo.GetPetIdentity(identityID)
	if errors.Is(err, ErrPetIdentityNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		now := formatReleaseTimestamp(time.Now())
		name := firstNonEmptyLegacy(req.PackageName, legacy.Name, "Migrated Pet")
		identity = &PetIdentityData{
			ID:                  identityID,
			OwnerSpaceID:        req.SpaceID,
			Name:                name,
			Slug:                makeIdentitySlug(name),
			NextReleaseSequence: 1,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := s.repo.CreatePetIdentity(identity); err != nil {
			return nil, NewReleaseError("IDENTITY_CREATE_FAILED", "创建迁移桌宠身份失败", err)
		}
	} else if err != nil {
		return nil, NewReleaseError("IDENTITY_LOAD_FAILED", "读取迁移桌宠身份失败", err)
	}
	if strings.TrimSpace(req.PetID) != "" {
		identity, err = s.repo.GetPetIdentity(req.PetID)
		if err != nil {
			return nil, NewReleaseError("PET_IDENTITY_NOT_FOUND", "迁移目标桌宠身份不存在", err)
		}
		if identity.OwnerSpaceID != req.SpaceID {
			return nil, NewReleaseError("OWNERSHIP_DENIED", "迁移目标桌宠身份不属于当前用户", nil)
		}
	}

	manifestHash, err := hashFileSHA256(manifestPath)
	if err != nil {
		return nil, NewReleaseError("LEGACY_PACKAGE_INVALID", "计算旧桌宠包 manifest 哈希失败", err)
	}
	processingTaskID := "legacy-package:" + req.LegacyPackageID
	buildReq := &BuildReleaseRequest{
		SpaceID:            req.SpaceID,
		ProcessingTaskID:   processingTaskID,
		PetID:              identity.ID,
		DefaultAction:      legacy.DefaultAction,
		IncludedActionKeys: actionKeys,
		IdempotencyKey:     "legacy-package:" + req.LegacyPackageID,
	}
	now := formatReleaseTimestamp(time.Now())
	op := &ReleaseBuildOperation{
		ID:             uuid.NewString(),
		SpaceID:        req.SpaceID,
		IdempotencyKey: buildReq.IdempotencyKey,
		PetID:          identity.ID,
		State:          BuildOpStateBuilding,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	gate := &QualityGateResult{
		GateStatus:            GateStatusPassedWithWarn,
		GateID:                "legacy:" + req.LegacyPackageID,
		GateHash:              manifestHash,
		IncludedActionKeys:    actionKeys,
		RequiredActionKeys:    actionKeys,
		ActiveRevisionSetHash: manifestHash,
		EvaluationSetHash:     manifestHash,
		RuleSetVersion:        "legacy-import-v1",
		RuleSetContentHash:    manifestHash,
	}
	source := &packageformat.Manifest{
		SchemaVersion:  packageformat.ManifestSchemaVersion,
		ManifestFormat: packageformat.ManifestFormatCanonical,
		Name:           identity.Name,
		DefaultAction:  legacy.DefaultAction,
		Preview:        firstNonEmptyLegacy(legacy.Preview, "preview.png"),
		Canvas: packageformat.ManifestCanvas{
			Width:            legacy.Canvas.Width,
			Height:           legacy.Canvas.Height,
			CoordinateSystem: packageformat.CoordinateSystemTopLeft,
		},
		Actions: manifestActions,
	}
	releaseID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("legacy-release:"+req.SpaceID+":"+req.LegacyPackageID)).String()
	if existing, loadErr := s.repo.GetRelease(releaseID); loadErr == nil && existing != nil {
		if existing.Lifecycle == string(ReleaseLifecycleReady) {
			return &LegacyPackageBuildResult{PetID: identity.ID, Release: existing}, nil
		}
	}
	snapshot, err := s.createSnapshot(ctx, buildReq, op, identity, source, gate)
	if err != nil {
		return nil, err
	}
	record, loadErr := s.repo.GetRelease(releaseID)
	if loadErr != nil && !errors.Is(loadErr, gorm.ErrRecordNotFound) {
		return nil, NewReleaseError("RELEASE_LOAD_FAILED", "读取迁移 Release 记录失败", loadErr)
	}
	if errors.Is(loadErr, gorm.ErrRecordNotFound) || record == nil {
		sequence, err := s.allocateSequence(ctx, identity.ID)
		if err != nil {
			return nil, err
		}
		version := fmt.Sprintf("1.0.%d", sequence)
		record = &ReleaseData{
			ID:                    releaseID,
			PetID:                 identity.ID,
			OwnerSpaceID:          req.SpaceID,
			Version:               version,
			ReleaseSequence:       sequence,
			SchemaVersion:         packageformat.ManifestSchemaVersion,
			Lifecycle:             string(ReleaseLifecycleBuilding),
			SourceType:            "migrated",
			LegacyPackageID:       req.LegacyPackageID,
			LegacyVersion:         req.Version,
			ActiveRevisionSetHash: gate.ActiveRevisionSetHash,
			QualityGateID:         gate.GateID,
			QualityGateHash:       gate.GateHash,
			EvaluationSetHash:     gate.EvaluationSetHash,
			DefaultActionKey:      legacy.DefaultAction,
			BuildSnapshotID:       snapshot.ID,
			IntegrityStatus:       string(ReleaseIntegrityUnknown),
			CompatibilityStatus:   string(ReleaseCompatUnknown),
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		if err := s.repo.CreateRelease(record); err != nil {
			return nil, NewReleaseError("RELEASE_CREATE_FAILED", "创建迁移 Release 记录失败", err)
		}
	} else {
		record.Lifecycle = string(ReleaseLifecycleBuilding)
		record.BuildSnapshotID = snapshot.ID
		record.UpdatedAt = now
		if err := s.repo.UpdateRelease(record); err != nil {
			return nil, NewReleaseError("RELEASE_UPDATE_FAILED", "恢复迁移 Release 记录失败", err)
		}
	}
	generated := &GeneratedPackageSourceResult{
		PackageID:    req.LegacyPackageID,
		PackageHash:  manifestHash,
		PackageDir:   sourceDir,
		ManifestData: manifestData,
		Ephemeral:    false,
	}
	finalRelease, err := s.finalizeRelease(ctx, op, snapshot, record, record.Version, identity, source, generated)
	if err != nil {
		record.Lifecycle = string(ReleaseLifecycleFailed)
		record.UpdatedAt = formatReleaseTimestamp(time.Now())
		_ = s.repo.UpdateRelease(record)
		return nil, err
	}
	return &LegacyPackageBuildResult{PetID: identity.ID, Release: finalRelease}, nil
}

func firstNonEmptyLegacy(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var _ LegacyPackageBuildPort = (*service)(nil)
