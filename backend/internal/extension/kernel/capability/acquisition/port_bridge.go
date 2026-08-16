package acquisition

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/mcp"
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
		ID: serverName,
		Owner: mcp.ExtensionOwnerRef{Type: "user"},
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

// NewEnableExistingPortBridge 创建启用现有能力端口桥接
func NewEnableExistingPortBridge(enablementSvc *enablement.EnablementService) EnableExistingPort {
	return &enableExistingPortBridge{enablementSvc: enablementSvc}
}

type enableExistingPortBridge struct {
	enablementSvc *enablement.EnablementService
}

func (b *enableExistingPortBridge) EnableExtension(ctx context.Context, extID string) error {
	if b.enablementSvc == nil {
		return fmt.Errorf("enablement service not configured")
	}
	return b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID})
}

func (b *enableExistingPortBridge) EnableSkill(ctx context.Context, skillID string) error {
	if b.enablementSvc == nil {
		return fmt.Errorf("enablement service not configured")
	}
	return b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectAgentSkill, ID: skillID})
}

func (b *enableExistingPortBridge) EnableMCP(ctx context.Context, serverName string) error {
	if b.enablementSvc == nil {
		return fmt.Errorf("enablement service not configured")
	}
	return b.enablementSvc.Enable(ctx, enablement.StateSubject{Kind: enablement.SubjectMCPServer, ID: serverName})
}

// ---------------------------------------------------------------------------

// 接口兼容性校验
var _ MCPInstallPort = (*mcpInstallPortBridge)(nil)
var _ SkillInstallPort = (*skillInstallPortBridge)(nil)
var _ EnableExistingPort = (*enableExistingPortBridge)(nil)
