package main

import (
	"os"
	"path/filepath"
	"strings"
)

func localAgentIdentityPluginDir() string {
	candidates := make([]string, 0, 12)
	appendRoot := func(root string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		candidates = append(candidates, filepath.Join(root, "plugin", "agent-identities"))
	}
	if configured := strings.TrimSpace(os.Getenv("AMITIA_PLUGIN_MARKET_DIR")); configured != "" {
		candidates = append(candidates, configured)
	}
	appendRoot(os.Getenv("AMITIA_WORKSPACE_DIR"))
	if cwd, err := os.Getwd(); err == nil {
		appendRoot(cwd)
		appendRoot(filepath.Dir(cwd))
	}
	if executable, err := os.Executable(); err == nil {
		current := filepath.Dir(executable)
		for depth := 0; depth < 7; depth++ {
			appendRoot(current)
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}
