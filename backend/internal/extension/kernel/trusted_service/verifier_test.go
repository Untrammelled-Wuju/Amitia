// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package trusted_service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type fakeManagedNodeChecker struct {
	managed map[string]bool
}

func (f *fakeManagedNodeChecker) IsManagedNode(exePath string) bool {
	return f.managed[exePath]
}

func TestIsNodeAliasDetectsNode(t *testing.T) {
	aliases := []string{"node", "NODE", "node.exe", "npm", "npm.cmd", "npx", "npx.exe"}

	for _, a := range aliases {
		if !isNodeAlias(a) {
			t.Errorf("expected %q to be detected as node alias", a)
		}
	}
}

func TestIsNodeAliasRejectsNonAliases(t *testing.T) {
	nonAliases := []string{
		"/usr/bin/python",
		"myapp.exe",
		"launcher.mjs",
		"",
		"nodeographer",
		"mynode",
	}
	for _, a := range nonAliases {
		if isNodeAlias(a) {
			t.Errorf("expected %q NOT to be detected as node alias", a)
		}
	}
}

func TestIsNodeAliasRejectsEvenAbsoluteNodePaths(t *testing.T) {
	absoluteNodePaths := []string{
		"/usr/bin/node",
		"/usr/bin/node.exe",
		"C:\\Program Files\\nodejs\\node.exe",
		"/opt/homebrew/bin/node",
	}
	for _, p := range absoluteNodePaths {
		if !isNodeAlias(p) {
			t.Errorf("expected %q to be detected as node alias (trusted_service forbids direct node paths)", p)
		}
	}
}

func TestIsManagedNodeCandidate(t *testing.T) {
	if !isManagedNodeCandidate("/usr/bin/node") {
		t.Fatal("expected /usr/bin/node to be managed node candidate")
	}
	if !isManagedNodeCandidate("/usr/bin/node.exe") {
		t.Fatal("expected /usr/bin/node.exe to be managed node candidate")
	}
	if !isManagedNodeCandidate("C:\\Program Files\\node\\node.exe") {
		t.Fatal("expected Windows path to be managed node candidate")
	}
	if isManagedNodeCandidate("/usr/bin/python") {
		t.Fatal("python should not be managed node candidate")
	}
	if isManagedNodeCandidate("/usr/bin/npm") {
		t.Fatal("npm should not be managed node candidate")
	}
}

func TestNewBinaryVerifierDefault(t *testing.T) {
	v := NewBinaryVerifier()
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
	if v.managedNodeChecker == nil {
		t.Fatal("default verifier should have noOpManagedNodeChecker")
	}
}

func TestNewBinaryVerifierWithManagedNode(t *testing.T) {
	checker := &fakeManagedNodeChecker{}
	v := NewBinaryVerifierWithManagedNode(checker)
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
	if v.managedNodeChecker != checker {
		t.Fatal("expected checker to be set")
	}
}

func TestNewBinaryVerifierWithManagedNodeNilFallsBack(t *testing.T) {
	v := NewBinaryVerifierWithManagedNode(nil)
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
	if v.managedNodeChecker == nil {
		t.Fatal("should fall back to noOpManagedNodeChecker")
	}
}

func TestVerifyRejectsNodeAlias(t *testing.T) {
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path: "node",
		Signature: BinarySignature{
			Trusted: true,
			Value:   "fake",
		},
		Sha256: "fake",
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error for node alias, got %v", err)
	}
}

func TestVerifyRejectsNilExecutable(t *testing.T) {
	v := NewBinaryVerifier()
	err := v.Verify(context.Background(), nil, "")
	if err == nil {
		t.Fatal("expected error for nil executable")
	}
}

func TestVerifyRejectsMissingFile(t *testing.T) {
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      filepath.Join(t.TempDir(), "nonexistent"),
		Sha256:    "abc",
		Signature: BinarySignature{Trusted: true, Value: "x"},
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestVerifyRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      dir,
		Sha256:    "abc",
		Signature: BinarySignature{Trusted: true, Value: "x"},
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil {
		t.Fatal("expected error for directory path")
	}
}

func TestVerifyRejectsNonMatchingHash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	exePath := filepath.Join(root, "test-binary")
	if err := os.WriteFile(exePath, []byte("test content"), 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      exePath,
		Sha256:    "wrong-hash",
		Signature: BinarySignature{Trusted: true, Value: "x"},
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil {
		t.Fatal("expected error for hash mismatch")
	}
}

func TestVerifyAcceptsValidBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	exePath := filepath.Join(root, "test-binary")
	content := []byte("valid executable content")
	if err := os.WriteFile(exePath, content, 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      exePath,
		Sha256:    expectedHash,
		Signature: BinarySignature{Trusted: true, Value: "valid-signature"},
	}
	if err := v.Verify(context.Background(), exe, ""); err != nil {
		t.Fatalf("unexpected error for valid binary: %v", err)
	}
}

func TestVerifyRejectsNonManagedNodeWhenCheckerAvailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	nodePath := filepath.Join(root, "node")
	content := []byte("fake node binary")
	if err := os.WriteFile(nodePath, content, 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	checker := &fakeManagedNodeChecker{managed: map[string]bool{
		nodePath: false,
	}}
	v := NewBinaryVerifierWithManagedNode(checker)

	exe := &PlatformExecutable{
		Path:      nodePath,
		Sha256:    expectedHash,
		Signature: BinarySignature{Trusted: true, Value: "sig"},
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil {
		t.Fatal("expected error for non-managed node")
	}
}

func TestVerifyAcceptsManagedNodeWhenCheckerApproves(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	nodePath := filepath.Join(root, "node")
	content := []byte("managed node binary")
	if err := os.WriteFile(nodePath, content, 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	checker := &fakeManagedNodeChecker{managed: map[string]bool{
		nodePath: true,
	}}
	v := NewBinaryVerifierWithManagedNode(checker)

	exe := &PlatformExecutable{
		Path:      nodePath,
		Sha256:    expectedHash,
		Signature: BinarySignature{Trusted: true, Value: "sig"},
	}
	if err := v.Verify(context.Background(), exe, ""); err != nil {
		t.Fatalf("unexpected error for managed node: %v", err)
	}
}

func TestVerifyRejectsMissingSha256(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	exePath := filepath.Join(root, "test-binary")
	if err := os.WriteFile(exePath, []byte("test"), 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      exePath,
		Sha256:    "",
		Signature: BinarySignature{Trusted: true, Value: "sig"},
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil {
		t.Fatal("expected error for missing sha256")
	}
}

func TestVerifyRejectsUntrustedSignature(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	exePath := filepath.Join(root, "test-binary")
	content := []byte("test content")
	if err := os.WriteFile(exePath, content, 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      exePath,
		Sha256:    expectedHash,
		Signature: BinarySignature{Trusted: false, Value: "sig"},
	}
	err := v.Verify(context.Background(), exe, "")
	if err == nil {
		t.Fatal("expected error for untrusted signature")
	}
}

func TestVerifyResolvesRelativePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	root := t.TempDir()
	exePath := filepath.Join(root, "subdir", "test-binary")
	if err := os.MkdirAll(filepath.Dir(exePath), 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	content := []byte("relative test")
	if err := os.WriteFile(exePath, content, 0755); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      "test-binary",
		Sha256:    expectedHash,
		Signature: BinarySignature{Trusted: true, Value: "sig"},
	}
	if err := v.Verify(context.Background(), exe, filepath.Join(root, "subdir")); err != nil {
		t.Fatalf("unexpected error resolving relative path: %v", err)
	}
}

func TestNoOpManagedNodeCheckerAlwaysFalse(t *testing.T) {
	checker := noOpManagedNodeChecker{}
	if checker.IsManagedNode("/any/path") {
		t.Fatal("noOpManagedNodeChecker should always return false")
	}
}

func TestBinaryVerifierAllowMissingFile(t *testing.T) {
	v := NewBinaryVerifier()
	v.allowMissingFile = true
	if !v.allowMissingFile {
		t.Fatal("allowMissingFile should be settable")
	}
}
