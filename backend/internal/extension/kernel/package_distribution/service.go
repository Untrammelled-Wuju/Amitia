package package_distribution

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Config struct {
	RegistryURL           string
	RegistryPublicKey     string
	DataDir               string
	HostVersion           string
	RuntimeProfile        string
	Platform              string
	Architecture          string
	AllowPrivateDownloads bool
}

type Service struct {
	config    Config
	client    *RegistryClient
	downloads *DownloadManager
	configErr error

	mu       sync.RWMutex
	metadata map[string]ExtensionUpdateMetadata
}

func NewService(config Config) *Service {
	service := &Service{
		config:    config,
		client:    NewRegistryClient(strings.TrimSpace(config.RegistryURL)),
		downloads: NewDownloadManager(config.DataDir),
		metadata:  make(map[string]ExtensionUpdateMetadata),
	}
	if strings.TrimSpace(config.RegistryURL) == "" {
		service.configErr = ErrRegistryNotConfigured
		return service
	}
	if key := strings.TrimSpace(config.RegistryPublicKey); key != "" {
		if err := service.client.SetPublicKeyBase64(key); err != nil {
			service.configErr = err
		}
	} else {
		service.configErr = fmt.Errorf("%w: registry signing key is unavailable", ErrRegistryNotConfigured)
	}
	return service
}

func (s *Service) Configured() bool {
	return s != nil && s.configErr == nil
}

func (s *Service) ConfigError() error {
	if s == nil {
		return ErrRegistryNotConfigured
	}
	return s.configErr
}

func (s *Service) Check(ctx context.Context, extensionID, currentVersion string) (*ExtensionUpdateMetadata, error) {
	if s == nil {
		return nil, ErrRegistryNotConfigured
	}
	if s.configErr != nil {
		return nil, s.configErr
	}
	metadata, err := s.client.QueryExtension(ctx, strings.TrimSpace(extensionID))
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		return nil, nil
	}
	if err := metadata.Validate(); err != nil {
		return nil, &UpdateError{Code: ErrorCodeIndexInvalid, Err: err}
	}
	if !metadata.SupportsPlatform(s.config.Platform) || !metadata.SupportsArch(s.config.Architecture) {
		return nil, &UpdateError{Code: ErrorCodeCompatibilityFailed, Err: ErrPlatformNotSupported}
	}
	if strings.TrimSpace(currentVersion) == "" {
		return metadata, nil
	}
	updateType, err := CompareVersions(currentVersion, metadata.Version)
	if err != nil {
		return nil, &UpdateError{Code: ErrorCodeCompatibilityFailed, Err: err}
	}
	if updateType == UpdateTypeSame || updateType == UpdateTypeDowngrade {
		return nil, nil
	}
	if strings.TrimSpace(metadata.MinimumHostVersion) != "" {
		compatible, err := hostVersionAtLeast(s.config.HostVersion, metadata.MinimumHostVersion)
		if err != nil || !compatible {
			return nil, &UpdateError{Code: ErrorCodeCompatibilityFailed, Err: fmt.Errorf("%w: minimum host version %s", ErrVersionNotCompatible, metadata.MinimumHostVersion)}
		}
	}
	if strings.TrimSpace(metadata.MaximumHostVersion) != "" {
		within, err := hostVersionAtMost(s.config.HostVersion, metadata.MaximumHostVersion)
		if err != nil || !within {
			return nil, &UpdateError{Code: ErrorCodeCompatibilityFailed, Err: fmt.Errorf("%w: maximum host version %s", ErrVersionNotCompatible, metadata.MaximumHostVersion)}
		}
	}
	return metadata, nil
}

func (s *Service) StartDownload(ctx context.Context, extensionID, currentVersion, targetVersion string) (DownloadState, error) {
	metadata, err := s.Check(ctx, extensionID, currentVersion)
	if err != nil {
		return DownloadState{}, err
	}
	if metadata == nil {
		return DownloadState{}, &UpdateError{Code: ErrorCodeNoUpdate, Err: fmt.Errorf("no update available for %s", extensionID)}
	}
	if strings.TrimSpace(targetVersion) != "" && metadata.Version != targetVersion {
		return DownloadState{}, &UpdateError{Code: ErrorCodeCompatibilityFailed, Err: fmt.Errorf("target version %s is no longer current registry release %s", targetVersion, metadata.Version)}
	}
	downloadID := "extension-download-" + uuid.NewString()
	expectedHash := metadata.PackageSHA256
	if metadata.PackageSHA512 != "" {
		expectedHash = metadata.PackageSHA512
	}
	if err := s.downloads.StartDownload(context.Background(), downloadID, metadata.PackageURL, expectedHash, metadata.PackageSize, s.config.AllowPrivateDownloads); err != nil {
		return DownloadState{}, err
	}
	s.mu.Lock()
	s.metadata[downloadID] = *metadata
	s.mu.Unlock()
	state, ok := s.downloads.GetDownloadState(downloadID)
	if !ok {
		return DownloadState{}, ErrDownloadNotFound
	}
	return *state, nil
}

func (s *Service) GetDownload(downloadID string) (DownloadState, ExtensionUpdateMetadata, error) {
	state, ok := s.downloads.GetDownloadState(downloadID)
	if !ok {
		return DownloadState{}, ExtensionUpdateMetadata{}, ErrDownloadNotFound
	}
	s.mu.RLock()
	metadata, ok := s.metadata[downloadID]
	s.mu.RUnlock()
	if !ok {
		return DownloadState{}, ExtensionUpdateMetadata{}, ErrDownloadNotFound
	}
	return *state, metadata, nil
}

func (s *Service) CancelDownload(downloadID string) error {
	if err := s.downloads.CancelDownload(downloadID); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.metadata, downloadID)
	s.mu.Unlock()
	return nil
}

func (s *Service) RetryDownload(downloadID string) (DownloadState, error) {
	if err := s.downloads.ResumeDownload(context.Background(), downloadID, s.config.AllowPrivateDownloads); err != nil {
		return DownloadState{}, err
	}
	state, ok := s.downloads.GetDownloadState(downloadID)
	if !ok {
		return DownloadState{}, ErrDownloadNotFound
	}
	return *state, nil
}

func (s *Service) CompletedDownload(downloadID string) (string, ExtensionUpdateMetadata, error) {
	state, metadata, err := s.GetDownload(downloadID)
	if err != nil {
		return "", ExtensionUpdateMetadata{}, err
	}
	if state.Status != DownloadStatusCompleted {
		return "", ExtensionUpdateMetadata{}, fmt.Errorf("%w: download status %s", ErrDownloadFailed, state.Status)
	}
	path, err := s.downloads.GetDownloadPath(downloadID)
	if err != nil {
		return "", ExtensionUpdateMetadata{}, err
	}
	return path, metadata, nil
}

func (s *Service) CleanupDownload(downloadID string) {
	if s == nil {
		return
	}
	s.downloads.CleanupDownload(downloadID)
	s.mu.Lock()
	delete(s.metadata, downloadID)
	s.mu.Unlock()
}

func hostVersionAtLeast(hostVersion, minimum string) (bool, error) {
	host, err := ParseVersion(hostVersion)
	if err != nil {
		return false, err
	}
	required, err := ParseVersion(minimum)
	if err != nil {
		return false, err
	}
	return host.Compare(required) >= 0, nil
}

func hostVersionAtMost(hostVersion, maximum string) (bool, error) {
	host, err := ParseVersion(hostVersion)
	if err != nil {
		return false, err
	}
	limit, err := ParseVersion(maximum)
	if err != nil {
		return false, err
	}
	return host.Compare(limit) <= 0, nil
}
