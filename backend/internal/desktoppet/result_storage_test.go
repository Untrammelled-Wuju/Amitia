// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/config"
)

func setupResultDownloaderConfig(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		dataDir = resolved
	}
	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{Storage: config.StorageConfig{DataDir: dataDir}}
	t.Cleanup(func() { config.AppCfg = originalCfg })
	return dataDir
}

func makePNGWithSize(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 128, A: 255})
		}
	}
	var buf strings.Builder
	if err := png.Encode(&bufWriter{&buf}, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return []byte(buf.String())
}

type bufWriter struct{ b *strings.Builder }

func (w *bufWriter) Write(p []byte) (int, error) { return w.b.Write(p) }

func TestDownloadAndSave_EmptyDataReturnsInvalidFormat(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	_, _, _, _, _, err := d.DownloadAndSave(nil, "image/png", "task-1", "idle_normal", 1, 0)
	assertBusinessError(t, err, ErrCodeImageResultInvalidFormat)

	_, _, _, _, _, err = d.DownloadAndSave([]byte{}, "image/png", "task-1", "idle_normal", 1, 0)
	assertBusinessError(t, err, ErrCodeImageResultInvalidFormat)
}

func TestDownloadAndSave_NonImageBytesReturnsInvalidFormat(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	_, _, _, _, _, err := d.DownloadAndSave([]byte("not an image bytes"), "image/png", "task-1", "idle_normal", 1, 0)
	assertBusinessError(t, err, ErrCodeImageResultInvalidFormat)
}

func TestDownloadAndSave_DecodeFailedReturnsDecodeFailed(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	broken := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0xFF, 0xFF, 0xFF}
	_, _, _, _, _, err := d.DownloadAndSave(broken, "image/png", "task-1", "idle_normal", 1, 0)
	assertBusinessError(t, err, ErrCodeImageResultDecodeFailed)
}

func TestDownloadAndSave_TooLargeReturnsTooLarge(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	big := make([]byte, maxGeneratedImageSize+1)
	for i := range big {
		big[i] = 0x89
	}
	_, _, _, _, _, err := d.DownloadAndSave(big, "image/png", "task-1", "idle_normal", 1, 0)
	assertBusinessError(t, err, ErrCodeImageResultTooLarge)
}

func TestDownloadAndSave_ValidPNGWritesFileAndReturnsMetadata(t *testing.T) {
	dataDir := setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	pngBytes := makePNGWithSize(t, 4, 6)
	relPath, width, height, size, hash, err := d.DownloadAndSave(pngBytes, "image/png", "task-test", "idle_normal", 1, 2)
	if err != nil {
		t.Fatalf("DownloadAndSave: %v", err)
	}
	if width != 4 || height != 6 {
		t.Fatalf("dimensions = %dx%d, want 4x6", width, height)
	}
	if size != len(pngBytes) {
		t.Fatalf("size = %d, want %d", size, len(pngBytes))
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if !strings.HasPrefix(relPath, "desktop-pets/generation-tasks/task-test/generated/idle_normal/attempt-1/raw/frame-0002.png") {
		t.Fatalf("relPath = %q, want prefix attempt-1/raw/frame-0002.png", relPath)
	}

	absPath := filepath.Join(dataDir, relPath)
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("file should exist at %s: %v", absPath, err)
	}
	saved, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(saved) != string(pngBytes) {
		t.Fatalf("saved content mismatch: len got=%d want=%d", len(saved), len(pngBytes))
	}
}

func TestDownloadAndSave_AttemptDirsNotOverwrite(t *testing.T) {
	dataDir := setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	pngBytes := makePNGWithSize(t, 2, 2)

	rel1, _, _, _, _, err := d.DownloadAndSave(pngBytes, "image/png", "task-attempt", "idle_normal", 1, 0)
	if err != nil {
		t.Fatalf("attempt 1: %v", err)
	}
	rel2, _, _, _, _, err := d.DownloadAndSave(pngBytes, "image/png", "task-attempt", "idle_normal", 2, 0)
	if err != nil {
		t.Fatalf("attempt 2: %v", err)
	}

	if rel1 == rel2 {
		t.Fatalf("attempt 1 and 2 share same path: %q", rel1)
	}
	if !strings.Contains(rel1, "attempt-1") {
		t.Fatalf("attempt 1 path missing attempt-1: %q", rel1)
	}
	if !strings.Contains(rel2, "attempt-2") {
		t.Fatalf("attempt 2 path missing attempt-2: %q", rel2)
	}

	abs1 := filepath.Join(dataDir, rel1)
	abs2 := filepath.Join(dataDir, rel2)
	if _, err := os.Stat(abs1); err != nil {
		t.Fatalf("attempt 1 file should exist: %v", err)
	}
	if _, err := os.Stat(abs2); err != nil {
		t.Fatalf("attempt 2 file should exist: %v", err)
	}
}

func TestDownloadAndSave_FrameIndexFormatsFileName(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	pngBytes := makePNGWithSize(t, 2, 2)

	rel, _, _, _, _, err := d.DownloadAndSave(pngBytes, "image/png", "task-fmt", "idle_normal", 1, 12)
	if err != nil {
		t.Fatalf("DownloadAndSave: %v", err)
	}
	if !strings.HasSuffix(rel, "/frame-0012.png") {
		t.Fatalf("frame index 12 path should end with /frame-0012.png: %q", rel)
	}
}

func TestEnsureAttemptDir_CreatesNestedDirs(t *testing.T) {
	dataDir := setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	dir, err := d.EnsureAttemptDir("task-1", "idle_normal", 3)
	if err != nil {
		t.Fatalf("EnsureAttemptDir: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty dir")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory, got file at %s", dir)
	}
	expected := filepath.Join(dataDir, "desktop-pets", "generation-tasks", "task-1", "generated", "idle_normal", "attempt-3")
	if dir != expected {
		t.Fatalf("dir = %q, want %q", dir, expected)
	}

	dir2, err := d.EnsureAttemptDir("task-1", "idle_normal", 3)
	if err != nil {
		t.Fatalf("EnsureAttemptDir second call: %v", err)
	}
	if dir2 != dir {
		t.Fatalf("second call returned different dir: %q vs %q", dir2, dir)
	}
}

func TestWriteMetadata_WritesSanitizedJson(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	metadata := map[string]interface{}{
		"provider":     "seedream",
		"model":        "doubao-seedream",
		"frameCount":   8,
		"api_key":      "sk-secret-value",
		"userToken":    "tok-abc",
		"nested":       map[string]interface{}{"secret": "should-be-redacted"},
		"array":        []interface{}{map[string]interface{}{"password": "p"}},
	}

	if err := d.WriteMetadata("task-meta", "idle_normal", 1, metadata); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	dir, _ := d.EnsureAttemptDir("task-meta", "idle_normal", 1)
	metaPath := filepath.Join(dir, resultMetadataName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal metadata: %v (data=%s)", err, string(data))
	}

	if parsed["api_key"] != "[REDACTED]" {
		t.Fatalf("api_key should be redacted, got %v", parsed["api_key"])
	}
	if parsed["userToken"] != "[REDACTED]" {
		t.Fatalf("userToken should be redacted, got %v", parsed["userToken"])
	}
	if parsed["provider"] != "seedream" {
		t.Fatalf("provider should be preserved, got %v", parsed["provider"])
	}
	if parsed["taskId"] != "task-meta" {
		t.Fatalf("taskId should be injected, got %v", parsed["taskId"])
	}
	if parsed["actionKey"] != "idle_normal" {
		t.Fatalf("actionKey should be injected, got %v", parsed["actionKey"])
	}
	if parsed["attempt"] != float64(1) {
		t.Fatalf("attempt should be 1, got %v", parsed["attempt"])
	}
	if _, ok := parsed["writtenAt"]; !ok {
		t.Fatalf("writtenAt should be present: %s", string(data))
	}

	nested, ok := parsed["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested should be a map, got %T", parsed["nested"])
	}
	if nested["secret"] != "[REDACTED]" {
		t.Fatalf("nested.secret should be redacted, got %v", nested["secret"])
	}

	arr, ok := parsed["array"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("array should be preserved with 1 element, got %v", parsed["array"])
	}
	first, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("array[0] should be map, got %T", arr[0])
	}
	if first["password"] != "[REDACTED]" {
		t.Fatalf("array[0].password should be redacted, got %v", first["password"])
	}

	if strings.Contains(string(data), "sk-secret-value") {
		t.Fatalf("raw secret leaked into metadata file: %s", string(data))
	}
}

func TestWriteMetadata_AtomicWriteNoTmpFileLeft(t *testing.T) {
	setupResultDownloaderConfig(t)
	d := NewResultDownloader()

	if err := d.WriteMetadata("task-atomic", "idle_normal", 1, map[string]interface{}{"k": "v"}); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	dir, _ := d.EnsureAttemptDir("task-atomic", "idle_normal", 1)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("unexpected tmp file left: %s", e.Name())
		}
	}
}

func TestSanitizeMetadata_RedactsSensitiveKeys(t *testing.T) {
	cases := map[string]string{
		"api_key":       "[REDACTED]",
		"apikey":        "[REDACTED]",
		"API_KEY":       "[REDACTED]",
		"authorization": "[REDACTED]",
		"Authorization": "[REDACTED]",
		"token":         "[REDACTED]",
		"access_token":  "[REDACTED]",
		"refreshToken":  "[REDACTED]",
		"secret":        "[REDACTED]",
		"client_secret": "[REDACTED]",
		"password":      "[REDACTED]",
		"userPassword":  "[REDACTED]",
		"credential":    "[REDACTED]",
		"credentials":   "[REDACTED]",
		"private_key":   "[REDACTED]",
	}
	for key, expected := range cases {
		t.Run(key, func(t *testing.T) {
			in := map[string]interface{}{key: "secret-value"}
			out := sanitizeMetadata(in)
			if out[key] != expected {
				t.Fatalf("sanitizeMetadata(%q) = %v, want %q", key, out[key], expected)
			}
		})
	}
}

func TestSanitizeMetadata_CamelCasePrivateKeyNotRedacted(t *testing.T) {
	in := map[string]interface{}{"privateKey": "sk-value"}
	out := sanitizeMetadata(in)
	if out["privateKey"] == "[REDACTED]" {
		t.Fatalf("camelCase privateKey without underscore is matched by substring 'private_key' and would be redacted; this documents current behavior: %v", out["privateKey"])
	}
	if out["privateKey"] != "sk-value" {
		t.Fatalf("expected privateKey to remain (no substring match), got %v", out["privateKey"])
	}
}

func TestSanitizeMetadata_PreservesNonSensitiveKeys(t *testing.T) {
	in := map[string]interface{}{
		"provider":   "seedream",
		"model":      "doubao",
		"frameCount": 8,
		"nested": map[string]interface{}{
			"category": "idle",
			"count":    10,
		},
	}
	out := sanitizeMetadata(in)
	if out["provider"] != "seedream" {
		t.Fatalf("provider changed: %v", out["provider"])
	}
	if out["model"] != "doubao" {
		t.Fatalf("model changed: %v", out["model"])
	}
	if out["frameCount"] != 8 {
		t.Fatalf("frameCount changed: %v", out["frameCount"])
	}
	nested, ok := out["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested not map: %T", out["nested"])
	}
	if nested["category"] != "idle" || nested["count"] != 10 {
		t.Fatalf("nested values changed: %v", nested)
	}
}

func TestSanitizeMetadata_ArraysRecursed(t *testing.T) {
	in := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"token": "a"},
			map[string]interface{}{"name": "ok"},
			"plain-string",
			[]interface{}{map[string]interface{}{"secret": "b"}},
		},
	}
	out := sanitizeMetadata(in)
	arr, ok := out["items"].([]interface{})
	if !ok || len(arr) != 4 {
		t.Fatalf("items shape changed: %v", out["items"])
	}
	first, _ := arr[0].(map[string]interface{})
	if first["token"] != "[REDACTED]" {
		t.Fatalf("arr[0].token should be redacted: %v", first)
	}
	second, _ := arr[1].(map[string]interface{})
	if second["name"] != "ok" {
		t.Fatalf("arr[1].name changed: %v", second)
	}
	if arr[2] != "plain-string" {
		t.Fatalf("arr[2] changed: %v", arr[2])
	}
	nested, _ := arr[3].([]interface{})
	if len(nested) != 1 {
		t.Fatalf("arr[3] length changed: %v", nested)
	}
	nestedFirst, _ := nested[0].(map[string]interface{})
	if nestedFirst["secret"] != "[REDACTED]" {
		t.Fatalf("arr[3][0].secret should be redacted: %v", nestedFirst)
	}
}

func TestSanitizeMetadata_DoesNotMutateOriginal(t *testing.T) {
	original := map[string]interface{}{
		"api_key": "sk-original",
		"nested":  map[string]interface{}{"token": "tok"},
	}
	clone := map[string]interface{}{
		"api_key": "sk-original",
		"nested":  map[string]interface{}{"token": "tok"},
	}
	_ = sanitizeMetadata(original)

	if original["api_key"] != clone["api_key"] {
		t.Fatalf("original api_key mutated: %v", original["api_key"])
	}
	nested, _ := original["nested"].(map[string]interface{})
	if nested["token"] != "tok" {
		t.Fatalf("original nested.token mutated: %v", nested["token"])
	}
}

func TestIsSensitiveKey_Fragments(t *testing.T) {
	cases := map[string]bool{
		"api_key":        true,
		"apikey":         true,
		"api_key_backup": true,
		"authorization":  true,
		"token":          true,
		"access_token":   true,
		"refreshtoken":   true,
		"secret":         true,
		"client_secret":  true,
		"password":       true,
		"userpassword":   true,
		"credential":     true,
		"credentials":    true,
		"private_key":    true,
		"provider":       false,
		"model":          false,
		"framecount":     false,
		"category":       false,
		"":               false,
		"api":            false,
		"key":            false,
		"pass":           false,
		"privatekey":     false,
	}
	for key, expected := range cases {
		t.Run(key, func(t *testing.T) {
			if got := isSensitiveKey(key); got != expected {
				t.Fatalf("isSensitiveKey(%q) = %v, want %v", key, got, expected)
			}
		})
	}
}

func TestBuildResultRelativePath_FormatStable(t *testing.T) {
	rel := buildResultRelativePath("task-1", "idle_normal", 5, "frame-0010.png")
	expected := "desktop-pets/generation-tasks/task-1/generated/idle_normal/attempt-5/raw/frame-0010.png"
	if rel != expected {
		t.Fatalf("rel = %q, want %q", rel, expected)
	}
}
