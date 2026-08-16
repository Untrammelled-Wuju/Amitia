package acquisition

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	legacymcp "github.com/u-ai/backend/internal/mcp"
)

// ---------------------------------------------------------------------------
// PackageInstallPort implementation
// ---------------------------------------------------------------------------

// packagePortBridge wraps lifecycle_manager.Manager to implement PackageInstallPort.
type packagePortBridge struct {
	manager *lifecycle_manager.Manager
}

// NewPackagePortBridgeFromManager creates a PackageInstallPort backed by the lifecycle Manager.
func NewPackagePortBridgeFromManager(manager *lifecycle_manager.Manager) PackageInstallPort {
	return &packagePortBridge{manager: manager}
}

func (b *packagePortBridge) InstallPackage(ctx context.Context, extID string, version string) (string, error) {
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
		RequestID:     fmt.Sprintf("acq_install_%d", time.Now().UnixNano()),
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
}

// NewSkillCatalogBridge creates a SkillServicePort backed by AgentSkillCatalog.
func NewSkillCatalogBridge(catalog *agent_skill.AgentSkillCatalog) SkillServicePort {
	return &skillCatalogBridge{catalog: catalog}
}

func (b *skillCatalogBridge) ImportSkill(ctx context.Context, sourceURI string, skillName string, hash string) (string, error) {
	if b.catalog == nil {
		return "", fmt.Errorf("skill catalog bridge: catalog not configured")
	}
	extID := "imported.skill." + skillName
	def := agent_skill.AgentSkillDefinition{
		ExtensionID: extID,
		Name:        skillName,
		Description: fmt.Sprintf("Imported from %s", sourceURI),
		Metadata: map[string]any{
			"sourceURI": sourceURI,
			"hash":      hash,
		},
	}
	if err := b.catalog.Register(def); err != nil {
		return "", fmt.Errorf("skill catalog bridge: register skill %s: %w", skillName, err)
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
