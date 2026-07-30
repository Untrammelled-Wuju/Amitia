package installation

import (
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type ReleaseService interface {
	BuildRelease(req *BuildReleaseRequest) (*BuildReleaseResult, error)
	ImportPackage(req *ImportPackageRequest) (*ImportPackageResult, error)
	ListReleases(userID string) ([]*PackageRelease, error)
	ListReleasesByPet(petID string) ([]*PackageRelease, error)
	GetRelease(releaseID string) (*PackageRelease, error)
	GetReleaseFiles(releaseID string) ([]ReleaseFile, error)
	ListPetIdentities(userID string) ([]*PetIdentity, error)
	GetPetIdentity(petID string) (*PetIdentity, error)
	InstallRelease(userID, petID, releaseID, characterID, idempotencyKey string) (*Installation, error)
	UpgradeInstallation(userID, installationID, targetReleaseID, idempotencyKey string) (*Installation, error)
	SwitchInstallation(userID, installationID, idempotencyKey string) error
	RepairInstallation(userID, installationID, idempotencyKey string) error
	UninstallInstallation(userID, installationID, idempotencyKey string) error
	RecoverPendingOperations() error
}

type releaseService struct {
	repo        Repository
	builder     *ReleaseBuilder
	importer    *ReleaseImporter
	coordinator *Coordinator
	storage     *ReleaseStorage
	packageRepo processing.Repository
	charRepo    character.Repository
	ctx         *app.AppContext
}

func NewReleaseService(
	repo Repository,
	storage *ReleaseStorage,
	source RevisionSource,
	packageRepo processing.Repository,
	charRepo character.Repository,
	notifier RuntimeNotifier,
	ctx *app.AppContext,
) ReleaseService {
	builder := NewReleaseBuilder(repo, storage, source)
	importer := NewReleaseImporter(repo, storage)
	coordinator := NewCoordinator(repo, storage, notifier)
	return &releaseService{
		repo:        repo,
		builder:     builder,
		importer:    importer,
		coordinator: coordinator,
		storage:     storage,
		packageRepo: packageRepo,
		charRepo:    charRepo,
		ctx:         ctx,
	}
}

func (s *releaseService) BuildRelease(req *BuildReleaseRequest) (*BuildReleaseResult, error) {
	if req.ProcessingTaskID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "处理任务 ID 为空", ErrInstallationInvalid)
	}
	if req.UserID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 为空", ErrInstallationInvalid)
	}
	return s.builder.BuildRelease(req)
}

func (s *releaseService) ImportPackage(req *ImportPackageRequest) (*ImportPackageResult, error) {
	if req.UserID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 为空", ErrInstallationInvalid)
	}
	if req.PackageDir == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "包目录为空", ErrInstallationInvalid)
	}
	return s.importer.ImportPackage(req)
}

func (s *releaseService) ListReleases(userID string) ([]*PackageRelease, error) {
	if userID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 为空", ErrInstallationInvalid)
	}
	return s.repo.ListPublishedReleases(userID)
}

func (s *releaseService) ListReleasesByPet(petID string) ([]*PackageRelease, error) {
	if petID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "宠物 ID 为空", ErrInstallationInvalid)
	}
	return s.repo.ListReleasesByPet(petID)
}

func (s *releaseService) GetRelease(releaseID string) (*PackageRelease, error) {
	if releaseID == "" {
		return nil, NewInstallationError(ErrCodeInstallationNotFound, "Release ID 为空", ErrInstallationNotFound)
	}
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return nil, NewInstallationError(ErrCodeInstallationNotFound, "Release 不存在", err)
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询 Release 失败", err)
	}
	return release, nil
}

func (s *releaseService) GetReleaseFiles(releaseID string) ([]ReleaseFile, error) {
	if releaseID == "" {
		return nil, NewInstallationError(ErrCodeInstallationNotFound, "Release ID 为空", ErrInstallationNotFound)
	}
	files, err := s.repo.GetReleaseFiles(releaseID)
	if err != nil {
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询 Release 文件失败", err)
	}
	return files, nil
}

func (s *releaseService) ListPetIdentities(userID string) ([]*PetIdentity, error) {
	if userID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "用户 ID 为空", ErrInstallationInvalid)
	}
	return s.repo.ListPetIdentitiesByUser(userID)
}

func (s *releaseService) GetPetIdentity(petID string) (*PetIdentity, error) {
	if petID == "" {
		return nil, NewInstallationError(ErrCodeInstallationNotFound, "宠物 ID 为空", ErrInstallationNotFound)
	}
	identity, err := s.repo.GetPetIdentity(petID)
	if err != nil {
		if errors.Is(err, ErrInstallationNotFound) {
			return nil, NewInstallationError(ErrCodeInstallationNotFound, "宠物身份不存在", err)
		}
		return nil, NewInstallationError(ErrCodeInstallationFailed, "查询宠物身份失败", err)
	}
	return identity, nil
}

func (s *releaseService) InstallRelease(userID, petID, releaseID, characterID, idempotencyKey string) (*Installation, error) {
	if userID == "" || petID == "" || releaseID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "参数缺失: userID/petID/releaseID 不能为空", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("install-%s-%s-%d", userID, releaseID, 0)
	}
	return s.coordinator.Install(userID, petID, releaseID, characterID, idempotencyKey)
}

func (s *releaseService) UpgradeInstallation(userID, installationID, targetReleaseID, idempotencyKey string) (*Installation, error) {
	if userID == "" || installationID == "" || targetReleaseID == "" {
		return nil, NewInstallationError(ErrCodeInstallationInvalid, "参数缺失", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("upgrade-%s-%s-%d", userID, installationID, 0)
	}
	return s.coordinator.Upgrade(userID, installationID, targetReleaseID, idempotencyKey)
}

func (s *releaseService) SwitchInstallation(userID, installationID, idempotencyKey string) error {
	if userID == "" || installationID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "参数缺失", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("switch-%s-%s-%d", userID, installationID, 0)
	}
	return s.coordinator.Switch(userID, installationID, idempotencyKey)
}

func (s *releaseService) RepairInstallation(userID, installationID, idempotencyKey string) error {
	if userID == "" || installationID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "参数缺失", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("repair-%s-%s-%d", userID, installationID, 0)
	}
	return s.coordinator.Repair(userID, installationID, idempotencyKey)
}

func (s *releaseService) UninstallInstallation(userID, installationID, idempotencyKey string) error {
	if userID == "" || installationID == "" {
		return NewInstallationError(ErrCodeInstallationInvalid, "参数缺失", ErrInstallationInvalid)
	}
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("uninstall-%s-%s-%d", userID, installationID, 0)
	}
	return s.coordinator.Uninstall(userID, installationID, idempotencyKey)
}

func (s *releaseService) RecoverPendingOperations() error {
	return s.coordinator.RecoverPendingOperations()
}

var _ = gorm.ErrRecordNotFound
var _ = character.Repository(nil)
