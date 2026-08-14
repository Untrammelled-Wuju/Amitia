// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateScriptPath(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "test.js"), []byte("console.log('test');"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultScriptPolicyContext()

	tests := []struct {
		name    string
		relPath string
		wantErr bool
	}{
		{"valid script path", "scripts/test.js", false},
		{"path traversal", "../etc/passwd", true},
		{"absolute path", "/etc/passwd", true},
		{"nested traversal", "scripts/../../etc/passwd", true},
		{"empty path", "", true},
		{"dot prefix", "./../secret", true},
		{"nonexistent file", "scripts/nonexistent.js", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateScriptPath(tmpDir, tt.relPath, policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScriptPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyScriptHash(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.js")
	content := []byte("console.log('hello');")
	if err := os.WriteFile(scriptPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	expectedHash := ComputeFileHash(content)
	ctx := context.Background()

	if err := VerifyScriptHash(ctx, scriptPath, expectedHash, defaultScriptFileInspector{}); err != nil {
		t.Errorf("VerifyScriptHash() with correct hash failed: %v", err)
	}

	if err := VerifyScriptHash(ctx, scriptPath, "wronghash", defaultScriptFileInspector{}); err != ErrScriptHashMismatch {
		t.Errorf("VerifyScriptHash() with wrong hash should return ErrScriptHashMismatch, got: %v", err)
	}
}

func TestSanitizeArgs(t *testing.T) {
	tests := []struct {
		name    string
		spec    []SkillScriptArgSpec
		args    map[string]any
		wantErr bool
	}{
		{
			name: "valid string arg",
			spec: []SkillScriptArgSpec{
				{Name: "input", Type: ArgTypeString, Required: true},
			},
			args:    map[string]any{"input": "hello"},
			wantErr: false,
		},
		{
			name: "missing required arg",
			spec: []SkillScriptArgSpec{
				{Name: "input", Type: ArgTypeString, Required: true},
			},
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name: "wrong type arg",
			spec: []SkillScriptArgSpec{
				{Name: "count", Type: ArgTypeInt, Required: true},
			},
			args:    map[string]any{"count": "not-a-number"},
			wantErr: true,
		},
		{
			name: "enum validation pass",
			spec: []SkillScriptArgSpec{
				{Name: "mode", Type: ArgTypeEnum, Enum: []string{"fast", "slow"}},
			},
			args:    map[string]any{"mode": "fast"},
			wantErr: false,
		},
		{
			name: "enum validation fail",
			spec: []SkillScriptArgSpec{
				{Name: "mode", Type: ArgTypeEnum, Enum: []string{"fast", "slow"}},
			},
			args:    map[string]any{"mode": "medium"},
			wantErr: true,
		},
		{
			name: "int bounds check",
			spec: []SkillScriptArgSpec{
				{Name: "count", Type: ArgTypeInt, MinInt: intPtr(1), MaxInt: intPtr(10)},
			},
			args:    map[string]any{"count": 15},
			wantErr: true,
		},
		{
			name: "undeclared arg rejected",
			spec: []SkillScriptArgSpec{
				{Name: "known", Type: ArgTypeString},
			},
			args:    map[string]any{"known": "ok", "unknown": "bad"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizeArgs(tt.args, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateScriptRuntime(t *testing.T) {
	tests := []struct {
		runtime string
		wantErr bool
	}{
		{ScriptRuntimeNode, false},
		{ScriptRuntimeNative, false},
		{ScriptRuntimePython, true},
		{ScriptRuntimeShell, true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			err := ValidateScriptRuntime(tt.runtime)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScriptRuntime(%q) error = %v, wantErr %v", tt.runtime, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{"default timeout", 0, false},
		{"valid timeout", 30 * time.Second, false},
		{"max timeout", MaxScriptTimeout, false},
		{"exceeds max", MaxScriptTimeout + time.Second, true},
		{"negative timeout", -1 * time.Second, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeout(tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDependencies(t *testing.T) {
	tests := []struct {
		name    string
		deps    []SkillScriptDependency
		wantErr bool
	}{
		{"empty deps", nil, false},
		{"allowed dep", []SkillScriptDependency{{Kind: "internal", Name: "utils"}}, false},
		{"npm dep forbidden", []SkillScriptDependency{{Kind: "npm", Name: "lodash"}}, true},
		{"pip dep forbidden", []SkillScriptDependency{{Kind: "pip", Name: "numpy"}}, true},
		{"package dep forbidden", []SkillScriptDependency{{Kind: "package", Name: "tool"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDependencies(tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeFileHash(t *testing.T) {
	data := []byte("test content")
	hash1 := ComputeFileHash(data)
	hash2 := ComputeFileHash(data)

	if hash1 != hash2 {
		t.Errorf("ComputeFileHash should be deterministic: %s != %s", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("SHA-256 hash should be 64 hex chars, got %d", len(hash1))
	}

	differentHash := ComputeFileHash([]byte("different"))
	if hash1 == differentHash {
		t.Errorf("Different content should produce different hashes")
	}
}

func TestDeriveScriptID(t *testing.T) {
	id := DeriveScriptID("my-skill", "scripts/analyze.js")
	if id != "my-skill/analyze.js" {
		t.Errorf("DeriveScriptID() = %s, want my-skill/analyze.js", id)
	}
}

func TestNormalizeScriptPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"scripts/test.js", "scripts/test.js"},
		{"./scripts/test.js", "scripts/test.js"},
		{"../secret", ""},
		{"/absolute/path.js", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeScriptPath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeScriptPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDiscoverFromInventory(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptsDir, "analyze.js")
	content := []byte("console.log('analyze');")
	if err := os.WriteFile(scriptPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	discovery := NewScriptDiscovery(ScriptDiscoveryContext{
		SkillRootResolver: func(ctx context.Context, extensionID string) (string, error) {
			return tmpDir, nil
		},
	})

	ctx := context.Background()
	desc, err := discovery.DiscoverFromInventory(ctx, "test-skill", "scripts/analyze.js")
	if err != nil {
		t.Fatalf("DiscoverFromInventory() error = %v", err)
	}

	if desc.ExtensionID != "test-skill" {
		t.Errorf("ExtensionID = %s, want test-skill", desc.ExtensionID)
	}
	if desc.RelativePath != "scripts/analyze.js" {
		t.Errorf("RelativePath = %s, want scripts/analyze.js", desc.RelativePath)
	}
	if desc.FileHash != ComputeFileHash(content) {
		t.Errorf("FileHash mismatch")
	}
	if desc.Runtime != ScriptRuntimeNode {
		t.Errorf("Runtime = %s, want node", desc.Runtime)
	}
	if desc.EntryName != "analyze.js" {
		t.Errorf("EntryName = %s, want analyze.js", desc.EntryName)
	}
}

func TestSymlinkRejected(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	externalFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(externalFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(scriptsDir, "link.js")
	if err := os.Symlink(externalFile, symlinkPath); err != nil {
		t.Skip("Cannot create symlink on this platform")
	}

	policy := DefaultScriptPolicyContext()
	_, err := ValidateScriptPath(tmpDir, "scripts/link.js", policy)
	if err != ErrScriptSymlinkForbidden {
		t.Errorf("ValidateScriptPath() should reject symlink, got: %v", err)
	}
}

func TestScriptRuntimeExecute(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptsDir, "hello.js")
	content := []byte("console.log('hello from script');")
	if err := os.WriteFile(scriptPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	runtime := NewScriptRuntime(ScriptRuntimeDeps{
		InterpreterResolver: &fakeInterpreterResolver{},
		ProcessSupervisor:   nil,
	})

	ctx := context.Background()
	_, err := runtime.Execute(ctx, ScriptExecutionRequest{
		ExtensionID:  "test-skill",
		SkillName:    "test-skill",
		RelativePath: "hello.js",
		Content:      content,
		ExpectedHash: ComputeFileHash(content),
		Args:         map[string]any{"name": "world"},
	})

	if err == nil {
		t.Skip("Full execution requires a real process supervisor and node environment")
	}
}

func TestHasScriptResource(t *testing.T) {
	resources := []struct {
		Path string
		Kind string
	}{
		{Path: "SKILL.md", Kind: "skill"},
		{Path: "scripts/run.js", Kind: "script"},
		{Path: "references/doc.md", Kind: "reference"},
	}

	if !HasScriptResource(resources) {
		t.Errorf("HasScriptResource() should return true when scripts present")
	}

	noScripts := []struct {
		Path string
		Kind string
	}{
		{Path: "SKILL.md", Kind: "skill"},
		{Path: "references/doc.md", Kind: "reference"},
	}

	if HasScriptResource(noScripts) {
		t.Errorf("HasScriptResource() should return false when no scripts")
	}
}

func TestFileNameWithoutExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"scripts/analyze.js", "analyze"},
		{"run.py", "run"},
		{"no-ext", "no-ext"},
	}

	for _, tt := range tests {
		result := FileNameWithoutExtension(tt.path)
		if result != tt.expected {
			t.Errorf("FileNameWithoutExtension(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestBuildNodeArgs(t *testing.T) {
	desc := SkillScriptDescriptor{
		RelativePath: "scripts/analyze.js",
		EntryName:    "analyze.js",
	}
	interp := ScriptInterpreter{
		Kind:       InterpreterKindNode,
		Executable: "/usr/bin/node",
		ArgsPrefix: []string{"--experimental-modules"},
	}

	args := buildNodeArgs(desc, interp)
	if len(args) != 2 {
		t.Fatalf("buildNodeArgs() returned %d args, want 2", len(args))
	}
	if args[0] != "--experimental-modules" {
		t.Errorf("args[0] = %s, want --experimental-modules", args[0])
	}
	if args[1] != "scripts/analyze.js" {
		t.Errorf("args[1] = %s, want scripts/analyze.js", args[1])
	}
}

func TestValidateWorkingDirPolicy(t *testing.T) {
	tests := []struct {
		policy  string
		wantErr bool
	}{
		{WorkingDirPolicySkillRoot, false},
		{WorkingDirPolicyTemp, false},
		{WorkingDirPolicyExplicit, false},
		{"", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		err := ValidateWorkingDirPolicy(tt.policy)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateWorkingDirPolicy(%q) error = %v, wantErr %v", tt.policy, err, tt.wantErr)
		}
	}
}

func TestValidateOutputMode(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{OutputModeStdout, false},
		{OutputModeFile, false},
		{OutputModeResource, false},
		{"", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		err := ValidateOutputMode(tt.mode)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateOutputMode(%q) error = %v, wantErr %v", tt.mode, err, tt.wantErr)
		}
	}
}

type fakeInterpreterResolver struct{}

func (f *fakeInterpreterResolver) Resolve(ctx context.Context, runtime string, extensionID string) (ScriptInterpreter, error) {
	return ScriptInterpreter{
		Kind:       InterpreterKindNode,
		Executable: "/usr/bin/node",
		Source:     "test",
	}, nil
}

func (f *fakeInterpreterResolver) ResolveFromDescriptor(ctx context.Context, desc SkillScriptDescriptor) (ScriptInterpreter, error) {
	return ScriptInterpreter{
		Kind:       InterpreterKindNode,
		Executable: "/usr/bin/node",
		Source:     "test",
	}, nil
}

func intPtr(v int64) *int64 {
	return &v
}

func TestSanitizeScriptRelPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"scripts/test.js", "scripts/test.js"},
		{"./test.js", "scripts/test.js"},
		{"test.js", "scripts/test.js"},
		{"  spaced.js  ", "scripts/spaced.js"},
	}

	for _, tt := range tests {
		result := SanitizeScriptRelPath(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeScriptRelPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEqualHashes(t *testing.T) {
	tests := []struct {
		a, b    string
		isEqual bool
	}{
		{"abc123", "abc123", true},
		{"ABC123", "abc123", true},
		{"abc123", "def456", false},
		{"abc", "abcd", false},
		{"", "", true},
	}

	for _, tt := range tests {
		result := equalHashes(tt.a, tt.b)
		if result != tt.isEqual {
			t.Errorf("equalHashes(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.isEqual)
		}
	}
}

func TestSanitizeArgsDefault(t *testing.T) {
	spec := []SkillScriptArgSpec{
		{Name: "mode", Type: ArgTypeString, Default: "fast"},
	}
	args := map[string]any{}

	result, err := SanitizeArgs(args, spec)
	if err != nil {
		t.Fatalf("SanitizeArgs() with default error = %v", err)
	}
	if result["mode"] != "fast" {
		t.Errorf("Expected default value 'fast', got %v", result["mode"])
	}
}
