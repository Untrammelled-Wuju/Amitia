package migration

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type LegacyPackageMigrator struct {
	repo    release.ReleaseRepository
	builder LegacyBuilder
}

type LegacyBuilder interface {
	BuildFromLegacy(userID, legacyPackageID string, mapping *release.LegacyPackageMapping) (*LegacyBuildResult, error)
}

type LegacyBuildResult struct {
	PetID     string
	ReleaseID string
}

type MigrateLegacyRequest struct {
	UserID          string
	LegacyPackageID string
	IdempotencyKey  string
}

type MigrateLegacyResult struct {
	Operation *release.LegacyPackageMigrationOperation
	Mapping   *release.LegacyPackageMapping
	PetID     string
	ReleaseID string
}

func NewLegacyPackageMigrator(
	repo release.ReleaseRepository,
	builder LegacyBuilder,
) *LegacyPackageMigrator {
	return &LegacyPackageMigrator{
		repo:    repo,
		builder: builder,
	}
}

func (m *LegacyPackageMigrator) MigrateLegacy(req *MigrateLegacyRequest) (*MigrateLegacyResult, error) {
	if req.UserID == "" {
		return nil, release.NewReleaseError("INVALID_USER", "用户 ID 不能为空", nil)
	}
	if req.LegacyPackageID == "" {
		return nil, release.NewReleaseError("INVALID_PACKAGE", "旧 Package ID 不能为空", nil)
	}

	existing, err := m.repo.GetLegacyPackageMapping(req.LegacyPackageID)
	if err == nil && existing != nil {
		return m.loadExistingMapping(existing)
	}

	operation, err := m.createMigrationOperation(req)
	if err != nil {
		return nil, err
	}

	mapping, err := m.createLegacyMapping(req)
	if err != nil {
		m.failMigrationOperation(operation, "MAPPING_CREATE_FAILED", err)
		return nil, err
	}

	buildResult, err := m.builder.BuildFromLegacy(req.UserID, req.LegacyPackageID, mapping)
	if err != nil {
		m.failMigrationOperation(operation, "BUILD_FAILED", err)
		m.markMappingFailed(mapping, err)
		return nil, release.NewReleaseError("LEGACY_BUILD_FAILED", "从旧包构建失败", err)
	}

	mapping.MigratedPetID = buildResult.PetID
	mapping.MigratedReleaseID = buildResult.ReleaseID
	mapping.MigrationStatus = release.LegacyMigrationStatusMigrated
	mapping.MigrationOperationId = operation.ID
	if err := m.repo.UpdateLegacyPackageMapping(mapping); err != nil {
		m.failMigrationOperation(operation, "MAPPING_UPDATE_FAILED", err)
		return nil, err
	}

	operation.State = release.LegacyMigrationOpStateCompleted
	operation.CompletedAt = formatMigrationTimestamp()
	if err := m.repo.UpdateLegacyMigrationOperation(operation); err != nil {
		return nil, release.NewReleaseError("OPERATION_UPDATE_FAILED", "更新迁移操作失败", err)
	}

	return &MigrateLegacyResult{
		Operation: operation,
		Mapping:   mapping,
		PetID:     buildResult.PetID,
		ReleaseID: buildResult.ReleaseID,
	}, nil
}

func (m *LegacyPackageMigrator) createMigrationOperation(req *MigrateLegacyRequest) (*release.LegacyPackageMigrationOperation, error) {
	operation := &release.LegacyPackageMigrationOperation{
		ID:              uuid.NewString(),
		LegacyPackageID: req.LegacyPackageID,
		UserID:          req.UserID,
		State:           release.LegacyMigrationOpStatePending,
		StartedAt:       formatMigrationTimestamp(),
		UpdatedAt:       formatMigrationTimestamp(),
	}
	if err := m.repo.CreateLegacyMigrationOperation(operation); err != nil {
		return nil, release.NewReleaseError("OPERATION_CREATE_FAILED", "创建迁移操作失败", err)
	}
	return operation, nil
}

func (m *LegacyPackageMigrator) createLegacyMapping(req *MigrateLegacyRequest) (*release.LegacyPackageMapping, error) {
	mapping := &release.LegacyPackageMapping{
		ID:              uuid.NewString(),
		LegacyPackageID: req.LegacyPackageID,
		MigrationStatus: release.LegacyMigrationStatusPending,
		OwnerUserId:     req.UserID,
		CreatedAt:       formatMigrationTimestamp(),
		UpdatedAt:       formatMigrationTimestamp(),
	}
	if err := m.repo.CreateLegacyPackageMapping(mapping); err != nil {
		return nil, release.NewReleaseError("MAPPING_CREATE_FAILED", "创建旧包映射失败", err)
	}
	return mapping, nil
}

func (m *LegacyPackageMigrator) loadExistingMapping(mapping *release.LegacyPackageMapping) (*MigrateLegacyResult, error) {
	result := &MigrateLegacyResult{
		Mapping: mapping,
		PetID:   mapping.MigratedPetID,
	}

	opID := mapping.MigrationOperationId
	if opID == "" {
		opID = mapping.ID
	}

	operation, err := m.repo.GetLegacyMigrationOperation(opID)
	if err != nil {
		operation = &release.LegacyPackageMigrationOperation{
			ID:              mapping.ID,
			LegacyPackageID: mapping.LegacyPackageID,
			UserID:          mapping.OwnerUserId,
			State:           migrationStatusToOpState(mapping.MigrationStatus),
		}
	}
	result.Operation = operation

	if mapping.MigrationStatus == release.LegacyMigrationStatusMigrated {
		result.ReleaseID = mapping.MigratedReleaseID
	}

	return result, nil
}

func (m *LegacyPackageMigrator) failMigrationOperation(op *release.LegacyPackageMigrationOperation, code string, err error) {
	op.State = release.LegacyMigrationOpStateFailedTerm
	op.ErrorCode = code
	op.ErrorMessage = err.Error()
	op.CompletedAt = formatMigrationTimestamp()
	op.UpdatedAt = formatMigrationTimestamp()
	m.repo.UpdateLegacyMigrationOperation(op)
}

func (m *LegacyPackageMigrator) markMappingFailed(mapping *release.LegacyPackageMapping, err error) {
	mapping.MigrationStatus = release.LegacyMigrationStatusFailed
	mapping.ErrorMessage = err.Error()
	mapping.UpdatedAt = formatMigrationTimestamp()
	m.repo.UpdateLegacyPackageMapping(mapping)
}

func migrationStatusToOpState(status string) string {
	switch status {
	case release.LegacyMigrationStatusMigrated:
		return release.LegacyMigrationOpStateCompleted
	case release.LegacyMigrationStatusFailed:
		return release.LegacyMigrationOpStateFailedTerm
	default:
		return release.LegacyMigrationOpStatePending
	}
}

func formatMigrationTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (m *LegacyPackageMigrator) ListPendingMigrations(userID string) ([]*release.LegacyPackageMapping, error) {
	return m.repo.ListPendingLegacyMappings()
}

func (m *LegacyPackageMigrator) RetryMigration(operationID string) (*MigrateLegacyResult, error) {
	op, err := m.repo.GetLegacyMigrationOperation(operationID)
	if err != nil {
		return nil, release.NewReleaseError("OPERATION_NOT_FOUND", "迁移操作不存在", err)
	}

	if op.State != release.LegacyMigrationOpStateFailedRetry && op.State != release.LegacyMigrationOpStatePending {
		return nil, release.NewReleaseError("INVALID_STATE", fmt.Sprintf("当前状态 %s 不允许重试", op.State), nil)
	}

	_, err = m.repo.GetLegacyPackageMapping(op.LegacyPackageID)
	if err != nil {
		return nil, release.NewReleaseError("MAPPING_NOT_FOUND", "旧包映射不存在", err)
	}

	return m.MigrateLegacy(&MigrateLegacyRequest{
		UserID:          op.UserID,
		LegacyPackageID: op.LegacyPackageID,
	})
}
