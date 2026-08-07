// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package util

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	RuntimeRootEnv  = "AMITIA_RUNTIME_ROOT"
	ConfigDirEnv    = "AMITIA_CONFIG_DIR"
	DataDirEnv      = "AMITIA_DATA_DIR"
	LogDirEnv       = "AMITIA_LOG_DIR"
	WorkspaceDirEnv = "AMITIA_WORKSPACE_DIR"
	CacheDirEnv     = "AMITIA_CACHE_DIR"
	TempDirEnv      = "AMITIA_TEMP_DIR"
	CONFIG_PATH     = "CONFIG_PATH"
)

type RuntimePaths struct {
	Root         string
	ConfigDir    string
	DataDir      string
	LogDir       string
	WorkspaceDir string
	CacheDir     string
	TempDir      string
}

func RuntimeRoot() string {
	if v := strings.TrimSpace(os.Getenv(RuntimeRootEnv)); v != "" {
		cleaned := filepath.Clean(v)
		if filepath.IsAbs(cleaned) {
			return cleaned
		}
		cwd, err := os.Getwd()
		if err == nil {
			return filepath.Clean(filepath.Join(cwd, cleaned))
		}
		return cleaned
	}

	candidates := make([]string, 0, 20)

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for i := 0; i < 5; i++ {
			candidates = append(candidates, exeDir)
			parent := filepath.Dir(exeDir)
			if parent == exeDir {
				break
			}
			exeDir = parent
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		for i := 0; i < 5; i++ {
			candidates = append(candidates, cwd)
			parent := filepath.Dir(cwd)
			if parent == cwd {
				break
			}
			cwd = parent
		}
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
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

func isRuntimeRoot(dir string) bool {
	if dirExists(filepath.Join(dir, "backend")) && dirExists(filepath.Join(dir, "config")) {
		return true
	}
	if fileExists(filepath.Join(dir, "go.mod")) && dirExists(filepath.Join(dir, "cmd")) && dirExists(filepath.Join(dir, "config")) {
		return true
	}
	if dirExists(filepath.Join(dir, "config")) && dirExists(filepath.Join(dir, "qdrant")) && dirExists(filepath.Join(dir, "surrealdb")) {
		return true
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ResolveRuntimePath(root, p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if root != "" {
		return filepath.Clean(filepath.Join(root, p))
	}
	return filepath.Clean(p)
}

func resolveEnvDir(v, root string) string {
	if filepath.IsAbs(v) {
		return filepath.Clean(v)
	}
	if root != "" {
		return filepath.Clean(filepath.Join(root, v))
	}
	return filepath.Clean(v)
}

func RuntimeConfigDir(runtimeRoot string) string {
	if v := strings.TrimSpace(os.Getenv(ConfigDirEnv)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	if v := strings.TrimSpace(os.Getenv(CONFIG_PATH)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	return filepath.Join(runtimeRoot, "config")
}

func RuntimeDataDir(runtimeRoot, configuredDataDir string) string {
	if v := strings.TrimSpace(os.Getenv(DataDirEnv)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	if configuredDataDir == "" {
		return filepath.Join(runtimeRoot, "data")
	}
	if filepath.IsAbs(configuredDataDir) {
		return filepath.Clean(configuredDataDir)
	}
	return filepath.Join(runtimeRoot, configuredDataDir)
}

func RuntimeLogDir(runtimeRoot string) string {
	if v := strings.TrimSpace(os.Getenv(LogDirEnv)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	return filepath.Join(runtimeRoot, "logs")
}

func RuntimeWorkspaceDir(runtimeRoot string) string {
	if v := strings.TrimSpace(os.Getenv(WorkspaceDirEnv)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	return runtimeRoot
}

func RuntimeCacheDir(runtimeRoot, dataDir string) string {
	if v := strings.TrimSpace(os.Getenv(CacheDirEnv)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	if dataDir != "" {
		return filepath.Join(dataDir, "cache")
	}
	return filepath.Join(runtimeRoot, "data", "cache")
}

func RuntimeTempDir(runtimeRoot, dataDir string) string {
	if v := strings.TrimSpace(os.Getenv(TempDirEnv)); v != "" {
		return resolveEnvDir(v, runtimeRoot)
	}
	if dataDir != "" {
		return filepath.Join(dataDir, "tmp")
	}
	return filepath.Join(runtimeRoot, "data", "tmp")
}

func DetectRuntimePaths(configuredDataDir string) RuntimePaths {
	root := RuntimeRoot()
	configDir := RuntimeConfigDir(root)
	dataDir := RuntimeDataDir(root, configuredDataDir)
	logDir := RuntimeLogDir(root)
	workspaceDir := RuntimeWorkspaceDir(root)
	cacheDir := RuntimeCacheDir(root, dataDir)
	tempDir := RuntimeTempDir(root, dataDir)
	return RuntimePaths{
		Root:         root,
		ConfigDir:    configDir,
		DataDir:      dataDir,
		LogDir:       logDir,
		WorkspaceDir: workspaceDir,
		CacheDir:     cacheDir,
		TempDir:      tempDir,
	}
}
