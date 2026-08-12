// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package installer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/mcp"
)

var revCounter uint64

type InstallScriptApproval struct {
	Package    string `json:"package"`
	Version    string `json:"version"`
	ScriptHash string `json:"scriptHash"`
	ApprovedBy string `json:"approvedBy"`
}

type InstallResult struct {
	Revision    *mcp.MCPRevision
	HasScripts  bool
	ScriptTypes []string
}

type NPXInstaller struct {
	mu       sync.Mutex
	bindings map[string]*mcp.MCPInstallation
}

func NewNPXInstaller() *NPXInstaller {
	return &NPXInstaller{
		bindings: make(map[string]*mcp.MCPInstallation),
	}
}

func (i *NPXInstaller) Install(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	if !plan.VerifyDigest() {
		return nil, &mcp.PlanChangedError{PlanID: plan.PlanID}
	}

	if err := ValidateNPXPlan(plan); err != nil {
		return nil, err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	rev := &mcp.MCPRevision{
		RevisionID:         GenerateRevisionID(),
		BindingID:          binding.ID,
		LauncherKind:       string(mcp.MCPLauncherNPX),
		RequestedSpecJSON:  plan.RequestedPackage + "@" + plan.RequestedVersion,
		PackageManager:     "npm",
		InstallRootURI:     fmt.Sprintf("extensions/mcp/%s/revisions/%s/node", binding.ID, GenerateRevisionID()),
		EntryPoint:         binding.Launcher.Command,
		RuntimeFingerprint: "",
		Validated:          true,
	}

	return rev, nil
}

type UVXInstaller struct {
	mu       sync.Mutex
	bindings map[string]*mcp.MCPInstallation
}

func NewUVXInstaller() *UVXInstaller {
	return &UVXInstaller{
		bindings: make(map[string]*mcp.MCPInstallation),
	}
}

func (i *UVXInstaller) Install(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	if !plan.VerifyDigest() {
		return nil, &mcp.PlanChangedError{PlanID: plan.PlanID}
	}

	if err := ValidateUVXPlan(plan); err != nil {
		return nil, err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	rev := &mcp.MCPRevision{
		RevisionID:         GenerateRevisionID(),
		BindingID:          binding.ID,
		LauncherKind:       string(mcp.MCPLauncherUVX),
		RequestedSpecJSON:  plan.RequestedPackage + plan.RequestedVersion,
		PackageManager:     "uv",
		InstallRootURI:     fmt.Sprintf("extensions/mcp/%s/revisions/%s/uv", binding.ID, GenerateRevisionID()),
		EntryPoint:         binding.Launcher.Command,
		RuntimeFingerprint: "",
		Validated:          true,
	}

	return rev, nil
}

type ExecutableInstaller struct{}

func NewExecutableInstaller() *ExecutableInstaller {
	return &ExecutableInstaller{}
}

func (i *ExecutableInstaller) Install(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	if !plan.VerifyDigest() {
		return nil, &mcp.PlanChangedError{PlanID: plan.PlanID}
	}

	if binding.Launcher == nil || binding.Launcher.Command == "" {
		return nil, fmt.Errorf("MCP_ENTRYPOINT_NOT_FOUND: executable command is required")
	}

	rev := &mcp.MCPRevision{
		RevisionID:         GenerateRevisionID(),
		BindingID:          binding.ID,
		LauncherKind:       string(mcp.MCPLauncherExecutable),
		RequestedSpecJSON:  binding.Launcher.Command,
		PackageManager:     "",
		InstallRootURI:     "",
		EntryPoint:         binding.Launcher.Command,
		RuntimeFingerprint: "",
		Validated:          true,
	}

	return rev, nil
}

type RemoteInstaller struct{}

func NewRemoteInstaller() *RemoteInstaller {
	return &RemoteInstaller{}
}

func (i *RemoteInstaller) Install(ctx context.Context, plan mcp.MCPInstallPlan, binding mcp.MCPBinding) (*mcp.MCPRevision, error) {
	if !plan.VerifyDigest() {
		return nil, &mcp.PlanChangedError{PlanID: plan.PlanID}
	}

	rev := &mcp.MCPRevision{
		RevisionID:         GenerateRevisionID(),
		BindingID:          binding.ID,
		LauncherKind:       "remote",
		RequestedSpecJSON:  plan.Source,
		PackageManager:     "",
		InstallRootURI:     "",
		EntryPoint:         "",
		RuntimeFingerprint: "",
		Validated:          true,
	}

	return rev, nil
}

func ValidateNPXPlan(plan mcp.MCPInstallPlan) error {
	if plan.RequestedPackage == "" {
		return fmt.Errorf("MCP_PACKAGE_SPEC_INVALID: package name is required")
	}
	if plan.RequestedVersion == "" {
		return mcp.ErrMCPPackageVersionUnpinned
	}
	if plan.RequestedVersion == "latest" {
		return mcp.ErrMCPPackageVersionUnpinned
	}
	for _, c := range plan.RequestedVersion {
		if c == '^' || c == '~' || c == '*' {
			return mcp.ErrMCPPackageVersionUnpinned
		}
	}
	return nil
}

func ValidateUVXPlan(plan mcp.MCPInstallPlan) error {
	if plan.RequestedPackage == "" {
		return fmt.Errorf("MCP_PACKAGE_SPEC_INVALID: package name is required")
	}
	if plan.RequestedVersion == "" {
		return mcp.ErrMCPPackageVersionUnpinned
	}
	for _, c := range plan.RequestedVersion {
		if c == '^' || c == '~' || c == '*' || c == '>' || c == '<' {
			return mcp.ErrMCPPackageVersionUnpinned
		}
	}
	return nil
}

func GenerateRevisionID() string {
	c := atomic.AddUint64(&revCounter, 1)
	return fmt.Sprintf("rev_%d_%d", time.Now().UnixNano(), c)
}
