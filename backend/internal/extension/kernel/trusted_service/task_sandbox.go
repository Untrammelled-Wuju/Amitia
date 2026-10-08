package trusted_service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/u-ai/backend/internal/platform/process"
)

type TaskSandboxLaunch struct {
	Path             string
	Args             []string
	WorkingDir       string
	SupervisorLimits process.ResourceLimits
	ExtraFiles       []*os.File
	AfterStart       func(context.Context, *exec.Cmd) error
	Cleanup          func()
}

func ValidateTaskSandboxPrerequisites() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("%w: 当前平台任务隔离启动器不可用", ErrNetworkSandboxUnavailable)
	}
	return ValidateNetworkSandboxPrerequisites(ServiceNetworkPolicy{Mode: "none", Enforce: true})
}

func PrepareTaskSandbox(executable string, args []string, workingDir, hostRoot, bundleRoot string, limits process.ResourceLimits) (TaskSandboxLaunch, error) {
	empty := TaskSandboxLaunch{}
	if limits.MaxMemoryBytes == 0 || limits.MaxMemoryBytes > 512<<20 || limits.MaxCPUPercent == 0 || limits.MaxCPUPercent > 50 || limits.MaxProcesses != 1 {
		return empty, fmt.Errorf("%w: 任务资源预算无效", ErrNetworkSandboxUnavailable)
	}
	paths := []string{executable, workingDir, hostRoot, bundleRoot}
	for index, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) == filepath.VolumeName(path)+string(filepath.Separator) {
			return empty, fmt.Errorf("%w: 任务隔离路径无效", ErrNetworkSandboxUnavailable)
		}
		for ancestor := filepath.Clean(path); ; ancestor = filepath.Dir(ancestor) {
			info, err := os.Lstat(ancestor)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return empty, fmt.Errorf("%w: 任务隔离路径不可用或经过链接", ErrNetworkSandboxUnavailable)
			}
			if ancestor == filepath.Dir(ancestor) {
				break
			}
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return empty, fmt.Errorf("%w: 任务隔离路径无法解析", ErrNetworkSandboxUnavailable)
		}
		paths[index] = filepath.Clean(resolved)
	}
	executable, workingDir, hostRoot, bundleRoot = paths[0], paths[1], paths[2], paths[3]
	for _, root := range []string{hostRoot, bundleRoot} {
		if pathWithin(root, workingDir) || pathWithin(workingDir, root) {
			return empty, fmt.Errorf("%w: 任务工作区与只读源码范围重叠", ErrNetworkSandboxUnavailable)
		}
	}
	if runtime.GOOS != "windows" {
		return empty, fmt.Errorf("%w: 当前平台尚无同时验证任务隔离和子进程预算的启动器", ErrNetworkSandboxUnavailable)
	}
	tempDir := filepath.Join(workingDir, ".runtime-tmp")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return empty, err
	}
	plan, err := prepareWindowsTaskContainerLaunch(executable, args, workingDir, tempDir, limits, hostRoot, bundleRoot)
	if err != nil {
		return empty, err
	}
	if !plan.FilesystemIsolated || !plan.NetworkPolicyEnforced {
		plan.cleanup()
		return empty, ErrNetworkSandboxUnavailable
	}
	outer := limits
	outer.MaxProcesses = 4
	outer.MaxMemoryBytes += 256 << 20
	return TaskSandboxLaunch{Path: plan.Path, Args: plan.Args, WorkingDir: plan.WorkingDir, SupervisorLimits: outer, ExtraFiles: plan.ExtraFiles, AfterStart: plan.AfterStart, Cleanup: plan.Cleanup}, nil
}
