package acquisition

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	"github.com/u-ai/backend/internal/extension/kernel/mcp"
	legacymcp "github.com/u-ai/backend/internal/mcp"
)

// MCPRuntimeConnectRequest describes the canonical runtime connection required
// after installation. The runtime port owns protocol initialization and discovery.
type MCPRuntimeConnectRequest struct {
	ServerID  string
	Transport string
	Command   string
	Args      []string
	Env       map[string]string
}

// MCPRuntimeConnectPort connects a real MCP runtime and returns its discovered tools.
type MCPRuntimeConnectPort interface {
	ConnectAndDiscover(ctx context.Context, req MCPRuntimeConnectRequest) ([]capability.MCPToolDescriptor, error)
	Disconnect(ctx context.Context, serverID string) error
}

// NewMCPPortBridge 创建 MCP 安装端口桥接
func NewMCPPortBridge(lifecycle *mcp.MCPLifecycle) MCPInstallPort {
	return &mcpInstallPortBridge{lifecycle: lifecycle}
}

// NewMCPPortBridgeWithDiscovery 创建带工具发现功能的 MCP 安装端口桥接
func NewMCPPortBridgeWithDiscovery(lifecycle *mcp.MCPLifecycle, toolSync MCPToolSyncPort) MCPInstallPort {
	return &mcpInstallPortBridge{lifecycle: lifecycle, toolSync: toolSync}
}

// NewMCPPortBridgeWithRuntime creates the production bridge that owns the
// complete Install -> Start -> Connect -> Discover -> Sync lifecycle.
func NewMCPPortBridgeWithRuntime(lifecycle *mcp.MCPLifecycle, runtime MCPRuntimeConnectPort, toolSync MCPToolSyncPort) MCPInstallPort {
	return &mcpInstallPortBridge{lifecycle: lifecycle, runtime: runtime, toolSync: toolSync}
}

// MCPToolSyncPort defines the interface for syncing MCP tools to the registry
type MCPToolSyncPort interface {
	SyncMCPTools(ctx context.Context, serverID string, descriptors []capability.MCPToolDescriptor) (*MCPToolSyncResult, error)
	ListMCPTools(ctx context.Context, serverID string) ([]capability.MCPToolDescriptor, error)
}

// MCPToolSyncResult represents the result of syncing MCP tools
type MCPToolSyncResult struct {
	ServerID   string
	Registered int
	Updated    int
	Removed    int
	Total      int
}

type mcpInstallPortBridge struct {
	lifecycle *mcp.MCPLifecycle
	runtime   MCPRuntimeConnectPort
	toolSync  MCPToolSyncPort
}

func (b *mcpInstallPortBridge) InstallMCP(ctx context.Context, serverName string, transport string, command string, args []string, env map[string]string) (string, error) {
	if b.lifecycle == nil {
		return "", fmt.Errorf("MCP lifecycle not configured")
	}
	launcherKind := resolveLauncherKind(transport, command)
	binding := mcp.MCPBinding{
		ID:        serverName,
		Owner:     mcp.ExtensionOwnerRef{Type: "user"},
		Transport: mcp.MCPTransportSpec{Kind: transport},
	}
	if transport == "streamable_http" || transport == "sse" || transport == "remote" {
		binding.Transport.Endpoint = command
	} else {
		binding.Launcher = &mcp.MCPLauncherSpec{
			Kind:        string(launcherKind),
			Command:     command,
			Args:        args,
			Environment: env,
		}
	}
	if _, err := b.lifecycle.RegisterBinding(binding); err != nil {
		return "", fmt.Errorf("MCP register binding: %w", err)
	}
	plan := mcp.MCPInstallPlan{
		PlanID:    "manual-" + serverName,
		BindingID: serverName,
		Transport: transport,
	}
	if binding.Launcher != nil {
		plan.Launcher = string(launcherKind)
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

	if b.runtime == nil {
		return "", fmt.Errorf("MCP runtime connector not configured")
	}
	descriptors, err := b.runtime.ConnectAndDiscover(ctx, MCPRuntimeConnectRequest{
		ServerID: serverName, Transport: transport, Command: command, Args: args, Env: env,
	})
	if err != nil {
		return "", fmt.Errorf("MCP connect/discover: %w", err)
	}
	if b.toolSync == nil {
		return "", fmt.Errorf("MCP tool sync not configured")
	}
	if _, err := b.toolSync.SyncMCPTools(ctx, serverName, descriptors); err != nil {
		return "", fmt.Errorf("MCP sync tools: %w", err)
	}
	if err := b.lifecycle.MarkReady(serverName); err != nil {
		return "", fmt.Errorf("MCP mark ready: %w", err)
	}

	return serverName, nil
}

func resolveLauncherKind(transport string, command string) mcp.MCPLauncherKind {
	lower := strings.ToLower(strings.TrimSpace(command))
	switch transport {
	case "executable":
		return mcp.MCPLauncherExecutable
	case "stdio":
		switch {
		case strings.Contains(lower, "uvx") || strings.Contains(lower, "uv "):
			return mcp.MCPLauncherUVX
		case strings.Contains(lower, "npx"):
			return mcp.MCPLauncherNPX
		default:
			return mcp.MCPLauncherExecutable
		}
	default:
		return mcp.MCPLauncherExecutable
	}
}

func (b *mcpInstallPortBridge) RemoveMCP(ctx context.Context, serverName string) error {
	if b.lifecycle == nil {
		return fmt.Errorf("MCP lifecycle not configured")
	}
	if b.runtime != nil {
		_ = b.runtime.Disconnect(ctx, serverName)
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
	LifecycleManager   *lifecycle_manager.Manager
	MCPRuntime         MCPRuntimeConnectPort
	MCPToolSync        MCPToolSyncPort
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
		lifecycleManager:   deps.LifecycleManager,
		mcpRuntime:         deps.MCPRuntime,
		mcpToolSync:        deps.MCPToolSync,
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
	lifecycleManager   *lifecycle_manager.Manager
	mcpRuntime         MCPRuntimeConnectPort
	mcpToolSync        MCPToolSyncPort
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

	// 通过正式 lifecycle manager 启用运行时；只有 runtime/lifecycle 成功后才写 enablement。
	if b.lifecycleManager != nil {
		if _, err := b.lifecycleManager.Execute(ctx, lifecycle_manager.LifecycleCommand{
			Kind:        lifecycle_manager.CmdEnable,
			ExtensionID: domain.ExtensionID(extID),
			RequestID:   fmt.Sprintf("acq_enable_%s", extID),
		}); err != nil {
			return fmt.Errorf("enable extension %s: lifecycle enable: %w", extID, err)
		}
	}

	// 持久化启用状态
	if err := b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}); err != nil {
		return fmt.Errorf("enable extension %s: %w", extID, err)
	}

	// 触发 ProviderInstance 协调：若 placement 未生成则生成真实 ProviderInstance
	if b.lifecycleManager == nil && b.instanceReconciler != nil && b.definitionRepo != nil && b.providerRegistry != nil {
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

	if b.agentSkillCatalog != nil {
		if err := b.agentSkillCatalog.SetEnabled(skillID, true); err != nil {
			return fmt.Errorf("enable skill %s: catalog enable: %w", skillID, err)
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

		if b.mcpRuntime == nil {
			return fmt.Errorf("enable MCP %s: runtime connector not configured", serverName)
		}
		descriptors, err := b.mcpRuntime.ConnectAndDiscover(ctx, MCPRuntimeConnectRequest{ServerID: serverName})
		if err != nil {
			return fmt.Errorf("enable MCP %s: connect/discover: %w", serverName, err)
		}
		if b.mcpToolSync != nil {
			if _, err := b.mcpToolSync.SyncMCPTools(ctx, serverName, descriptors); err != nil {
				return fmt.Errorf("enable MCP %s: tool sync: %w", serverName, err)
			}
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
