// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestLocalCredentialStore_GenerateWhenMissing(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}

	token := store.token
	if len(token) < 32 {
		t.Fatalf("token too short: %d", len(token))
	}

	info, err := os.Stat(tokenFile)
	if err != nil {
		t.Fatalf("token file not found: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %o", info.Mode().Perm())
	}
}

func TestLocalCredentialStore_ReadExisting(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	if err := os.MkdirAll(filepath.Dir(tokenFile), 0o700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	expected := strings.Repeat("a", 48)
	if err := os.WriteFile(tokenFile, []byte(expected), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}
	if store.token != expected {
		t.Fatalf("token mismatch: got %q, want %q", store.token, expected)
	}
}

func TestLocalCredentialStore_ValidateCorrect(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}

	if !store.Validate(store.token) {
		t.Fatal("Validate should return true for correct token")
	}
}

func TestLocalCredentialStore_ValidateIncorrect(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}

	if store.Validate("wrong_token_that_is_still_long_enough_here") {
		t.Fatal("Validate should return false for incorrect token")
	}
}

func TestLocalCredentialStore_VersionStable(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}

	v1 := store.Version()
	v2 := store.Version()
	if v1 != v2 {
		t.Fatalf("Version not stable: %q vs %q", v1, v2)
	}
	if v1 == "" {
		t.Fatal("Version should not be empty")
	}
}

func TestLocalCredentialStore_Rotate(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}
	oldVersion := store.Version()

	newToken := strings.Repeat("b", 48)
	oldV, newV, err := store.Rotate(newToken)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if oldV != oldVersion {
		t.Fatalf("old version mismatch: got %q, want %q", oldV, oldVersion)
	}
	if newV == oldV {
		t.Fatal("new version should differ from old")
	}
	if store.token != newToken {
		t.Fatalf("token not updated after rotate")
	}
	if !store.Validate(newToken) {
		t.Fatal("Validate should succeed with new token")
	}
	if store.Validate(strings.Repeat("a", 48)) {
		t.Fatal("Validate should fail with old token")
	}
}

func TestLocalCredentialStore_RotateFileUpdate(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}

	newToken := strings.Repeat("c", 48)
	_, _, err = store.Rotate(newToken)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	content, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if strings.TrimSpace(string(content)) != newToken {
		t.Fatalf("file content mismatch: got %q, want %q", strings.TrimSpace(string(content)), newToken)
	}
}

func TestLocalCredentialStore_ConcurrentValidateAndRotate(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = store.Validate(store.token)
		}()
		go func(i int) {
			defer wg.Done()
			candidate := strings.Repeat(string(rune('a'+i%26)), 48)
			_, _, _ = store.Rotate(candidate)
		}(i)
	}
	wg.Wait()

	if store.token == "" {
		t.Fatal("token should not be empty after concurrent operations")
	}
}

func TestLocalCredentialStore_InvalidTokenFile(t *testing.T) {
	_, err := NewLocalCredentialStore("")
	if err == nil {
		t.Fatal("expected error for empty token file path")
	}
}

func TestLocalCredentialStore_RegenerateShortToken(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "security", "local-token")

	if err := os.MkdirAll(filepath.Dir(tokenFile), 0o700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(tokenFile, []byte("short"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	store, err := NewLocalCredentialStore(tokenFile)
	if err != nil {
		t.Fatalf("NewLocalCredentialStore failed: %v", err)
	}

	if len(store.token) < 32 {
		t.Fatalf("short token should be regenerated, got length %d", len(store.token))
	}
}
