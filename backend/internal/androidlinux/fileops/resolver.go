//go:build linux && !android

package fileops

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/pkg/util"
)

type Resolver struct {
	paths  util.RuntimePaths
	policy Policy
}

func NewResolver(paths util.RuntimePaths, policy Policy) *Resolver {
	return &Resolver{
		paths:  paths,
		policy: policy,
	}
}

func (r *Resolver) ResolveRead(path string) (string, error) {
	resolved, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	return r.validateReadTarget(resolved)
}

func (r *Resolver) ResolveWrite(path string) (string, error) {
	resolved, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	return r.validateMutationTarget(resolved)
}

func (r *Resolver) ResolveCreate(path string) (string, error) {
	resolved, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	return r.validateMutationTarget(resolved)
}

func (r *Resolver) resolve(input string) (string, error) {
	if input == "" {
		if r.paths.WorkspaceDir != "" {
			return r.paths.WorkspaceDir, nil
		}
		return "/tmp", nil
	}

	if strings.HasPrefix(input, "amitia://") {
		return r.resolveResourceURI(input)
	}

	cleanPath := filepath.Clean(input)
	if !filepath.IsAbs(cleanPath) {
		if r.paths.WorkspaceDir != "" {
			cleanPath = filepath.Join(r.paths.WorkspaceDir, cleanPath)
		} else {
			cleanPath = filepath.Join("/tmp", cleanPath)
		}
	}

	return filepath.Clean(cleanPath), nil
}

func (r *Resolver) resolveResourceURI(uri string) (string, error) {
	withoutScheme := strings.TrimPrefix(uri, "amitia://")
	parts := strings.SplitN(withoutScheme, "/", 2)
	resourceType := parts[0]

	switch resourceType {
	case "workspace":
		relative := ""
		if len(parts) > 1 {
			relative = parts[1]
		}
		if r.paths.WorkspaceDir == "" {
			return "", ErrNotAvailable("workspace root not available")
		}
		return filepath.Join(r.paths.WorkspaceDir, filepath.Clean("/"+relative)), nil
	case "temp":
		if r.paths.TempDir == "" {
			return "", ErrNotAvailable("temp root not available")
		}
		relative := ""
		if len(parts) > 1 {
			relative = parts[1]
		}
		return filepath.Join(r.paths.TempDir, filepath.Clean("/"+relative)), nil
	default:
		return "", ErrPathDenied(uri, "unsupported resource type: "+resourceType)
	}
}

func (r *Resolver) validateReadTarget(path string) (string, error) {
	resolved, err := r.resolveAndCheckSymlink(path, false)
	if err != nil {
		return "", err
	}
	if resolved != "" {
		path = resolved
	}

	if !r.policy.AllowAbsolutePaths && !r.isUnderAllowedRoot(path) {
		return "", ErrPathDenied(path, "path outside allowed roots")
	}

	return path, nil
}

func (r *Resolver) validateMutationTarget(path string) (string, error) {
	resolved, err := r.resolveAndCheckSymlink(path, false)
	if err != nil {
		return "", err
	}
	if resolved != "" {
		path = resolved
	}

	if !r.isUnderAllowedRoot(path) {
		return "", ErrPathDenied(path, "path outside allowed roots")
	}

	for _, root := range r.policy.DeniedMutationRoots {
		if root != "" && isSubPath(path, root) {
			return "", ErrMutationRootDenied(path)
		}
	}

	return path, nil
}

func (r *Resolver) resolveAndCheckSymlink(path string, followSymlink bool) (string, error) {
	if !followSymlink {
		return "", nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		return "", nil
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}

	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", ErrIOFailed("failed to resolve symlink: " + err.Error())
	}

	if !r.isUnderAllowedRoot(target) {
		return "", ErrSymlinkDenied(path)
	}

	for _, root := range r.policy.DeniedMutationRoots {
		if root != "" && isSubPath(target, root) {
			return "", ErrSymlinkDenied(path)
		}
	}

	return target, nil
}

func (r *Resolver) isUnderAllowedRoot(path string) bool {
	if r.paths.WorkspaceDir != "" && isSubPath(path, r.paths.WorkspaceDir) {
		return true
	}
	if r.paths.TempDir != "" && isSubPath(path, r.paths.TempDir) {
		return true
	}
	if r.paths.CacheDir != "" && isSubPath(path, r.paths.CacheDir) {
		return true
	}
	return false
}

func isSubPath(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}
