package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	legacymcp "github.com/u-ai/backend/internal/mcp"
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
}

// NewPackagePortBridgeFromManager creates a PackageInstallPort backed by the lifecycle Manager.
func NewPackagePortBridgeFromManager(manager *lifecycle_manager.Manager) PackageInstallPort {
	return &packagePortBridge{manager: manager}
}

// NewPackagePortBridgeWithResolver creates a PackageInstallPort with artifact resolution for proper URI handling.
func NewPackagePortBridgeWithResolver(manager *lifecycle_manager.Manager, store RemoteArtifactStorer, registry RemoteArtifactRegistry) PackageInstallPort {
	return &packagePortBridge{manager: manager, artifactStore: store, artifactRegistry: registry}
}

func (b *packagePortBridge) InstallPackage(ctx context.Context, extID string, version string, packageID string, hash string) (string, error) {
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

func (b *packagePortBridge) UninstallPackage(ctx context.Context, extID string) error {
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
// SkillInstallPort implementation
// ---------------------------------------------------------------------------

// skillCatalogBridge wraps agent_skill.AgentSkillCatalog to implement SkillServicePort.
type skillCatalogBridge struct {
	catalog *agent_skill.AgentSkillCatalog
	builder *agent_skill.SkillDefinitionBuilder
}

// NewSkillCatalogBridge creates a SkillServicePort backed by AgentSkillCatalog.
func NewSkillCatalogBridge(catalog *agent_skill.AgentSkillCatalog) SkillServicePort {
	return &skillCatalogBridge{
		catalog: catalog,
		builder: agent_skill.NewSkillDefinitionBuilder("1.0.0", "windows"),
	}
}

// readSkillSource reads the skill source content from the given URI.
// Supports file:// scheme, http/https URLs, and plain file paths.
func (b *skillCatalogBridge) readSkillSource(sourceURI string) ([]byte, error) {
	if sourceURI == "" {
		return nil, fmt.Errorf("skill catalog bridge: source URI is empty")
	}
	if len(sourceURI) >= 7 && sourceURI[:7] == "file://" {
		content, err := os.ReadFile(sourceURI[7:])
		if err != nil {
			return nil, fmt.Errorf("skill catalog bridge: read source %s: %w", sourceURI, err)
		}
		return content, nil
	}
	if len(sourceURI) >= 5 && (sourceURI[:5] == "http:" || sourceURI[:6] == "https:") {
		return b.downloadSkillSource(sourceURI)
	}
	content, err := os.ReadFile(sourceURI)
	if err != nil {
		return nil, fmt.Errorf("skill catalog bridge: read source %s: %w", sourceURI, err)
	}
	return content, nil
}

func (b *skillCatalogBridge) downloadSkillSource(sourceURI string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, sourceURI, nil)
	if err != nil {
		return nil, fmt.Errorf("skill catalog bridge: create request for %s: %w", sourceURI, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skill catalog bridge: download %s: %w", sourceURI, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("skill catalog bridge: download %s returned status %d", sourceURI, resp.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("skill catalog bridge: read response from %s: %w", sourceURI, err)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("skill catalog bridge: downloaded empty content from %s", sourceURI)
	}
	return content, nil
}

func (b *skillCatalogBridge) ImportSkill(ctx context.Context, sourceURI string, skillName string, hash string) (string, error) {
	if b.catalog == nil {
		return "", fmt.Errorf("skill catalog bridge: catalog not configured")
	}

	// Step 1: Source read — must actually read the source file
	raw, err := b.readSkillSource(sourceURI)
	if err != nil {
		return "", err
	}

	// Step 1.5: Hash verification — if hash provided, verify integrity
	if hash != "" {
		rawHash := sha256.Sum256(raw)
		computed := hex.EncodeToString(rawHash[:])
		if !strings.EqualFold(computed, hash) {
			return "", fmt.Errorf("skill catalog bridge: hash mismatch for %s: expected %s, got %s", skillName, hash, computed)
		}
	}

	// Step 2: Parser + Step 3: Validator
	extID := "imported.skill." + skillName
	def, err := b.builder.Build(raw, extID)
	if err != nil {
		// Validator failure must NOT write to the formal Skill Registry
		return "", fmt.Errorf("skill catalog bridge: build skill %s: %w", skillName, err)
	}

	// Step 4: Install
	if err := b.catalog.Register(*def); err != nil {
		return "", fmt.Errorf("skill catalog bridge: register skill %s: %w", skillName, err)
	}

	// Step 5: Scope Enable
	if err := b.catalog.SetEnabled(extID, true); err != nil {
		return "", fmt.Errorf("skill catalog bridge: enable skill %s: %w", skillName, err)
	}

	return extID, nil
}

func (b *skillCatalogBridge) RemoveSkill(ctx context.Context, skillID string) error {
	if b.catalog == nil {
		return fmt.Errorf("skill catalog bridge: catalog not configured")
	}
	return b.catalog.Unregister(skillID)
}

func (b *skillCatalogBridge) EnableSkill(ctx context.Context, skillID string) error {
	if b.catalog == nil {
		return fmt.Errorf("skill catalog bridge: catalog not configured")
	}
	return b.catalog.SetEnabled(skillID, true)
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

var _ PackageInstallPort = (*packagePortBridge)(nil)
var _ MCPInstallPort = (*mcpRepositoryBridge)(nil)
var _ SkillServicePort = (*skillCatalogBridge)(nil)
