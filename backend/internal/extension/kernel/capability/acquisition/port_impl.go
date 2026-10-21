package acquisition

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	legacymcp "github.com/u-ai/backend/internal/extension/mcpruntime"
)

// RemoteArtifactStorer is the interface needed to resolve a remote PackageURI
// into a managed ArtifactID. Implemented by the kernel's PackageArtifactStore.
type RemoteArtifactStorer interface {
	PutArchiveFromURI(ctx context.Context, uri string, metadata ArtifactStoreMetadata) (StoredArtifact, error)
	HasArtifactAtHash(expectedHash string) (string, error)
	ArtifactIDFromHash(hash string) string
}

// ArtifactStoreMetadata carries metadata for storing a remote artifact.
type ArtifactStoreMetadata struct {
	ExtensionID  string
	Version      string
	SourceURI    string
	ExpectedHash string
}

// StoredArtifact is the result of storing a remote artifact.
type StoredArtifact struct {
	ArtifactID   string
	ArchiveHash  string
	ArchivePath  string
	ManifestHash string
}

// RemoteArtifactRegistry is the interface needed to register an artifact
// in the package repository.
type RemoteArtifactRegistry interface {
	PutArtifact(ctx context.Context, artifact ArtifactRecord) error
	GetArtifactByArchivePath(ctx context.Context, archivePath string) (*ArtifactRecord, error)
}

// ArtifactRecord is the canonical artifact record.
type ArtifactRecord struct {
	ArtifactID   string
	ExtensionID  string
	Version      string
	ArchiveHash  string
	ArchivePath  string
	ManifestHash string
}

// ---------------------------------------------------------------------------
// PackageInstallPort implementation
// ---------------------------------------------------------------------------

// packagePortBridge wraps lifecycle_manager.Manager to implement PackageInstallPort.
type packagePortBridge struct {
	manager          *lifecycle_manager.Manager
	artifactStore    RemoteArtifactStorer
	artifactRegistry RemoteArtifactRegistry
	canonical        CanonicalPackageInstallPort
}

// CanonicalPackageInstallPort is implemented by the kernel package runtime.
// Acquisition uses it to enter the same Preview -> Confirmation -> PackageInstallSaga
// path as all other package installs instead of maintaining a second install saga.
type CanonicalPackageInstallPort interface {
	InstallArtifact(ctx context.Context, artifactID, extensionID, version, expectedHash, spaceID string) (string, error)
	UninstallExtension(ctx context.Context, extensionID, spaceID string) error
}

type PackageInstallationVerifier interface {
	VerifyPackageInstalled(context.Context, string, string) (bool, error)
}

func (b *packagePortBridge) VerifyPackageInstalled(ctx context.Context, id, spaceID string) (bool, error) {
	if verifier, ok := b.canonical.(PackageInstallationVerifier); ok {
		return verifier.VerifyPackageInstalled(ctx, id, spaceID)
	}
	return false, fmt.Errorf("canonical package installation verifier is unavailable")
}

// NewPackagePortBridgeFromManager creates a PackageInstallPort backed by the lifecycle Manager.
func NewPackagePortBridgeFromManager(manager *lifecycle_manager.Manager) PackageInstallPort {
	return &packagePortBridge{manager: manager}
}

// NewPackagePortBridgeWithResolver creates a PackageInstallPort with artifact resolution for proper URI handling.
func NewPackagePortBridgeWithResolver(manager *lifecycle_manager.Manager, store RemoteArtifactStorer, registry RemoteArtifactRegistry) PackageInstallPort {
	return &packagePortBridge{manager: manager, artifactStore: store, artifactRegistry: registry}
}

// NewPackagePortBridgeWithCanonicalResolver routes actual installation through
// the canonical package runtime after URI -> ArtifactID resolution.
func NewPackagePortBridgeWithCanonicalResolver(manager *lifecycle_manager.Manager, store RemoteArtifactStorer, registry RemoteArtifactRegistry, canonical CanonicalPackageInstallPort) PackageInstallPort {
	return &packagePortBridge{manager: manager, artifactStore: store, artifactRegistry: registry, canonical: canonical}
}

func (b *packagePortBridge) InstallPackage(ctx context.Context, extID string, version string, packageID string, hash string, spaceID string) (string, error) {
	if b.canonical != nil {
		return b.canonical.InstallArtifact(ctx, packageID, extID, version, hash, spaceID)
	}
	if b.manager == nil {
		return "", fmt.Errorf("package port bridge: manager not configured")
	}
	ver := domain.SemanticVersion{}
	if version != "" {
		ver, _ = domain.ParseVersion(version)
	}
	cmd := lifecycle_manager.LifecycleCommand{
		Kind:          lifecycle_manager.CmdInstall,
		ExtensionID:   domain.ExtensionID(extID),
		TargetVersion: ver,
		PackageID:     packageID,
		RequestID:     fmt.Sprintf("acq_install_%d", time.Now().UnixNano()),
		Metadata: map[string]any{
			"hash": hash,
		},
	}
	result, err := b.manager.Execute(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("package port bridge: install %s: %w", extID, err)
	}
	return result.OperationID, nil
}

func (b *packagePortBridge) UninstallPackage(ctx context.Context, extID string, spaceID string) error {
	if b.canonical != nil {
		return b.canonical.UninstallExtension(ctx, extID, spaceID)
	}
	if b.manager == nil {
		return fmt.Errorf("package port bridge: manager not configured")
	}
	cmd := lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdUninstall,
		ExtensionID: domain.ExtensionID(extID),
		RequestID:   fmt.Sprintf("acq_uninstall_%d", time.Now().UnixNano()),
	}
	_, err := b.manager.Execute(ctx, cmd)
	if err != nil {
		return fmt.Errorf("package port bridge: uninstall %s: %w", extID, err)
	}
	return nil
}

func (b *packagePortBridge) ResolveArtifact(ctx context.Context, extID string, version string, packageURI string, hash string) (string, error) {
	if b.manager == nil {
		return "", fmt.Errorf("package port bridge: manager not configured")
	}
	if packageURI == "" {
		return "", fmt.Errorf("package port bridge: packageURI is empty")
	}
	if b.artifactStore == nil || b.artifactRegistry == nil {
		return "", fmt.Errorf("package port bridge: artifact resolver not configured, cannot resolve remote URI")
	}

	artifactID, err := b.resolveAndStoreArtifact(ctx, extID, version, packageURI, hash)
	if err != nil {
		return "", fmt.Errorf("package port bridge: resolve artifact: %w", err)
	}
	return artifactID, nil
}

// resolveAndStoreArtifact downloads a remote package, stores it in the managed
// Artifact Store, registers it in the PackageRepository, and returns the canonical ArtifactID.
func (b *packagePortBridge) resolveAndStoreArtifact(ctx context.Context, extID string, version string, packageURI string, expectedHash string) (string, error) {
	archivePath, err := b.artifactStore.HasArtifactAtHash(expectedHash)
	if err == nil && archivePath != "" {
		if artifact, getErr := b.artifactRegistry.GetArtifactByArchivePath(ctx, archivePath); getErr == nil && artifact != nil {
			return artifact.ArtifactID, nil
		}
	}

	metadata := ArtifactStoreMetadata{
		ExtensionID:  extID,
		Version:      version,
		SourceURI:    packageURI,
		ExpectedHash: expectedHash,
	}

	artifact, err := b.artifactStore.PutArchiveFromURI(ctx, packageURI, metadata)
	if err != nil {
		return "", fmt.Errorf("store remote artifact: %w", err)
	}

	artifactID := artifact.ArtifactID
	if artifactID == "" {
		artifactID = b.artifactStore.ArtifactIDFromHash(artifact.ArchiveHash)
	}

	if err := b.artifactRegistry.PutArtifact(ctx, ArtifactRecord{
		ArtifactID:   artifactID,
		ExtensionID:  extID,
		Version:      version,
		ArchiveHash:  artifact.ArchiveHash,
		ArchivePath:  artifact.ArchivePath,
		ManifestHash: artifact.ManifestHash,
	}); err != nil {
		return "", fmt.Errorf("register artifact: %w", err)
	}

	return artifactID, nil
}

// ---------------------------------------------------------------------------
// MCPInstallPort implementation
// ---------------------------------------------------------------------------

// mcpRepositoryBridge wraps legacymcp.Repository to implement MCPInstallPort.
type mcpRepositoryBridge struct {
	repo *legacymcp.Repository
}

// NewMCPRepositoryBridge creates a MCPInstallPort backed by legacymcp.Repository.
func NewMCPRepositoryBridge(repo *legacymcp.Repository) MCPInstallPort {
	return &mcpRepositoryBridge{repo: repo}
}

func (b *mcpRepositoryBridge) InstallMCP(ctx context.Context, serverName string, transport string, command string, args []string, env map[string]string) (string, error) {
	if b.repo == nil {
		return "", fmt.Errorf("mcp repository bridge: repository not configured")
	}
	input := legacymcp.ServerInput{
		Name:      serverName,
		Transport: transport,
		Command:   command,
		Args:      args,
		Enabled:   true,
		Source:    "acquisition",
	}
	server, err := b.repo.CreateServer(ctx, input)
	if err != nil {
		return "", fmt.Errorf("mcp repository bridge: create server %s: %w", serverName, err)
	}
	return server.ID, nil
}

func (b *mcpRepositoryBridge) RemoveMCP(ctx context.Context, serverName string) error {
	if b.repo == nil {
		return fmt.Errorf("mcp repository bridge: repository not configured")
	}
	return b.repo.DeleteServer(ctx, serverName)
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

var _ PackageInstallPort = (*packagePortBridge)(nil)
var _ MCPInstallPort = (*mcpRepositoryBridge)(nil)
