package util

import (
	"os"
	"path/filepath"
)

func RuntimeRoot() string {
	candidates := make([]string, 0, 4)

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
		candidates = append(candidates, filepath.Dir(filepath.Dir(exe)))
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
		candidates = append(candidates, filepath.Dir(cwd))
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if isRuntimeRoot(candidate) {
			return candidate
		}
	}

	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		return cwd
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

func ResolveRuntimePath(root, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func isRuntimeRoot(dir string) bool {
	for _, name := range []string{"config", "data", "qdrant", "surrealdb"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}
