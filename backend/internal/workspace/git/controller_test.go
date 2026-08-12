package git

import (
	"context"
	"strings"
	"testing"
)

func TestFakeGitEngine_DetectNotFound(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_, err := engine.Detect(context.Background(), "/nonexistent")
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got: %v", err)
	}
}

func TestFakeGitEngine_Init(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	err := engine.Init(context.Background(), "/repo1")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	state, err := engine.Detect(context.Background(), "/repo1")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if state.Branch != "main" {
		t.Errorf("expected branch main, got %q", state.Branch)
	}
	if !state.Clean {
		t.Error("expected clean state")
	}
}

func TestFakeGitEngine_Clone(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	result, err := engine.Clone(context.Background(), "/cloned", CloneOptions{
		URL: "https://github.com/example/repo.git",
		Ref: "main",
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if result.Branch != "main" {
		t.Errorf("expected branch main, got %q", result.Branch)
	}
	remotes, err := engine.ListRemotes(context.Background(), "/cloned")
	if err != nil {
		t.Fatalf("ListRemotes failed: %v", err)
	}
	found := false
	for _, r := range remotes {
		if r.Name == "origin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected origin remote")
	}
}

func TestFakeGitEngine_Status(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_ = engine.Init(context.Background(), "/repo2")
	engine.AddFile("/repo2", "test.txt", []byte("content"))
	engine.SetClean("/repo2", false)
	result, err := engine.Status(context.Background(), "/repo2", false, 500)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if result.Clean {
		t.Error("expected non-clean status")
	}
}

func TestFakeGitEngine_AddCommit(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_ = engine.Init(context.Background(), "/repo3")
	engine.AddFile("/repo3", "main.go", []byte("package main"))
	staged, err := engine.Add(context.Background(), "/repo3", AddOptions{
		Paths: []string{"main.go"},
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if len(staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(staged))
	}
	result, err := engine.Commit(context.Background(), "/repo3", CommitOptions{
		Message: "initial commit",
		Author:  &GitIdentity{Name: "Test", Email: "test@example.com"},
	})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if result.Hash == "" {
		t.Error("expected non-empty commit hash")
	}
	if result.FilesChanged != 0 {
		t.Errorf("expected 0 files changed, got %d", result.FilesChanged)
	}
}

func TestFakeGitEngine_Log(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_ = engine.Init(context.Background(), "/repo4")
	engine.AddFile("/repo4", "file.txt", []byte("data"))
	_, _ = engine.Add(context.Background(), "/repo4", AddOptions{Paths: []string{"file.txt"}})
	_, _ = engine.Commit(context.Background(), "/repo4", CommitOptions{Message: "commit1"})
	logResult, err := engine.Log(context.Background(), "/repo4", LogOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if len(logResult.Entries) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logResult.Entries))
	}
}

func TestFakeGitEngine_Branches(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_ = engine.Init(context.Background(), "/repo5")
	branches, err := engine.ListBranches(context.Background(), "/repo5")
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}
	if len(branches.Branches) != 1 {
		t.Errorf("expected 1 branch, got %d", len(branches.Branches))
	}
	if branches.Current != "main" {
		t.Errorf("expected current branch main, got %q", branches.Current)
	}
}

func TestFakeGitEngine_Checkout(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_ = engine.Init(context.Background(), "/repo6")
	result, err := engine.Checkout(context.Background(), "/repo6", CheckoutOptions{
		Branch: "feature",
		Create: true,
	})
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}
	if result.Branch != "feature" {
		t.Errorf("expected branch feature, got %q", result.Branch)
	}
	branches, _ := engine.ListBranches(context.Background(), "/repo6")
	found := false
	for _, b := range branches.Branches {
		if b.Name == "feature" {
			found = true
		}
	}
	if !found {
		t.Error("expected feature branch in list")
	}
}

func TestFakeGitEngine_CheckoutDirtyReject(t *testing.T) {
	engine := NewFakeGitEngine("/tmp/test")
	_ = engine.Init(context.Background(), "/repo7")
	engine.AddFile("/repo7", "dirty.txt", []byte("data"))
	engine.SetClean("/repo7", false)
	_, err := engine.Checkout(context.Background(), "/repo7", CheckoutOptions{
		Branch: "other",
		Create: true,
	})
	if err != ErrGitDirty {
		t.Errorf("expected ErrGitDirty, got: %v", err)
	}
}

func TestValidateRefName(t *testing.T) {
	validRefs := []string{
		"HEAD",
		"main",
		"feature/test",
		"refs/heads/main",
		"refs/tags/v1.0",
		"refs/remotes/origin/main",
		"abc123def456789012345678901234567890abcd",
	}
	for _, ref := range validRefs {
		if err := ValidateRefName(ref); err != nil {
			t.Errorf("expected ref %q to be valid, got: %v", ref, err)
		}
	}
	invalidRefs := []string{
		"",
		"-invalid",
		"with space",
		"with\nnewline",
		"with\ttab",
	}
	for _, ref := range invalidRefs {
		if err := ValidateRefName(ref); err == nil {
			t.Errorf("expected ref %q to be invalid", ref)
		}
	}
}

func TestValidateRemoteURL(t *testing.T) {
	validURLs := []string{
		"https://github.com/example/repo.git",
		"ssh://git@github.com/example/repo.git",
		"/local/path/repo.git",
	}
	for _, url := range validURLs {
		if err := ValidateRemoteURL(url); err != nil {
			t.Errorf("expected URL %q to be valid, got: %v", url, err)
		}
	}
	invalidURLs := []string{
		"",
		"git://example.com/repo.git",
		"ftp://example.com/repo.git",
		"ext::ssh -i key git@host repo",
	}
	for _, url := range invalidURLs {
		if err := ValidateRemoteURL(url); err == nil {
			t.Errorf("expected URL %q to be invalid", url)
		}
	}
}

func TestSanitizeRemoteURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://user:token@github.com/repo.git",
			expected: "https://***@github.com/repo.git",
		},
		{
			input:    "https://github.com/repo.git",
			expected: "https://github.com/repo.git",
		},
		{
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		result := SanitizeRemoteURL(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeRemoteURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsSecretFile(t *testing.T) {
	secretFiles := []string{".env", "id_rsa", "credentials.json", "secret.key"}
	for _, f := range secretFiles {
		if !IsSecretFile(f) {
			t.Errorf("expected %q to be detected as secret", f)
		}
	}
	normalFiles := []string{"main.go", "README.md", "config.yaml"}
	for _, f := range normalFiles {
		if IsSecretFile(f) {
			t.Errorf("expected %q to NOT be detected as secret", f)
		}
	}
}

func TestValidatePaths(t *testing.T) {
	if err := ValidatePaths([]string{"file.go", "dir/sub.txt"}); err != nil {
		t.Errorf("expected valid paths, got: %v", err)
	}
	if err := ValidatePaths([]string{""}); err == nil {
		t.Error("expected invalid for empty path")
	}
	if err := ValidatePaths([]string{"../escape"}); err == nil {
		t.Error("expected invalid for traversal path")
	}
	if err := ValidatePaths([]string{"/absolute"}); err == nil {
		t.Error("expected invalid for absolute path")
	}
}

func TestGitError(t *testing.T) {
	err := NewGitError("GIT_TEST", "test message", ErrGitUnavailable)
	if !strings.Contains(err.Error(), "GIT_TEST") {
		t.Errorf("expected error to contain code, got: %v", err)
	}
	if !strings.Contains(err.Error(), "test message") {
		t.Errorf("expected error to contain message, got: %v", err)
	}
}

func TestParseMountIDFromURI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{
			input:    "amitia://workspace/@abc123/",
			expected: "abc123",
		},
		{
			input:    "amitia://workspace/@mnt123/path/to/file.txt",
			expected: "mnt123",
		},
		{
			input:    "invalid-uri",
			hasError: true,
		},
		{
			input:    "amitia://workspace/@",
			hasError: true,
		},
	}
	for _, tt := range tests {
		result, err := parseMountIDFromURI(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for input %q", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for input %q: %v", tt.input, err)
			continue
		}
		if string(result) != tt.expected {
			t.Errorf("parseMountIDFromURI(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
