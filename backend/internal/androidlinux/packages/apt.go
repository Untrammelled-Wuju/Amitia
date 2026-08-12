//go:build linux && !android

package packages

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/androidlinux/shell"
	"github.com/u-ai/backend/pkg/util"
)

type AptManager struct {
	executor    shell.ShellExecutor
	policy      PackagesPolicy
	paths       util.RuntimePaths
	dpkgLock    chan struct{}
}

func NewAptManager(executor shell.ShellExecutor, policy PackagesPolicy, paths util.RuntimePaths) *AptManager {
	return &AptManager{
		executor: executor,
		policy:   policy,
		paths:    paths,
		dpkgLock: make(chan struct{}, 1),
	}
}

func (m *AptManager) acquireLock() bool {
	select {
	case m.dpkgLock <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *AptManager) releaseLock() {
	select {
	case <-m.dpkgLock:
	default:
	}
}

func (m *AptManager) Update(ctx context.Context, timeoutMs int64) (*PackageInstallResult, error) {
	if !m.acquireLock() {
		return nil, ErrManagerBusy("apt-get")
	}
	defer m.releaseLock()

	effectiveTimeout := m.policy.DefaultInstallTimeout.Milliseconds()
	if timeoutMs > 0 && timeoutMs < effectiveTimeout {
		effectiveTimeout = timeoutMs
	}

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "apt-get",
		Args:       []string{"update"},
		TimeoutMs:  effectiveTimeout,
		Environment: map[string]string{
			"DEBIAN_FRONTEND": "noninteractive",
		},
	})

	if result.TimedOut {
		return nil, ErrTimeout("apt-update")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("apt-update")
	}

	return &PackageInstallResult{
		Manager:    "apt",
		ExitCode:   result.ExitCode,
		DurationMs: result.DurationMs,
	}, nil
}

func (m *AptManager) Install(ctx context.Context, packages []string, timeoutMs int64) (*PackageInstallResult, error) {
	if !m.acquireLock() {
		return nil, ErrManagerBusy("apt-get")
	}
	defer m.releaseLock()

	for _, pkg := range packages {
		if err := ValidateDebianPackageName(pkg); err != nil {
			return nil, err
		}
	}

	effectiveTimeout := m.policy.DefaultInstallTimeout.Milliseconds()
	if timeoutMs > 0 && timeoutMs < effectiveTimeout {
		effectiveTimeout = timeoutMs
	}

	args := []string{"-y"}
	if m.policy.InstallNoRecommends {
		args = append(args, "--no-install-recommends")
	}
	args = append(args, "install")
	args = append(args, packages...)

	result := m.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "apt-get",
		Args:       args,
		TimeoutMs:  effectiveTimeout,
		Environment: map[string]string{
			"DEBIAN_FRONTEND": "noninteractive",
		},
	})

	if result.TimedOut {
		return nil, ErrTimeout("apt-install")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled("apt-install")
	}

	res := &PackageInstallResult{
		Manager:    "apt",
		Requested:  packages,
		ExitCode:   result.ExitCode,
		DurationMs: result.DurationMs,
	}

	if result.ExitCode != 0 {
		if strings.Contains(result.Stderr, "E: Unable to locate package") {
			return res, ErrPackageInstallFailed(strings.Join(packages, ","), "package not found")
		}
		if strings.Contains(result.Stderr, "dpkg was interrupted") {
			return res, ErrManagerRecovery()
		}
		return res, ErrPackageInstallFailed(strings.Join(packages, ","), "exit code "+itoa(result.ExitCode))
	}

	for _, pkg := range packages {
		parsed := ParseDebianPackageSpec(pkg)
		installed, err := m.queryPackage(parsed.Name)
		if err == nil {
			res.Installed = append(res.Installed, *installed)
		}
	}

	return res, nil
}

func (m *AptManager) Query(ctx context.Context, packages []string) (*PackageInstallResult, error) {
	res := &PackageInstallResult{
		Manager:   "apt",
		Requested: packages,
	}

	for _, pkg := range packages {
		parsed := ParseDebianPackageSpec(pkg)
		installed, err := m.queryPackage(parsed.Name)
		if err == nil {
			res.Installed = append(res.Installed, *installed)
		}
	}

	res.ExitCode = 0
	return res, nil
}

func (m *AptManager) queryPackage(name string) (*InstalledPackage, error) {
	return &InstalledPackage{
		Name:    name,
		Version: "unknown",
	}, nil
}

func (m *AptManager) ManagedVenvPath() string {
	if m.paths.DataDir == "" {
		return ""
	}
	return filepath.Join(m.paths.DataDir, m.policy.PythonVenvBaseDir)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
