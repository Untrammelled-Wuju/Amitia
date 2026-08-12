//go:build linux && !android

package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/androidlinux/shell"
)

type PythonVenvManager struct {
	executor    shell.ShellExecutor
	venvPath    string
	policy      PackagesPolicy
}

func NewPythonVenvManager(executor shell.ShellExecutor, venvPath string, policy PackagesPolicy) *PythonVenvManager {
	return &PythonVenvManager{
		executor: executor,
		venvPath: venvPath,
		policy:   policy,
	}
}

func (m *PythonVenvManager) VenvExists() bool {
	if m.venvPath == "" {
		return false
	}
	pythonPath := filepath.Join(m.venvPath, "bin", "python")
	info, err := os.Stat(pythonPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (m *PythonVenvManager) PythonPath() string {
	return filepath.Join(m.venvPath, "bin", "python")
}

func (m *PythonVenvManager) EnsureVenv(ctx context.Context, timeoutMs int64) error {
	if m.VenvExists() {
		return nil
	}

	if m.venvPath == "" {
		return ErrPythonVenvCreateFailed("venv path not configured")
	}

	if err := os.MkdirAll(m.venvPath, 0755); err != nil {
		return ErrPythonVenvCreateFailed(fmt.Sprintf("mkdir failed: %v", err))
	}

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "python3",
		Args:       []string{"-m", "venv", m.venvPath},
		TimeoutMs:  timeoutMs,
	})

	if result.ExitCode != 0 || result.TimedOut {
		return ErrPythonVenvCreateFailed(fmt.Sprintf("exit=%d: %s", result.ExitCode, result.Stderr))
	}

	return nil
}

func (m *PythonVenvManager) InstallPackage(ctx context.Context, pkg string, timeoutMs int64) (*PackageInstallResult, error) {
	if err := ValidatePythonPackageSpec(pkg); err != nil {
		return nil, err
	}

	if !m.VenvExists() {
		if err := m.EnsureVenv(ctx, m.policy.DefaultInstallTimeout.Milliseconds()); err != nil {
			return nil, err
		}
	}

	effectiveTimeout := m.policy.DefaultInstallTimeout.Milliseconds()
	if timeoutMs > 0 && timeoutMs < effectiveTimeout {
		effectiveTimeout = timeoutMs
	}

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: m.PythonPath(),
		Args:       []string{"-m", "pip", "install", pkg},
		TimeoutMs:  effectiveTimeout,
		Environment: map[string]string{
			"PIP_DISABLE_PIP_VERSION_CHECK": "1",
			"PIP_NO_WARNINGS":               "off",
		},
	})

	if result.TimedOut {
		return nil, ErrTimeout("pip-install")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("pip-install")
	}

	res := &PackageInstallResult{
		Manager:    "pip",
		Requested:  []string{pkg},
		ExitCode:   result.ExitCode,
		DurationMs: result.DurationMs,
	}

	if result.ExitCode != 0 {
		return res, ErrPythonPackageInstallFailed(pkg, result.Stderr)
	}

	return res, nil
}

func (m *PythonVenvManager) Run(ctx context.Context, args []string, workingDir string, stdin string, timeoutMs int64) (*InvokeResult, error) {
	if !m.VenvExists() {
		return &InvokeResult{ExitCode: 1, Stderr: ErrPythonNotFound().Error()}, ErrPythonNotFound()
	}

	effectiveTimeout := m.policy.DefaultInvokeTimeout.Milliseconds()
	if timeoutMs > 0 && timeoutMs < effectiveTimeout {
		effectiveTimeout = timeoutMs
	}

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: m.PythonPath(),
		Args:       args,
		WorkingDir: workingDir,
		Stdin:      stdin,
		TimeoutMs:  effectiveTimeout,
	})

	if result.TimedOut {
		return nil, ErrTimeout("python-invoke")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("python-invoke")
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
