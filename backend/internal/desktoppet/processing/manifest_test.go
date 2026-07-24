// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManifestBuildManifestFields(t *testing.T) {
	actions := []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
		BuildManifestAction("wave", "挥手"),
	}
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "idle_normal", actions)

	if m.SchemaVersion != ManifestSchemaVersion {
		t.Errorf("expected schemaVersion %d, got %d", ManifestSchemaVersion, m.SchemaVersion)
	}
	if m.PackageID != "pkg-1" {
		t.Errorf("expected packageId pkg-1, got %s", m.PackageID)
	}
	if m.Name != "测试包" {
		t.Errorf("expected name 测试包, got %s", m.Name)
	}
	if m.CharacterID != "char-1" {
		t.Errorf("expected characterId char-1, got %s", m.CharacterID)
	}
	if m.GenerationTaskID != "task-1" {
		t.Errorf("expected generationTaskId task-1, got %s", m.GenerationTaskID)
	}
	if m.ProcessingVersion != 3 {
		t.Errorf("expected processingVersion 3, got %d", m.ProcessingVersion)
	}
	if m.Canvas.Width != 512 || m.Canvas.Height != 512 {
		t.Errorf("expected canvas 512x512, got %dx%d", m.Canvas.Width, m.Canvas.Height)
	}
	if m.DefaultAction != "idle_normal" {
		t.Errorf("expected defaultAction idle_normal, got %s", m.DefaultAction)
	}
	if m.Preview != "preview.png" {
		t.Errorf("expected preview preview.png, got %s", m.Preview)
	}
	if len(m.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(m.Actions))
	}
	if !m.Capabilities.HasTransparentBackground {
		t.Errorf("expected HasTransparentBackground true")
	}
	if !m.Capabilities.SupportsFrameSequence {
		t.Errorf("expected SupportsFrameSequence true")
	}
	if _, err := time.Parse(time.RFC3339, m.CreatedAt); err != nil {
		t.Errorf("expected createdAt RFC3339, got %s: %v", m.CreatedAt, err)
	}
}

func TestManifestBuildManifestAction(t *testing.T) {
	a := BuildManifestAction("idle_normal", "正常待机")
	if a.Key != "idle_normal" {
		t.Errorf("expected key idle_normal, got %s", a.Key)
	}
	if a.Name != "正常待机" {
		t.Errorf("expected name 正常待机, got %s", a.Name)
	}
	if a.Config != "actions/idle_normal/action.json" {
		t.Errorf("expected config actions/idle_normal/action.json, got %s", a.Config)
	}
}

func TestManifestWriteManifest(t *testing.T) {
	tmp := t.TempDir()
	b := NewManifestBuilder(tmp)
	actions := []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
	}
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "idle_normal", actions)

	relPath, err := b.WriteManifest("task-1", "pkg-1", m)
	if err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}
	expectedRel := "desktop-pets/generation-tasks/task-1/packages/pkg-1/manifest.json"
	if relPath != expectedRel {
		t.Errorf("expected rel path %s, got %s", expectedRel, relPath)
	}
	absPath := filepath.Join(tmp, filepath.FromSlash(relPath))
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("manifest file not found: %v", err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read manifest failed: %v", err)
	}
	var loaded Manifest
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal manifest failed: %v", err)
	}
	if loaded.PackageID != "pkg-1" {
		t.Errorf("expected packageId pkg-1, got %s", loaded.PackageID)
	}
	if loaded.DefaultAction != "idle_normal" {
		t.Errorf("expected defaultAction idle_normal, got %s", loaded.DefaultAction)
	}
}

func TestManifestValidateSchemaVersionError(t *testing.T) {
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "idle_normal", []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
	})
	m.SchemaVersion = 999
	if err := ValidateManifest(m); err == nil {
		t.Errorf("expected error for unsupported schemaVersion")
	}
}

func TestManifestValidateDefaultActionNotFound(t *testing.T) {
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "missing_action", []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
	})
	if err := ValidateManifest(m); err == nil {
		t.Errorf("expected error when defaultAction not in actions")
	}
}

func TestManifestValidateDefaultActionEmpty(t *testing.T) {
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "", []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
	})
	if err := ValidateManifest(m); err == nil {
		t.Errorf("expected error when defaultAction empty")
	}
}

func TestManifestValidatePathTraversal(t *testing.T) {
	cases := []struct {
		name    string
		preview string
		config  string
	}{
		{"preview parent traversal", "../etc/passwd", "actions/idle_normal/action.json"},
		{"preview absolute unix", "/etc/passwd", "actions/idle_normal/action.json"},
		{"preview windows drive", "C:\\Windows\\system32", "actions/idle_normal/action.json"},
		{"preview backslash path", "subdir\\preview.png", "actions/idle_normal/action.json"},
		{"config parent traversal", "preview.png", "../secret.json"},
		{"config absolute unix", "preview.png", "/etc/secret.json"},
		{"config windows drive", "preview.png", "D:\\secret.json"},
		{"config traversal mid", "preview.png", "actions/../../etc/secret.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "idle_normal", []ManifestAction{
				{Key: "idle_normal", Name: "正常待机", Config: c.config},
			})
			m.Preview = c.preview
			if err := ValidateManifest(m); err == nil {
				t.Errorf("expected error for path traversal case %s", c.name)
			}
		})
	}
}

func TestManifestValidateSuccess(t *testing.T) {
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "idle_normal", []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
		BuildManifestAction("wave", "挥手"),
	})
	if err := ValidateManifest(m); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestManifestValidateNil(t *testing.T) {
	if err := ValidateManifest(nil); err == nil {
		t.Errorf("expected error for nil manifest")
	}
}

func TestManifestIsSafeRelativePath(t *testing.T) {
	valid := []string{
		"preview.png",
		"actions/idle_normal/action.json",
		"actions/wave/action.json",
	}
	for _, p := range valid {
		if !isSafeRelativePath(p) {
			t.Errorf("expected %s to be safe", p)
		}
	}
	invalid := []string{
		"",
		"../etc/passwd",
		"/etc/passwd",
		"C:\\Windows\\system32",
		"subdir\\file.png",
		"actions/../etc/secret.json",
		"D:data.json",
	}
	for _, p := range invalid {
		if isSafeRelativePath(p) {
			t.Errorf("expected %s to be unsafe", p)
		}
	}
}

func TestManifestWriteManifestPathFormat(t *testing.T) {
	tmp := t.TempDir()
	b := NewManifestBuilder(tmp)
	m := BuildManifest("pkg-1", "测试包", "char-1", "task-1", 3, 512, 512, "idle_normal", []ManifestAction{
		BuildManifestAction("idle_normal", "正常待机"),
	})
	relPath, err := b.WriteManifest("task-1", "pkg-1", m)
	if err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}
	if strings.Contains(relPath, "\\") {
		t.Errorf("rel path should not contain backslash: %s", relPath)
	}
	if strings.HasPrefix(relPath, "/") {
		t.Errorf("rel path should not be absolute: %s", relPath)
	}
}
