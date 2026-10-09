package installer

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/mcp"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
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
	npx      *NPXInstaller
	uvx      *UVXInstaller
	exec     *ExecutableInstaller
	remote   *RemoteInstaller
	resolver commandenv.Resolver
}

func NewDefaultInstaller(resolvers ...commandenv.Resolver) mcp.MCPInstaller {
	result := &defaultInstaller{
		npx:    NewNPXInstaller(),
		uvx:    NewUVXInstaller(),
		exec:   NewExecutableInstaller(),
		remote: NewRemoteInstaller(),
	}
	if len(resolvers) > 0 {
		result.resolver = resolvers[0]
	}
	return result
}

func (d *defaultInstaller) InstallNPX(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	var err error
	if d.resolver != nil {
		_, err = d.resolver.Resolve(ctx, commandenv.Request{Command: "npx", Args: binding.Launcher.Args})
	} else {
		_, err = exec.LookPath("npm")
	}
	if err != nil {
		return nil, fmt.Errorf("MCP_INSTALL_FAILED: npm runtime unavailable: %w", err)
	}
	return d.npx.Install(ctx, plan, binding)
}

func (d *defaultInstaller) InstallUVX(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	var err error
	if d.resolver != nil {
		_, err = d.resolver.Resolve(ctx, commandenv.Request{Command: "uvx", Args: binding.Launcher.Args})
	} else {
		_, err = exec.LookPath("uv")
	}
	if err != nil {
		return nil, fmt.Errorf("MCP_INSTALL_FAILED: Python uv runtime unavailable: %w", err)
	}
	return d.uvx.Install(ctx, plan, binding)
}

func (d *defaultInstaller) InstallExecutable(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	return d.exec.Install(ctx, plan, binding)
}

func (d *defaultInstaller) InstallRemote(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	return d.remote.Install(ctx, plan, binding)
}
