package task_runtime

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

func hostWorkspaceRoot(profile runtimeprofile.Profile, configured string) (string, error) {
	if configured != "" {
		return filepath.Abs(configured)
	}
	if profile == runtimeprofile.ProfileCloudCore {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(cache, "amitia", "task-runtime"))
	}
	return filepath.Abs(os.TempDir())
}

func verifyWorkspaceRoot(root string, strict bool) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return NewTaskError(ErrTaskScopeDenied, "任务工作目录根路径必须是真实目录")
	}
	if strict && info.Mode().Perm()&0o077 != 0 {
		return NewTaskError(ErrTaskScopeDenied, "云端任务工作目录根路径必须仅由宿主用户访问")
	}
	if strict {
		return verifyExistingWorkspaceAncestors(root)
	}
	return nil
}

func verifyExistingWorkspaceAncestors(root string) error {
	for parent := filepath.Dir(root); parent != root; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if errors.Is(err, os.ErrNotExist) {

		} else if err != nil {
			return err
		} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return NewTaskError(ErrTaskScopeDenied, "云端任务工作目录不得经过符号链接父目录")
		}
		if parent == filepath.Dir(parent) {
			break
		}
	}
	return nil
}
