package release

import (
	"context"
	"fmt"
	"os"
	"time"
)

type ReleaseService interface {
	GetRelease(ctx context.Context, releaseID, spaceID string) (*ReleaseData, error)
	ListReleases(ctx context.Context, spaceID string) ([]*ReleaseData, error)
	ListReleasesForPet(ctx context.Context, spaceID, petID string) ([]*ReleaseData, error)
	GetReleaseFiles(ctx context.Context, releaseID, spaceID string) ([]ReleaseFileData, error)
	ArchiveRelease(ctx context.Context, releaseID, spaceID string) error
	RevokeRelease(ctx context.Context, releaseID, spaceID, reason string) error
	GetPetIdentity(ctx context.Context, spaceID, petID string) (*PetIdentityData, error)
	GetReleaseArchivePath(ctx context.Context, releaseID, spaceID string) (string, *ReleaseData, error)
}

type service struct {
	repo           ReleaseRepository
	storage        ReleaseStoragePort
	eventPublisher EventPublisher
}

func NewReleaseService(
	repo ReleaseRepository,
	storage ReleaseStoragePort,
	eventPublisher EventPublisher,
) ReleaseService {
	return &service{
		repo:           repo,
		storage:        storage,
		eventPublisher: eventPublisher,
	}
}

func (s *service) GetRelease(ctx context.Context, releaseID, spaceID string) (*ReleaseData, error) {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return nil, NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerSpaceID != spaceID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	return release, nil
}

func (s *service) ListReleases(ctx context.Context, spaceID string) ([]*ReleaseData, error) {
	return s.repo.ListPublishedReleases(spaceID)
}

func (s *service) ListReleasesForPet(ctx context.Context, spaceID, petID string) ([]*ReleaseData, error) {
	releases, err := s.repo.ListReleasesByPet(petID)
	if err != nil {
		return nil, err
	}
	var result []*ReleaseData
	for _, item := range releases {
		if item.OwnerSpaceID == spaceID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *service) GetReleaseFiles(ctx context.Context, releaseID, spaceID string) ([]ReleaseFileData, error) {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return nil, NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerSpaceID != spaceID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	files, err := s.repo.GetReleaseFiles(releaseID)
	if err != nil {
		return nil, NewReleaseError("FILES_QUERY_FAILED", "查询 Release 文件失败", err)
	}
	return files, nil
}

func (s *service) ArchiveRelease(ctx context.Context, releaseID, spaceID string) error {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerSpaceID != spaceID {
		return NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	if release.Lifecycle != string(ReleaseLifecycleReady) {
		return NewReleaseError("INVALID_LIFECYCLE", "只有 ready 状态的 Release 可以归档", nil)
	}
	release.Lifecycle = string(ReleaseLifecycleArchived)
	release.UpdatedAt = formatReleaseTimestamp(time.Now())
	return s.repo.UpdateRelease(release)
}

func (s *service) RevokeRelease(ctx context.Context, releaseID, spaceID, reason string) error {
	release, err := s.repo.GetRelease(releaseID)
	if err != nil {
		return NewReleaseError("RELEASE_NOT_FOUND", "Release 不存在", err)
	}
	if release.OwnerSpaceID != spaceID {
		return NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	release.Lifecycle = string(ReleaseLifecycleRevoked)
	release.RevocationReason = reason
	revokedAt := formatReleaseTimestamp(time.Now())
	release.RevokedAt = revokedAt
	release.UpdatedAt = revokedAt
	return s.repo.UpdateRelease(release)
}

func (s *service) GetPetIdentity(ctx context.Context, spaceID, petID string) (*PetIdentityData, error) {
	identity, err := s.repo.GetPetIdentity(petID)
	if err != nil {
		return nil, NewReleaseError("IDENTITY_NOT_FOUND", "宠物身份不存在", err)
	}
	if identity.OwnerSpaceID != spaceID {
		return nil, NewReleaseError("OWNERSHIP_DENIED", "不属于当前用户", nil)
	}
	return identity, nil
}

func (s *service) GetReleaseArchivePath(ctx context.Context, releaseID, spaceID string) (string, *ReleaseData, error) {
	releaseData, err := s.GetRelease(ctx, releaseID, spaceID)
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

func formatReleaseTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
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
