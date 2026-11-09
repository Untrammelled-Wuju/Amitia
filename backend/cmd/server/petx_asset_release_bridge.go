package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/integration"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	domainkernel "github.com/u-ai/backend/internal/extension/kernel/domain"
	"gorm.io/gorm"
)

type petxAssetReleaseBridge struct {
	mu             sync.Mutex
	kernelInstalls domainkernel.InstallationRepository
	installations  installation.Repository
	releases       release.ReleaseRepository
	storage        release.ReleaseStoragePort
	coordinator    coordinator.InstallationCoordinator
	spaceID        string
	deviceID       string
	handles        map[string]integration.ExistingPetResourceBinding
}

func newPetxAssetReleaseBridge(
	kernelInstalls domainkernel.InstallationRepository,
	installations installation.Repository,
	releases release.ReleaseRepository,
	storage release.ReleaseStoragePort,
	installationCoordinator coordinator.InstallationCoordinator,
	spaceID string,
	deviceID string,
) *petxAssetReleaseBridge {
	return &petxAssetReleaseBridge{
		kernelInstalls: kernelInstalls,
		installations:  installations,
		releases:       releases,
		storage:        storage,
		coordinator:    installationCoordinator,
		spaceID:        strings.TrimSpace(spaceID),
		deviceID:       strings.TrimSpace(deviceID),
		handles:        make(map[string]integration.ExistingPetResourceBinding),
	}
}

func (b *petxAssetReleaseBridge) AttachPluginResource(
	ctx context.Context,
	extensionID string,
	contributionID string,
	revision int,
	definition map[string]any,
) (string, error) {
	resourceRef := strings.TrimSpace(fmt.Sprint(definition["resourceRef"]))
	if resourceRef == "" || resourceRef == "<nil>" {
		return b.attachCorrelation(extensionID, contributionID, revision, definition)
	}
	if b.spaceID == "" || b.deviceID == "" {
		return "", errors.New("petx asset bridge requires local space and device identity")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	assetRoot, manifest, err := b.resolveAssetManifest(ctx, extensionID, resourceRef)
	if err != nil {
		return "", err
	}
	report := packageformat.NewValidator().ValidateDirectory(assetRoot, manifest)
	if report == nil || report.Verdict == "invalid" {
		return "", fmt.Errorf("petx asset validation failed: %+v", report)
	}
	releaseData, err := b.ensureRelease(ctx, extensionID, assetRoot, manifest)
	if err != nil {
		return "", err
	}
	if err := b.ensureInstallation(ctx, extensionID, releaseData); err != nil {
		return "", err
	}

	handle := "petx://" + extensionID + "/" + contributionID + "/" + releaseData.ContentRootHash
	b.handles[handle] = integration.ExistingPetResourceBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
		Definition:     clonePetxDefinition(definition),
	}
	return handle, nil
}

func (b *petxAssetReleaseBridge) DetachPluginResource(ctx context.Context, handle string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handles, handle)
	return nil
}

func (b *petxAssetReleaseBridge) ListAttachedResources(ctx context.Context, extensionID string) ([]integration.ExistingPetResourceBinding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]integration.ExistingPetResourceBinding, 0)
	for _, binding := range b.handles {
		if extensionID == "" || binding.ExtensionID == extensionID {
			result = append(result, binding)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Handle < result[j].Handle })
	return result, nil
}

func (b *petxAssetReleaseBridge) RebuildFromExisting() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handles = make(map[string]integration.ExistingPetResourceBinding)
	return nil
}

func (b *petxAssetReleaseBridge) attachCorrelation(extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handle := "petx-correlation://" + extensionID + "/" + contributionID + "/" + fmt.Sprint(revision)
	b.handles[handle] = integration.ExistingPetResourceBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
		Definition:     clonePetxDefinition(definition),
	}
	return handle, nil
}

func (b *petxAssetReleaseBridge) resolveAssetManifest(
	ctx context.Context,
	extensionID string,
	resourceRef string,
) (string, *packageformat.Manifest, error) {
	if b.kernelInstalls == nil {
		return "", nil, errors.New("kernel installation repository unavailable")
	}
	installed, err := b.kernelInstalls.GetInstallation(ctx, domainkernel.ExtensionID(extensionID))
	if err != nil {
		return "", nil, fmt.Errorf("read petx installation: %w", err)
	}
	installPath := strings.TrimSpace(fmt.Sprint(installed.Metadata["currentPath"]))
	if installPath == "" || installPath == "<nil>" {
		installPath = strings.TrimSpace(fmt.Sprint(installed.Metadata["installedPath"]))
	}
	if installPath == "" || installPath == "<nil>" {
		return "", nil, errors.New("petx installation path is missing")
	}
	manifestPath := filepath.Join(installPath, filepath.FromSlash(resourceRef))
	relative, err := filepath.Rel(installPath, manifestPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", nil, errors.New("petx resource reference escapes installation")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", nil, fmt.Errorf("read petx asset manifest: %w", err)
	}
	var manifest packageformat.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", nil, fmt.Errorf("decode petx asset manifest: %w", err)
	}
	return filepath.Dir(manifestPath), &manifest, nil
}

func (b *petxAssetReleaseBridge) ensureRelease(
	ctx context.Context,
	extensionID string,
	assetRoot string,
	manifest *packageformat.Manifest,
) (*release.ReleaseData, error) {
	if b.releases == nil || b.storage == nil {
		return nil, errors.New("desktop pet release services unavailable")
	}
	if manifest.ReleaseID == "" || manifest.PetID == "" || manifest.DefaultAction == "" {
		return nil, errors.New("petx asset manifest identity is incomplete")
	}
	existing, err := b.releases.GetRelease(manifest.ReleaseID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("read petx release: %w", err)
	}
	if err == nil && existing != nil {
		if existing.OwnerSpaceID != b.spaceID || existing.PetID != manifest.PetID {
			return nil, errors.New("petx release identity belongs to another owner or pet")
		}
		if existing.ContentRootHash != manifest.Integrity.ContentRootHash ||
			existing.ManifestHash != manifest.Integrity.ManifestHash {
			return nil, fmt.Errorf("petx release id %s was reused with different content", manifest.ReleaseID)
		}
		return existing, nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	identity, err := b.releases.GetPetIdentity(manifest.PetID)
	if err != nil {
		identity = &release.PetIdentityData{
			ID:                  manifest.PetID,
			OwnerSpaceID:        b.spaceID,
			Name:                manifest.Name,
			Slug:                strings.ToLower(strings.ReplaceAll(manifest.PetID, " ", "-")),
			UpstreamPetID:       extensionID,
			DefaultActionKey:    manifest.DefaultAction,
			NextReleaseSequence: 1,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	}
	if identity.OwnerSpaceID != b.spaceID {
		return nil, errors.New("petx pet identity belongs to another owner")
	}

	if err := b.publishPetxFiles(manifest, assetRoot); err != nil {
		return nil, err
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode petx asset manifest: %w", err)
	}
	storageKey, err := b.storage.PublishedStorageKey(manifest.PetID, manifest.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("resolve petx release storage: %w", err)
	}
	archiveKey, archiveHash, archiveBytes, err := b.storePetxArchive(assetRoot, manifest)
	if err != nil {
		return nil, err
	}
	sequence := identity.NextReleaseSequence
	if sequence < 1 {
		sequence = 1
	}
	identity.NextReleaseSequence = sequence + 1
	identity.UpdatedAt = now
	releaseData := &release.ReleaseData{
		ID:                   manifest.ReleaseID,
		PetID:                manifest.PetID,
		OwnerSpaceID:         b.spaceID,
		Version:              manifest.Version,
		ReleaseSequence:      sequence,
		SchemaVersion:        manifest.SchemaVersion,
		Lifecycle:            string(release.ReleaseLifecycleReady),
		ContentRootHash:      manifest.Integrity.ContentRootHash,
		ManifestHash:         manifest.Integrity.ManifestHash,
		StorageKey:           storageKey,
		ArchiveStorageKey:    archiveKey,
		TotalBytes:           manifest.Integrity.TotalBytes,
		FileCount:            manifest.Integrity.FileCount,
		ActionCount:          len(manifest.Actions),
		DefaultActionKey:     manifest.DefaultAction,
		MinRuntimeVersion:    manifest.Compatibility.MinRuntimeVersion,
		SourceType:           "petx_plugin",
		SourceProcessingTask: extensionID,
		IntegrityStatus:      string(release.ReleaseIntegrityVerified),
		CompatibilityStatus:  string(release.ReleaseCompatCompatible),
		ManifestJSON:         string(manifestJSON),
		PublishedAt:          now,
		ArchiveHash:          archiveHash,
		ArchiveBytes:         archiveBytes,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	files := make([]release.ReleaseFileData, 0, len(manifest.Integrity.Files))
	for _, item := range manifest.Integrity.Files {
		files = append(files, release.ReleaseFileData{
			ID:        "petx_release_file_" + uuid.NewString(),
			ReleaseID: manifest.ReleaseID,
			Path:      item.Path,
			SHA256:    item.SHA256,
			Bytes:     item.Bytes,
			MediaType: item.MediaType,
			Role:      item.Role,
			ActionKey: item.ActionKey,
			FrameID:   item.FrameID,
			CreatedAt: now,
		})
	}
	if err := b.releases.Transaction(func(tx *gorm.DB) error {
		if existing, getErr := b.releases.GetPetIdentity(manifest.PetID); getErr != nil || existing == nil {
			if err := b.releases.CreatePetIdentityTx(tx, identity); err != nil {
				return err
			}
		} else {
			if err := b.releases.UpdatePetIdentityTx(tx, identity); err != nil {
				return err
			}
		}
		if err := b.releases.CreateReleaseTx(tx, releaseData); err != nil {
			return err
		}
		if len(files) > 0 {
			if err := b.releases.CreateReleaseFilesTx(tx, files); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return releaseData, nil
}

func (b *petxAssetReleaseBridge) publishPetxFiles(manifest *packageformat.Manifest, assetRoot string) error {
	if err := b.storage.EnsureStagingDir(manifest.ReleaseID); err != nil {
		return fmt.Errorf("create petx release staging: %w", err)
	}
	stagingDir, err := b.storage.StagingDir(manifest.ReleaseID)
	if err != nil {
		return fmt.Errorf("resolve petx release staging: %w", err)
	}
	if err := copyPetxTree(assetRoot, stagingDir); err != nil {
		return err
	}
	if err := b.storage.AtomicRenameStagingToPublished(manifest.PetID, manifest.ReleaseID); err != nil {
		return fmt.Errorf("publish petx release files: %w", err)
	}
	return nil
}

func (b *petxAssetReleaseBridge) storePetxArchive(assetRoot string, manifest *packageformat.Manifest) (string, string, int64, error) {
	temp, err := os.CreateTemp("", "amitia-petx-release-*.zip")
	if err != nil {
		return "", "", 0, err
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		return "", "", 0, err
	}
	defer os.Remove(tempPath)
	if err := packageformat.NewArchiveWriter().WriteArchive(assetRoot, tempPath); err != nil {
		return "", "", 0, err
	}
	return b.storage.StoreVerifiedArchive(tempPath, manifest.PetID, manifest.ReleaseID, "")
}

func (b *petxAssetReleaseBridge) ensureInstallation(ctx context.Context, extensionID string, releaseData *release.ReleaseData) error {
	if b.coordinator == nil || b.installations == nil {
		return errors.New("desktop pet installation services unavailable")
	}
	items, err := b.installations.ListInstallationsBySpaceDevice(b.spaceID, b.deviceID)
	if err != nil {
		return fmt.Errorf("list desktop pet installations: %w", err)
	}
	var current *installation.Installation
	for _, item := range items {
		if item == nil || item.CurrentReleaseID == "" {
			continue
		}
		if item.PetID == releaseData.PetID {
			current = item
			break
		}
	}
	deviceCtx := device.DeviceContext{SpaceID: b.spaceID, DeviceID: b.deviceID}
	if current != nil {
		if current.CurrentReleaseID == releaseData.ID {
			return nil
		}
		_, err := b.coordinator.Upgrade(ctx, coordinator.UpgradeRequest{
			DeviceCtx:       deviceCtx,
			InstallationID:  current.ID,
			TargetReleaseID: releaseData.ID,
			IdempotencyKey:  "petx:" + extensionID + ":" + releaseData.ContentRootHash,
		})
		return err
	}
	_, err = b.coordinator.Install(ctx, coordinator.InstallRequest{
		DeviceCtx:       deviceCtx,
		PetID:           releaseData.PetID,
		TargetReleaseID: releaseData.ID,
		IdempotencyKey:  "petx:" + extensionID + ":" + releaseData.ContentRootHash,
	})
	return err
}

func copyPetxTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("petx asset path escapes source")
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("petx asset symlink is forbidden")
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func clonePetxDefinition(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

var _ integration.ExistingPetResourcePort = (*petxAssetReleaseBridge)(nil)
