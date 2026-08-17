package acquisition

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/mcp"
	legacymcp "github.com/u-ai/backend/internal/mcp"
)

// NewMCPPortBridge 创建 MCP 安装端口桥接
func NewMCPPortBridge(lifecycle *mcp.MCPLifecycle) MCPInstallPort {
	return &mcpInstallPortBridge{lifecycle: lifecycle}
}

type mcpInstallPortBridge struct {
	lifecycle *mcp.MCPLifecycle
}

func (b *mcpInstallPortBridge) InstallMCP(ctx context.Context, serverName string, transport string, command string, args []string, env map[string]string) (string, error) {
	if b.lifecycle == nil {
		return "", fmt.Errorf("MCP lifecycle not configured")
	}
	binding := mcp.MCPBinding{
		ID:        serverName,
		Owner:     mcp.ExtensionOwnerRef{Type: "user"},
		Transport: mcp.MCPTransportSpec{Kind: transport},
		Launcher: &mcp.MCPLauncherSpec{
			Kind:    string(mcp.MCPLauncherNPX),
			Command: command,
			Args:    args,
		},
	}
	if _, err := b.lifecycle.RegisterBinding(binding); err != nil {
		return "", fmt.Errorf("MCP register binding: %w", err)
	}
	plan := mcp.MCPInstallPlan{
		PlanID:    "manual-" + serverName,
		BindingID: serverName,
		Transport: transport,
		Launcher:  string(mcp.MCPLauncherNPX),
	}
	plan.PlanDigest = plan.ComputeDigest()
	if err := b.lifecycle.Install(ctx, binding, plan); err != nil {
		return "", fmt.Errorf("MCP install: %w", err)
	}
	if err := b.lifecycle.Enable(serverName); err != nil {
		return "", fmt.Errorf("MCP enable: %w", err)
	}
	if err := b.lifecycle.Start(serverName); err != nil {
		return "", fmt.Errorf("MCP start: %w", err)
	}
	return serverName, nil
}

func (b *mcpInstallPortBridge) RemoveMCP(ctx context.Context, serverName string) error {
	if b.lifecycle == nil {
		return fmt.Errorf("MCP lifecycle not configured")
	}
	return b.lifecycle.Uninstall(serverName)
}

// ---------------------------------------------------------------------------

// SkillServicePort 定义技能安装所需的最小接口
type SkillServicePort interface {
	ImportSkill(ctx context.Context, sourceURI string, skillName string, hash string) (string, error)
	RemoveSkill(ctx context.Context, skillID string) error
	EnableSkill(ctx context.Context, skillID string) error
}

// NewSkillPortBridge 创建技能安装端口桥接
func NewSkillPortBridge(svc SkillServicePort) SkillInstallPort {
	return &skillInstallPortBridge{service: svc}
}

type skillInstallPortBridge struct {
	service SkillServicePort
}

func (b *skillInstallPortBridge) ImportSkill(ctx context.Context, sourceURI string, skillName string, hash string) (string, error) {
	if b.service == nil {
		return "", fmt.Errorf("skill service not configured")
	}
	return b.service.ImportSkill(ctx, sourceURI, skillName, hash)
}

func (b *skillInstallPortBridge) RemoveSkill(ctx context.Context, skillID string) error {
	if b.service == nil {
		return fmt.Errorf("skill service not configured")
	}
	return b.service.RemoveSkill(ctx, skillID)
}

// ---------------------------------------------------------------------------

// NewPackagePortBridge 创建包安装端口桥接（直接透传）
func NewPackagePortBridge(installer PackageInstallPort) PackageInstallPort {
	return installer
}

// ---------------------------------------------------------------------------

// EnableExistingDeps 定义 EnableExistingPort 所需的依赖。
type EnableExistingDeps struct {
	EnablementSvc      *enablement.EnablementService
	InstallRepo        domain.InstallationRepository
	DefinitionRepo     domain.DefinitionRepository
	InstanceReconciler capability.ProviderInstanceReconciler
	MCPRepository      *legacymcp.Repository
	MCPLifecycle       *mcp.MCPLifecycle
	AgentSkillCatalog  *agent_skill.AgentSkillCatalog
	ProviderRegistry   *capability.ProviderRegistry
}

// NewEnableExistingPortBridge 创建启用现有能力端口桥接
func NewEnableExistingPortBridge(enablementSvc *enablement.EnablementService) EnableExistingPort {
	return &enableExistingPortBridge{enablementSvc: enablementSvc}
}

// NewEnableExistingPortBridgeWithDeps 创建带完整依赖的启用现有能力端口桥接
func NewEnableExistingPortBridgeWithDeps(deps EnableExistingDeps) EnableExistingPort {
	return &enableExistingPortBridge{
		enablementSvc:      deps.EnablementSvc,
		installRepo:        deps.InstallRepo,
		definitionRepo:     deps.DefinitionRepo,
		instanceReconciler: deps.InstanceReconciler,
		mcpRepository:      deps.MCPRepository,
		mcpLifecycle:       deps.MCPLifecycle,
		agentSkillCatalog:  deps.AgentSkillCatalog,
		providerRegistry:   deps.ProviderRegistry,
	}
}

type enableExistingPortBridge struct {
	enablementSvc      *enablement.EnablementService
	installRepo        domain.InstallationRepository
	definitionRepo     domain.DefinitionRepository
	instanceReconciler capability.ProviderInstanceReconciler
	mcpRepository      *legacymcp.Repository
	mcpLifecycle       *mcp.MCPLifecycle
	agentSkillCatalog  *agent_skill.AgentSkillCatalog
	providerRegistry   *capability.ProviderRegistry
}

func (b *enableExistingPortBridge) EnableExtension(ctx context.Context, extID string) error {
	if b.enablementSvc == nil {
		return fmt.Errorf("enablement service not configured")
	}

	if extID == "" {
		return fmt.Errorf("enable extension: empty extensionId")
	}

	// 验证扩展是否真实安装
	if b.installRepo != nil {
		_, err := b.installRepo.GetInstallation(ctx, domain.ExtensionID(extID))
		if err != nil {
			return fmt.Errorf("enable extension %s: not installed or not found: %w", extID, err)
		}
	}

	// 持久化启用状态
	if err := b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}); err != nil {
		return fmt.Errorf("enable extension %s: %w", extID, err)
	}

	// 触发 ProviderInstance 协调：若 placement 未生成则生成真实 ProviderInstance
	if b.instanceReconciler != nil && b.definitionRepo != nil && b.providerRegistry != nil {
		def, err := b.definitionRepo.GetExtension(ctx, domain.ExtensionID(extID), domain.SemanticVersion{})
		if err == nil && def.ID != "" {
			providerDefs := b.providerRegistry.ListByExtension(extID)
			hasInstances := false
			for _, pd := range providerDefs {
				if pd == nil {
					continue
				}
				if insts := b.providerRegistry.ListInstancesByProvider(pd.ID); len(insts) > 0 {
					hasInstances = true
					break
				}
			}
			if !hasInstances {
				_, _ = b.instanceReconciler.ActivateExtension(def, nil)
			}
		}
	}

	return nil
}

func (b *enableExistingPortBridge) EnableSkill(ctx context.Context, skillID string) error {
	if b.enablementSvc == nil {
		return fmt.Errorf("enablement service not configured")
	}

	if skillID == "" {
		return fmt.Errorf("enable skill: empty skillId")
	}

	// 验证技能是否存在
	if b.agentSkillCatalog != nil {
		_, exists := b.agentSkillCatalog.Get(skillID)
		if !exists {
			return fmt.Errorf("enable skill %s: not found", skillID)
		}
	}

	// 持久化启用状态
	if err := b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectAgentSkill, ID: skillID}); err != nil {
		return fmt.Errorf("enable skill %s: %w", skillID, err)
	}

	return nil
}

func (b *enableExistingPortBridge) EnableMCP(ctx context.Context, serverName string) error {
	if b.enablementSvc == nil {
		return fmt.Errorf("enablement service not configured")
	}

	if serverName == "" {
		return fmt.Errorf("enable MCP: empty server name")
	}

	if b.mcpRepository != nil {
		_, err := b.mcpRepository.GetServer(ctx, serverName)
		if err != nil {
			return fmt.Errorf("enable MCP %s: not found: %w", serverName, err)
		}
	}

	if b.mcpLifecycle != nil {
		if _, err := b.mcpLifecycle.GetInstallation(serverName); err != nil {
			binding := mcp.MCPBinding{
				ID:        serverName,
				Owner:     mcp.ExtensionOwnerRef{Type: "user"},
				Transport: mcp.MCPTransportSpec{Kind: "stdio"},
			}
			if _, regErr := b.mcpLifecycle.RegisterBinding(binding); regErr != nil {
				return fmt.Errorf("enable MCP %s: register binding: %w", serverName, regErr)
			}
		}

		if err := b.mcpLifecycle.Enable(serverName); err != nil {
			return fmt.Errorf("enable MCP %s: lifecycle enable: %w", serverName, err)
		}

		if err := b.mcpLifecycle.Start(serverName); err != nil {
			return fmt.Errorf("enable MCP %s: lifecycle start: %w", serverName, err)
		}

		if !b.isMCPConnected(ctx, serverName) {
			return fmt.Errorf("enable MCP %s: server not connected", serverName)
		}

		if err := b.mcpLifecycle.MarkReady(serverName); err != nil {
			return fmt.Errorf("enable MCP %s: lifecycle mark ready: %w", serverName, err)
		}
	}

	if err := b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectMCPServer, ID: serverName}); err != nil {
		return fmt.Errorf("enable MCP %s: %w", serverName, err)
	}

	return nil
}

func (b *enableExistingPortBridge) isMCPConnected(ctx context.Context, serverName string) bool {
	if b.mcpRepository == nil {
		return false
	}
	server, err := b.mcpRepository.GetServer(ctx, serverName)
	if err != nil {
		return false
	}
	return server.Status == "running" || server.Status == "connected"
}

// ---------------------------------------------------------------------------

// 接口兼容性校验
var _ MCPInstallPort = (*mcpInstallPortBridge)(nil)
var _ SkillInstallPort = (*skillInstallPortBridge)(nil)
var _ EnableExistingPort = (*enableExistingPortBridge)(nil)
