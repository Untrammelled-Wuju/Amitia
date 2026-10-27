package process

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPowerShellCandidatesPreferConfiguredRuntime(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell resolution")
	}
	configured := filepath.Join(t.TempDir(), "pwsh.exe")
	t.Setenv(PowerShellPathEnv, configured)
	systemRoot := t.TempDir()
	candidates := PowerShellCandidates(systemRoot)
	if len(candidates) < 2 {
		t.Fatalf("expected configured and system candidates, got %v", candidates)
	}
	if candidates[0] != configured {
		t.Fatalf("configured PowerShell candidate = %q, want %q", candidates[0], configured)
	}
	wantSystem := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if candidates[1] != wantSystem {
		t.Fatalf("system PowerShell candidate = %q, want %q", candidates[1], wantSystem)
	}
}

func TestResolvePowerShellUsesConfiguredRuntime(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell resolution")
	}
	configured := filepath.Join(t.TempDir(), "pwsh.exe")
	if err := os.WriteFile(configured, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PowerShellPathEnv, configured)
	resolved, err := ResolvePowerShell(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if resolved != configured {
		t.Fatalf("resolved PowerShell = %q, want %q", resolved, configured)
	}
}

func TestResolvePowerShellRejectsInvalidConfiguredRuntime(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell resolution")
	}
	t.Setenv(PowerShellPathEnv, filepath.Join(t.TempDir(), "missing.exe"))
	if _, err := ResolvePowerShell(t.TempDir()); err == nil {
		t.Fatal("expected invalid configured PowerShell runtime to fail")
	}
}

func TestPowerShellCandidatesAreUnique(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell resolution")
	}
	systemRoot := t.TempDir()
	systemPowerShell := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	t.Setenv(PowerShellPathEnv, systemPowerShell)
	candidates := PowerShellCandidates(systemRoot)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := filepath.Clean(candidate)
		if seen[key] {
			t.Fatalf("duplicate PowerShell candidate: %s", candidate)
		}
		seen[key] = true
	}
}
