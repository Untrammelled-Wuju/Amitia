package task_runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

func taskNodeLaunchArgs(profile runtimeprofile.Profile, cfg ProcessHostConfig) ([]string, error) {
	args := []string{"--max-old-space-size=128"}
	if profile != runtimeprofile.ProfileCloudCore || cfg.RequireSandbox {
		return append(args, cfg.HostPath), nil
	}
	readRoot := cfg.BundleRoot
	if readRoot == "" {
		if cfg.BundleHash != "" {
			return nil, NewTaskError(ErrTaskScopeDenied, "云端任务的文件树身份不完整")
		}
		readRoot = filepath.Dir(cfg.EntryPath)
	} else if cfg.BundleHash == "" {
		return nil, NewTaskError(ErrTaskScopeDenied, "云端任务的文件树身份不完整")
	}
	if readRoot == filepath.VolumeName(readRoot)+string(filepath.Separator) {
		return nil, NewTaskError(ErrTaskScopeDenied, "云端插件可读目录不得为文件系统根目录")
	}
	for _, path := range []string{cfg.NodePath, cfg.HostPath, cfg.EntryPath, readRoot, cfg.WorkDir} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "*?[]{},\r\n") {
			return nil, NewTaskError(ErrTaskScopeDenied, "云端任务执行路径必须为无通配符的绝对路径")
		}
		if _, err := os.Lstat(path); err != nil {
			return nil, fmt.Errorf("cloud task path is unavailable: %w", err)
		}
		if err := verifyExistingWorkspaceAncestors(path); err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, NewTaskError(ErrTaskScopeDenied, "云端执行不接受链接路径")
		}
		if path == cfg.WorkDir && (!info.IsDir() || info.Mode().Perm()&0o077 != 0) {
			return nil, NewTaskError(ErrTaskScopeDenied, "云端任务工作目录必须为私有目录")
		}
	}
	hostRoot := filepath.Dir(cfg.HostPath)
	if err := verifyExistingWorkspaceAncestors(hostRoot); err != nil {
		return nil, err
	}
	if sameOrNestedTaskPath(hostRoot, cfg.WorkDir) || sameOrNestedTaskPath(readRoot, cfg.WorkDir) || sameOrNestedTaskPath(cfg.WorkDir, hostRoot) || sameOrNestedTaskPath(cfg.WorkDir, readRoot) {
		return nil, NewTaskError(ErrTaskScopeDenied, "云端可写工作区不得与插件和运行时源码重叠")
	}
	args = append(args,
		"--permission",
		"--allow-fs-read="+hostRoot,
		"--allow-fs-read="+readRoot,
		"--allow-fs-read="+cfg.WorkDir,
		"--allow-fs-write="+cfg.WorkDir,
		cfg.HostPath,
	)
	return args, nil
}

func sameOrNestedTaskPath(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	return err == nil && (rel == "." || rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
