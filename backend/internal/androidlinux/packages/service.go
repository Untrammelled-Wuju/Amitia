//go:build linux && !android

package packages

import (
	"context"
	"path/filepath"

	"github.com/u-ai/backend/internal/androidlinux/shell"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/pkg/util"
)

type PackageService interface {
	Status(ctx context.Context) (RuntimePackagesStatus, error)
	AptUpdate(ctx context.Context, timeoutMs int64) (*PackageInstallResult, error)
	AptInstall(ctx context.Context, req AptInstallRequest, timeoutMs int64) (*PackageInstallResult, error)
	AptQuery(ctx context.Context, packages []string) (*PackageInstallResult, error)
	PythonStatus(ctx context.Context, timeoutMs int64) (PythonStatus, error)
	PythonInvoke(ctx context.Context, req PythonInvokeRequest) (*InvokeResult, error)
	PythonInstall(ctx context.Context, req PythonPackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error)
	NodeStatus(ctx context.Context, timeoutMs int64) (NodeStatus, error)
	NodeInvoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error)
	NodeInstall(ctx context.Context, req NodePackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error)
	NpxInvoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error)
}

type PackageRuntimeService struct {
	executor     shell.ShellExecutor
	nodeResolver nodeenv.Resolver
	policy       PackagesPolicy
	paths        util.RuntimePaths

	aptDetector    *AptDetector
	pythonDetector *PythonDetector
	venvMgr        *PythonVenvManager
	nodeMgr        *NodeManager
}

func NewPackageRuntimeService(
	executor shell.ShellExecutor,
	nodeResolver nodeenv.Resolver,
	policy PackagesPolicy,
	paths util.RuntimePaths,
) *PackageRuntimeService {
	svc := &PackageRuntimeService{
		executor:     executor,
		nodeResolver: nodeResolver,
		policy:       policy,
		paths:        paths,
	}

	svc.aptDetector = NewAptDetector(executor)
	svc.pythonDetector = NewPythonDetector(executor)

	venvPath := filepath.Join(paths.DataDir, policy.PythonVenvBaseDir)
	svc.venvMgr = NewPythonVenvManager(executor, venvPath, policy)

	svc.nodeMgr = NewNodeManager(executor, nodeResolver, policy, paths)

	return svc
}

func (s *PackageRuntimeService) Status(ctx context.Context) (RuntimePackagesStatus, error) {
	status := RuntimePackagesStatus{
		Supported: s.executor != nil,
	}

	if !s.policy.Enabled {
		return status, nil
	}

	statusTimeout := s.policy.StatusTimeout.Milliseconds()

	if s.policy.AptEnabled {
		status.Apt, _ = s.aptDetector.Detect(ctx, statusTimeout)
	}
	if s.policy.PythonEnabled {
		status.Python, _ = s.pythonDetector.Detect(ctx, statusTimeout)
	}
	if s.policy.NodeEnabled {
		status.Node, _ = s.nodeMgr.Status(ctx, statusTimeout)
	}

	return status, nil
}

func (s *PackageRuntimeService) AptUpdate(ctx context.Context, timeoutMs int64) (*PackageInstallResult, error) {
	mgr := NewAptManager(s.executor, s.policy, s.paths)
	return mgr.Update(ctx, timeoutMs)
}

func (s *PackageRuntimeService) AptInstall(ctx context.Context, req AptInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	mgr := NewAptManager(s.executor, s.policy, s.paths)
	return mgr.Install(ctx, req.Packages, timeoutMs)
}

func (s *PackageRuntimeService) AptQuery(ctx context.Context, packages []string) (*PackageInstallResult, error) {
	mgr := NewAptManager(s.executor, s.policy, s.paths)
	return mgr.Query(ctx, packages)
}

func (s *PackageRuntimeService) PythonStatus(ctx context.Context, timeoutMs int64) (PythonStatus, error) {
	return s.pythonDetector.Detect(ctx, timeoutMs)
}

func (s *PackageRuntimeService) PythonInvoke(ctx context.Context, req PythonInvokeRequest) (*InvokeResult, error) {
	return s.venvMgr.Run(ctx, req.Args, req.WorkingDir, req.Stdin, req.TimeoutMs)
}

func (s *PackageRuntimeService) PythonInstall(ctx context.Context, req PythonPackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	res := &PackageInstallResult{
		Manager:   "pip",
		Requested: req.Packages,
	}

	for _, pkg := range req.Packages {
		result, err := s.venvMgr.InstallPackage(ctx, pkg, timeoutMs)
		if err != nil {
			return res, err
		}
		res.ExitCode = result.ExitCode
		res.DurationMs += result.DurationMs
	}

	return res, nil
}

func (s *PackageRuntimeService) NodeStatus(ctx context.Context, timeoutMs int64) (NodeStatus, error) {
	return s.nodeMgr.Status(ctx, timeoutMs)
}

func (s *PackageRuntimeService) NodeInvoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error) {
	return s.nodeMgr.Invoke(ctx, req)
}

func (s *PackageRuntimeService) NodeInstall(ctx context.Context, req NodePackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	return s.nodeMgr.Install(ctx, req, timeoutMs)
}

func (s *PackageRuntimeService) NpxInvoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error) {
	return s.nodeMgr.Npx(ctx, req)
}
