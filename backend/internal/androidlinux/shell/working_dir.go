//go:build linux && !android

package shell

import (
	"path/filepath"
	"strings"
)

type WorkingDirResolver struct {
	workspaceRoot string
	tempRoot      string
}

func NewWorkingDirResolver(workspaceRoot, tempRoot string) *WorkingDirResolver {
	return &WorkingDirResolver{
		workspaceRoot: workspaceRoot,
		tempRoot:      tempRoot,
	}
}

func (r *WorkingDirResolver) Resolve(input string) (string, error) {
	if input == "" {
		if r.workspaceRoot != "" {
			return r.workspaceRoot, nil
		}
		return "/tmp", nil
	}

	if strings.HasPrefix(input, "amitia://") {
		return r.resolveResourceURI(input)
	}

	cleanPath := filepath.Clean(input)
	if !filepath.IsAbs(cleanPath) {
		if r.workspaceRoot != "" {
			cleanPath = filepath.Join(r.workspaceRoot, cleanPath)
		} else {
			cleanPath = filepath.Join("/tmp", cleanPath)
		}
	}

	cleanPath = filepath.Clean(cleanPath)

	if r.workspaceRoot != "" && !r.isUnderRoot(cleanPath, r.workspaceRoot) {
		if r.tempRoot == "" || !r.isUnderRoot(cleanPath, r.tempRoot) {
			return "", ErrWorkingDirInputRestricted(input)
		}
	}

	return cleanPath, nil
}

func (r *WorkingDirResolver) resolveResourceURI(uri string) (string, error) {
	withoutScheme := strings.TrimPrefix(uri, "amitia://")

	parts := strings.SplitN(withoutScheme, "/", 2)
	resourceType := parts[0]

	switch resourceType {
	case "workspace":
		relative := ""
		if len(parts) > 1 {
			relative = parts[1]
		}
		if r.workspaceRoot == "" {
			return "", ErrWorkingDirNoWorkspace()
		}
		return filepath.Join(r.workspaceRoot, filepath.Clean("/"+relative)), nil
	case "temp":
		if r.tempRoot == "" {
			return "", ErrWorkingDirNoTempRoot()
		}
		relative := ""
		if len(parts) > 1 {
			relative = parts[1]
		}
		return filepath.Join(r.tempRoot, filepath.Clean("/"+relative)), nil
	default:
		return "", ErrWorkingDirUnsupportedType(resourceType)
	}
}

func (r *WorkingDirResolver) isUnderRoot(path, root string) bool {
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

func ErrWorkingDirInputRestricted(path string) *Error {
	return &Error{code: ErrCodeWorkingDirInvalid, message: "working directory path restricted: " + path}
}

func ErrWorkingDirNoWorkspace() *Error {
	return &Error{code: ErrCodeWorkingDirInvalid, message: "workspace root not available"}
}

func ErrWorkingDirNoTempRoot() *Error {
	return &Error{code: ErrCodeWorkingDirInvalid, message: "temp root not available"}
}

func ErrWorkingDirUnsupportedType(t string) *Error {
	return &Error{code: ErrCodeWorkingDirInvalid, message: "unsupported resource type: " + t}
}
