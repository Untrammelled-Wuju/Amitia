package installer

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/mcp"
)

type defaultProvisioner struct{}

func NewDefaultProvisioner() mcp.MCPDependencyProvisioner {
	return &defaultProvisioner{}
}

func (p *defaultProvisioner) Preview(ctx context.Context, spec mcp.MCPBinding) (mcp.MCPInstallPlan, error) {
	plan := mcp.MCPInstallPlan{
		PlanID:           fmt.Sprintf("plan_%s_%d", spec.ID, time.Now().UnixNano()),
		BindingID:        spec.ID,
		Transport:        spec.Transport.Kind,
		Launcher:         spec.Launcher.Kind,
		RequestedPackage: spec.Launcher.Command,
		RequestedVersion: spec.Launcher.Version,
		ExpiresAt:        time.Now().Add(24 * time.Hour),
	}
	plan.PlanDigest = plan.ComputeDigest()
	return plan, nil
}

func (p *defaultProvisioner) Prepare(ctx context.Context, plan mcp.MCPInstallPlan) error {
	return nil
}

type defaultInstaller struct {
	npx    *NPXInstaller
	uvx    *UVXInstaller
	exec   *ExecutableInstaller
	remote *RemoteInstaller
}

func NewDefaultInstaller() mcp.MCPInstaller {
	return &defaultInstaller{
		npx:    NewNPXInstaller(),
		uvx:    NewUVXInstaller(),
		exec:   NewExecutableInstaller(),
		remote: NewRemoteInstaller(),
	}
}

func (d *defaultInstaller) InstallNPX(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	_, err := exec.LookPath("npm")
	if err != nil {
		return nil, fmt.Errorf("MCP_INSTALL_FAILED: npm not found in PATH")
	}
	return d.npx.Install(ctx, plan, binding)
}

func (d *defaultInstaller) InstallUVX(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	_, err := exec.LookPath("uv")
	if err != nil {
		return nil, fmt.Errorf("MCP_INSTALL_FAILED: uv not found in PATH")
	}
	return d.uvx.Install(ctx, plan, binding)
}

func (d *defaultInstaller) InstallExecutable(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	return d.exec.Install(ctx, plan, binding)
}

func (d *defaultInstaller) InstallRemote(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	return d.remote.Install(ctx, plan, binding)
}
