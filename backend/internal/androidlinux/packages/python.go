//go:build linux && !android

package packages

import (
	"context"

	"github.com/u-ai/backend/internal/androidlinux/shell"
)

type PythonManager struct {
	detector    *PythonDetector
	venvMgr     *PythonVenvManager
	executor    shell.ShellExecutor
}

func NewPythonManager(detector *PythonDetector, venvMgr *PythonVenvManager, executor shell.ShellExecutor) *PythonManager {
	return &PythonManager{
		detector: detector,
		venvMgr:  venvMgr,
		executor: executor,
	}
}

func (m *PythonManager) Status(ctx context.Context, timeoutMs int64) (PythonStatus, error) {
	return m.detector.Detect(ctx, timeoutMs)
}

func (m *PythonManager) Invoke(ctx context.Context, req PythonInvokeRequest) (*InvokeResult, error) {
	return m.venvMgr.Run(ctx, req.Args, req.WorkingDir, req.Stdin, req.TimeoutMs)
}

func (m *PythonManager) Install(ctx context.Context, req PythonPackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	if len(req.Packages) == 0 {
		return nil, ErrInvalidPythonPackageSpec("<empty>")
	}

	res := &PackageInstallResult{
		Manager:   "pip",
		Requested: req.Packages,
	}

	for _, pkg := range req.Packages {
		result, err := m.venvMgr.InstallPackage(ctx, pkg, timeoutMs)
		if err != nil {
			return res, err
		}
		res.ExitCode = result.ExitCode
		res.DurationMs += result.DurationMs
		res.Installed = append(res.Installed, result.Installed...)
	}

	return res, nil
}

func (m *PythonManager) VenvPath() string {
	return m.venvMgr.venvPath
}
