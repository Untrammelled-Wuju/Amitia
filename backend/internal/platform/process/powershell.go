package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const PowerShellPathEnv = "AMITIA_POWERSHELL_PATH"

func PowerShellCandidates(systemRoot string) []string {
	candidates := make([]string, 0, 4)
	if configured := strings.TrimSpace(os.Getenv(PowerShellPathEnv)); configured != "" {
		candidates = append(candidates, configured)
	}
	root := strings.TrimSpace(systemRoot)
	if root == "" {
		root = strings.TrimSpace(os.Getenv("SystemRoot"))
	}
	if root == "" {
		root = strings.TrimSpace(os.Getenv("WINDIR"))
	}
	if filepath.IsAbs(root) {
		candidates = append(candidates, filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"))
	}
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if resolved, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, resolved)
		}
	}
	seen := make(map[string]struct{}, len(candidates))
	unique := candidates[:0]
	for _, candidate := range candidates {
		cleaned := filepath.Clean(strings.TrimSpace(candidate))
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, cleaned)
	}
	return unique
}

func ResolvePowerShell(systemRoot string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("PowerShell is only available on Windows")
	}
	if configured := strings.TrimSpace(os.Getenv(PowerShellPathEnv)); configured != "" {
		resolved, err := validatePowerShellPath(configured)
		if err != nil {
			return "", fmt.Errorf("%s: %w", PowerShellPathEnv, err)
		}
		return resolved, nil
	}
	for _, candidate := range PowerShellCandidates(systemRoot) {
		resolved, err := validatePowerShellPath(candidate)
		if err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("PowerShell executable unavailable")
}

func validatePowerShellPath(candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !filepath.IsAbs(candidate) {
		return "", fmt.Errorf("path is not absolute: %s", candidate)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", candidate, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", candidate)
	}
	return filepath.Clean(candidate), nil
}
