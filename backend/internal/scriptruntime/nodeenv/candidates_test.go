package nodeenv

import (
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/pkg/platform"
)

func TestRuntimePackageNodeCandidatesWindows(t *testing.T) {
	candidates := runtimePackageNodeCandidates(platform.GuestPlatformWindows)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	expected := []string{
		"amitia://runtime/node/node.exe",
		"amitia://runtime/node/bin/node.exe",
	}
	for i, c := range candidates {
		if c.uri != expected[i] {
			t.Errorf("expected uri %s, got %s", expected[i], c.uri)
		}
		if c.source != SourceRuntimePackage {
			t.Errorf("expected source %s, got %s", SourceRuntimePackage, c.source)
		}
	}
}

func TestRuntimePackageNodeCandidatesLinux(t *testing.T) {
	candidates := runtimePackageNodeCandidates(platform.GuestPlatformLinux)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	expected := []string{
		"amitia://runtime/node/bin/node",
		"amitia://runtime/node/node",
	}
	for i, c := range candidates {
		if c.uri != expected[i] {
			t.Errorf("expected uri %s, got %s", expected[i], c.uri)
		}
	}
}

func TestRuntimePackageNodeCandidatesMacOS(t *testing.T) {
	candidates := runtimePackageNodeCandidates(platform.GuestPlatformMacOS)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].uri != "amitia://runtime/node/bin/node" {
		t.Errorf("unexpected uri: %s", candidates[0].uri)
	}
}

func TestRuntimePackageNodeCandidatesAndroidIsEmpty(t *testing.T) {
	candidates := runtimePackageNodeCandidates(platform.GuestPlatformAndroid)
	if candidates != nil {
		t.Errorf("expected nil candidates for android, got %v", candidates)
	}
}

func TestRuntimePackageNodeCandidatesIOSIsEmpty(t *testing.T) {
	candidates := runtimePackageNodeCandidates(platform.GuestPlatformIOS)
	if candidates != nil {
		t.Errorf("expected nil candidates for ios, got %v", candidates)
	}
}

func TestRuntimePackageNPMCandidatesWindows(t *testing.T) {
	candidates := runtimePackageNPMCandidates(platform.GuestPlatformWindows)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].uri != "amitia://runtime/node/node_modules/npm/bin/npm-cli.js" {
		t.Errorf("unexpected uri: %s", candidates[0].uri)
	}
	if candidates[1].uri != "amitia://runtime/node/lib/node_modules/npm/bin/npm-cli.js" {
		t.Errorf("unexpected uri: %s", candidates[1].uri)
	}
}

func TestRuntimePackageNPMCandidatesLinux(t *testing.T) {
	candidates := runtimePackageNPMCandidates(platform.GuestPlatformLinux)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].uri != "amitia://runtime/node/lib/node_modules/npm/bin/npm-cli.js" {
		t.Errorf("unexpected first uri: %s", candidates[0].uri)
	}
}

func TestRuntimePackageNPXCandidatesOrder(t *testing.T) {
	npx := runtimePackageNPXCandidates(platform.GuestPlatformWindows)
	if len(npx) != 2 {
		t.Fatalf("expected 2 npx candidates, got %d", len(npx))
	}
	if npx[0].uri != "amitia://runtime/node/node_modules/npm/bin/npx-cli.js" {
		t.Errorf("unexpected uri: %s", npx[0].uri)
	}
}

func TestLegacyWindowsNodeCandidates(t *testing.T) {
	guest := platform.GuestPlatformWindows
	runRoot := string(filepath.Separator) + "run"
	wsRoot := string(filepath.Separator) + "ws"
	candidates := legacyNodeCandidates(guest, runRoot, wsRoot)
	if len(candidates) == 0 {
		t.Fatal("expected candidates")
	}
	for _, c := range candidates {
		if c.source != SourceLegacyBundled {
			t.Errorf("expected source legacy, got %s", c.source)
		}
	}
	expected0 := filepath.Join(runRoot, "node", "node.exe")
	expected1 := filepath.Join(runRoot, "backend", "node", "node.exe")
	if candidates[0].path != expected0 {
		t.Errorf("unexpected first path: %s, want %s", candidates[0].path, expected0)
	}
	if candidates[1].path != expected1 {
		t.Errorf("unexpected second path: %s, want %s", candidates[1].path, expected1)
	}
}

func TestLegacyLinuxNodeCandidates(t *testing.T) {
	guest := platform.GuestPlatformLinux
	runRoot := string(filepath.Separator) + "run"
	wsRoot := string(filepath.Separator) + "ws"
	candidates := legacyNodeCandidates(guest, runRoot, wsRoot)
	if len(candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(candidates))
	}
	expected0 := filepath.Join(runRoot, "node", "bin", "node")
	if candidates[0].path != expected0 {
		t.Errorf("unexpected first path: %s, want %s", candidates[0].path, expected0)
	}
}

func TestLegacyCandidatesEmptyRootProducesNothing(t *testing.T) {
	guest := platform.GuestPlatformWindows
	candidates := legacyNodeCandidates(guest, "", "")
	if len(candidates) != 0 {
		t.Errorf("expected no candidates with empty roots, got %d", len(candidates))
	}
}

func TestLegacyCandidatesDedupes(t *testing.T) {
	guest := platform.GuestPlatformLinux
	runRoot := string(filepath.Separator) + "run"
	ws := filepath.Join(runRoot, "backend")
	candidates := legacyNodeCandidates(guest, runRoot, ws)
	seen := make(map[string]bool)
	for _, c := range candidates {
		if seen[c.path] {
			t.Errorf("duplicate path: %s", c.path)
		}
		seen[c.path] = true
	}
}

func TestLegacyCandidatesAndroidEmpty(t *testing.T) {
	candidates := legacyNodeCandidates(platform.GuestPlatformAndroid, "/run", "/ws")
	if len(candidates) != 0 {
		t.Errorf("expected no candidates for android guest, got %d", len(candidates))
	}
}

func TestDeriveDistributionRootWindows(t *testing.T) {
	sep := string(filepath.Separator)
	tests := []struct {
		node     string
		expected string
	}{
		{sep + "dist" + sep + "node.exe", sep + "dist"},
		{sep + "dist" + sep + "bin" + sep + "node.exe", sep + "dist"},
	}
	for _, tc := range tests {
		got := deriveDistributionRoot(tc.node, platform.GuestPlatformWindows)
		if got != tc.expected {
			t.Errorf("deriveDistributionRoot(%q) = %q, want %q", tc.node, got, tc.expected)
		}
	}
}

func TestDeriveDistributionRootLinux(t *testing.T) {
	sep := string(filepath.Separator)
	tests := []struct {
		node     string
		expected string
	}{
		{sep + "dist" + sep + "bin" + sep + "node", sep + "dist"},
		{sep + "dist" + sep + "node", sep + "dist"},
	}
	for _, tc := range tests {
		got := deriveDistributionRoot(tc.node, platform.GuestPlatformLinux)
		if got != tc.expected {
			t.Errorf("deriveDistributionRoot(%q) = %q, want %q", tc.node, got, tc.expected)
		}
	}
}

func TestDeriveDistributionRootEmpty(t *testing.T) {
	got := deriveDistributionRoot("", platform.GuestPlatformLinux)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestNodeFileNameForGuest(t *testing.T) {
	if nodeFileNameForGuest(platform.GuestPlatformWindows) != "node.exe" {
		t.Error("windows should use node.exe")
	}
	if nodeFileNameForGuest(platform.GuestPlatformLinux) != "node" {
		t.Error("linux should use node")
	}
	if nodeFileNameForGuest(platform.GuestPlatformMacOS) != "node" {
		t.Error("macos should use node")
	}
}

func TestIsShellWrapperExtension(t *testing.T) {
	wrappers := []string{"npm.cmd", "npm.bat", "npm.ps1", "npm.sh"}
	for _, w := range wrappers {
		if !isShellWrapperExtension(w) {
			t.Errorf("expected %s to be a shell wrapper", w)
		}
	}
	nonWrappers := []string{"npm-cli.js", "npm-cli.mjs", "npm-cli.cjs"}
	for _, n := range nonWrappers {
		if isShellWrapperExtension(n) {
			t.Errorf("expected %s not to be a shell wrapper", n)
		}
	}
}

func TestIsPackageManagerExtension(t *testing.T) {
	valid := []string{"npm-cli.js", "index.mjs", "cli.cjs"}
	for _, v := range valid {
		if !isPackageManagerExtension(v) {
			t.Errorf("expected %s to be a valid package manager extension", v)
		}
	}
	invalid := []string{"npm.cmd", "npm", "npm.sh", "npm.exe"}
	for _, i := range invalid {
		if isPackageManagerExtension(i) {
			t.Errorf("expected %s not to be a valid package manager extension", i)
		}
	}
}

func TestDedupeCandidates(t *testing.T) {
	input := []candidatePath{
		{source: SourceExplicit, path: "/a"},
		{source: SourceExplicit, path: "/a"},
		{source: SourceExplicit, path: "/b"},
	}
	got := dedupeCandidates(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 dedupe'd, got %d", len(got))
	}
}

func TestDedupeCandidatesEmpty(t *testing.T) {
	got := dedupeCandidates(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestLegacyNpmCandidatesWindows(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep + "root"
	candidates := legacyNpmCandidates(root, platform.GuestPlatformWindows)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	expected0 := filepath.Join(root, "node_modules", "npm", "bin", "npm-cli.js")
	if candidates[0].path != expected0 {
		t.Errorf("unexpected path: %s, want %s", candidates[0].path, expected0)
	}
}

func TestLegacyNpxCandidatesLinux(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep + "root"
	candidates := legacyNpxCandidates(root, platform.GuestPlatformLinux)
	for _, c := range candidates {
		if c.path == "" {
			t.Error("expected non-empty path")
		}
	}
}

func TestLegacyPackageCandidatesEmptyRoot(t *testing.T) {
	if legacyNpmCandidates("", platform.GuestPlatformLinux) != nil {
		t.Error("expected nil with empty root")
	}
	if legacyNpxCandidates("", platform.GuestPlatformWindows) != nil {
		t.Error("expected nil with empty root")
	}
}
