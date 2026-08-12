//go:build linux && !android

package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/androidlinux/shell"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/pkg/util"
)

type NodeManager struct {
	executor    shell.ShellExecutor
	resolver    nodeenv.Resolver
	policy      PackagesPolicy
	paths       util.RuntimePaths
	nodeEnv     nodeenv.Environment
}

func NewNodeManager(executor shell.ShellExecutor, resolver nodeenv.Resolver, policy PackagesPolicy, paths util.RuntimePaths) *NodeManager {
	return &NodeManager{
		executor: executor,
		resolver: resolver,
		policy:   policy,
		paths:    paths,
	}
}

func (m *NodeManager) resolveEnv(ctx context.Context) error {
	if m.nodeEnv.NodeBinary != "" {
		return nil
	}
	if m.resolver == nil {
		return ErrNodeEnvironmentUnavailable()
	}

	env, err := m.resolver.Resolve(ctx)
	if err != nil {
		return ErrNodeUnavailable()
	}
	m.nodeEnv = env
	return nil
}

func (m *NodeManager) Status(ctx context.Context, timeoutMs int64) (NodeStatus, error) {
	status := NodeStatus{}

	if err := m.resolveEnv(ctx); err != nil {
		if m.resolver != nil {
			if env, rerr := m.resolver.Resolve(ctx); rerr == nil {
				m.nodeEnv = env
			} else {
				return status, nil
			}
		}
		return status, nil
	}

	if m.nodeEnv.NodeBinary == "" {
		return status, nil
	}

	status.Available = true
	status.Source = string(m.nodeEnv.Source)
	status.Architecture = m.nodeEnv.Architecture

	verResult := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: m.nodeEnv.NodeBinary,
		Args:       []string{"--version"},
		TimeoutMs:  timeoutMs,
	})

	if verResult.ExitCode == 0 && !verResult.TimedOut {
		status.Version = strings.TrimSpace(verResult.Stdout)
	}

	status.NPMAvailable = m.nodeEnv.NPMCLI != ""
	status.NPXAvailable = m.nodeEnv.NPXCLI != ""
	status.PackageManagementAvailable = m.nodeEnv.PackageManagementAvailable

	if status.NPMAvailable && m.nodeEnv.NPMCLI != "" {
		npmResult := m.executor.Execute(ctx, shell.ShellExecuteRequest{
			Mode:       shell.ShellModeArgv,
			Executable: m.nodeEnv.NodeBinary,
			Args:       []string{m.nodeEnv.NPMCLI, "--version"},
			TimeoutMs:  timeoutMs,
		})
		if npmResult.ExitCode == 0 && !npmResult.TimedOut {
			status.NPMVersion = strings.TrimSpace(npmResult.Stdout)
		}
	}

	if status.NPXAvailable && m.nodeEnv.NPXCLI != "" {
		npxResult := m.executor.Execute(ctx, shell.ShellExecuteRequest{
			Mode:       shell.ShellModeArgv,
			Executable: m.nodeEnv.NodeBinary,
			Args:       []string{m.nodeEnv.NPXCLI, "--version"},
			TimeoutMs:  timeoutMs,
		})
		if npxResult.ExitCode == 0 && !npxResult.TimedOut {
			status.NPXVersion = strings.TrimSpace(npxResult.Stdout)
		}
	}

	return status, nil
}

func (m *NodeManager) Invoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error) {
	if err := m.resolveEnv(ctx); err != nil {
		return nil, err
	}

	if m.nodeEnv.NodeBinary == "" {
		return &InvokeResult{ExitCode: 1, Stderr: ErrNodeEnvironmentUnavailable().Error()}, ErrNodeEnvironmentUnavailable()
	}

	effectiveTimeout := m.policy.DefaultInvokeTimeout.Milliseconds()
	if req.TimeoutMs > 0 && req.TimeoutMs < effectiveTimeout {
		effectiveTimeout = req.TimeoutMs
	}

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: m.nodeEnv.NodeBinary,
		Args:       req.Args,
		WorkingDir: req.WorkingDir,
		Stdin:      req.Stdin,
		TimeoutMs:  effectiveTimeout,
	})

	if result.TimedOut {
		return nil, ErrTimeout("node-invoke")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("node-invoke")
	}

	return &InvokeResult{
		ExitCode:        result.ExitCode,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		DurationMs:      result.DurationMs,
		TimedOut:        result.TimedOut,
		Signal:          result.Signal,
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
	}, nil
}

func (m *NodeManager) Install(ctx context.Context, req NodePackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	if err := m.resolveEnv(ctx); err != nil {
		return nil, err
	}

	for _, pkg := range req.Packages {
		if err := ValidateNpmPackageSpec(pkg); err != nil {
			return nil, err
		}
	}

	if m.nodeEnv.NPMCLI == "" {
		return nil, ErrNpmUnavailable()
	}

	effectiveTimeout := m.policy.DefaultInstallTimeout.Milliseconds()
	if timeoutMs > 0 && timeoutMs < effectiveTimeout {
		effectiveTimeout = timeoutMs
	}

	nodePkgRoot := m.ManagedNodePkgPath()
	if nodePkgRoot != "" {
		if err := os.MkdirAll(nodePkgRoot, 0755); err != nil {
			return nil, ErrNpmPackageInstallFailed(strings.Join(req.Packages, ","), fmt.Sprintf("mkdir failed: %v", err))
		}
	}

	args := []string{m.nodeEnv.NPMCLI, "install", "--ignore-scripts", "--no-audit", "--no-fund"}
	if nodePkgRoot != "" {
		args = append(args, "--prefix", nodePkgRoot)
	}
	args = append(args, req.Packages...)

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: m.nodeEnv.NodeBinary,
		Args:       args,
		TimeoutMs:  effectiveTimeout,
	})

	if result.TimedOut {
		return nil, ErrTimeout("npm-install")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("npm-install")
	}

	res := &PackageInstallResult{
		Manager:    "npm",
		Requested:  req.Packages,
		ExitCode:   result.ExitCode,
		DurationMs: result.DurationMs,
	}

	if result.ExitCode != 0 {
		return res, ErrNpmPackageInstallFailed(strings.Join(req.Packages, ","), result.Stderr)
	}

	return res, nil
}

func (m *NodeManager) Npx(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error) {
	if err := m.resolveEnv(ctx); err != nil {
		return nil, err
	}

	if m.nodeEnv.NPXCLI == "" {
		return &InvokeResult{ExitCode: 1, Stderr: ErrNpxUnavailable().Error()}, ErrNpxUnavailable()
	}

	effectiveTimeout := m.policy.DefaultInvokeTimeout.Milliseconds()
	if req.TimeoutMs > 0 && req.TimeoutMs < effectiveTimeout {
		effectiveTimeout = req.TimeoutMs
	}

	fullArgs := []string{m.nodeEnv.NPXCLI, "--no-install"}
	fullArgs = append(fullArgs, req.Args...)

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: m.nodeEnv.NodeBinary,
		Args:       fullArgs,
		WorkingDir: req.WorkingDir,
		Stdin:      req.Stdin,
		TimeoutMs:  effectiveTimeout,
	})

	if result.TimedOut {
		return nil, ErrTimeout("npx-invoke")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("npx-invoke")
	}

	return &InvokeResult{
		ExitCode:        result.ExitCode,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		DurationMs:      result.DurationMs,
		TimedOut:        result.TimedOut,
		Signal:          result.Signal,
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
	}, nil
}

func (m *NodeManager) ManagedNodePkgPath() string {
	if m.paths.DataDir == "" {
		return ""
	}
	return filepath.Join(m.paths.DataDir, m.policy.NodePackageBaseDir)
}
