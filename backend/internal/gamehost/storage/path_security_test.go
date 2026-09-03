package storage

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestPathTraversal_PreventEscape(t *testing.T) {
	dm, dataRoot := newTestDirManager(t)
	traversalCases := []domain.PluginID{
		"../../../etc/passwd",
		"../../secret",
		"foo/../bar",
		"foo/..",
		"../outside",
		"a/../../../b",
		"..\\windows",
	}
	for _, id := range traversalCases {
		paths, err := dm.ResolvePluginPaths(id)
		if err != nil {
			continue
		}
		cleaned := filepath.Clean(paths.Root)
		if !strings.HasPrefix(cleaned, dataRoot) {
			t.Fatalf("path traversal escape for id %s: %s not under %s", id, cleaned, dataRoot)
		}
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			continue
		}
		if strings.Contains(string(key), "..") {
			t.Fatalf("storage key must not contain ..: %s", key)
		}
		if strings.ContainsAny(string(key), `/\`) {
			t.Fatalf("storage key must not contain path separators: %s", key)
		}
	}
}

func TestWindowsDrive_PreventEscape(t *testing.T) {
	cases := []domain.PluginID{
		`C:\Windows\System32`,
		`D:\secret`,
		`C:/Windows`,
	}
	for _, id := range cases {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			continue
		}
		if err := ValidateStorageKey(key); err != nil {
			t.Logf("key for %s rejected: %v", id, err)
			continue
		}
		if strings.ContainsAny(string(key), `/\:`) {
			t.Fatalf("key should not contain path separators: %s", key)
		}
	}
}

func TestUNCPath_PreventEscape(t *testing.T) {
	cases := []domain.PluginID{
		`\\server\share\evil`,
		`\\192.168.1.1\share`,
		`//server/share/evil`,
	}
	for _, id := range cases {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			continue
		}
		if strings.ContainsAny(string(key), `/\`) {
			t.Fatalf("UNC path should be sanitized: %s from %s", key, id)
		}
	}
}

func TestUnixAbsolute_PreventEscape(t *testing.T) {
	cases := []domain.PluginID{
		"/etc/passwd",
		"/tmp/foo",
		"/root/.ssh/id_rsa",
	}
	for _, id := range cases {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			continue
		}
		if filepath.IsAbs(string(key)) {
			t.Fatalf("key must not be absolute: %s", key)
		}
	}
}

func TestStorageKey_InputSanitization(t *testing.T) {
	cases := []domain.PluginID{
		"plugin/with/slashes",
		`plugin\with\backslashes`,
		"plugin:with:colons",
		"plugin*with*stars",
		"plugin?with?questions",
		"plugin<with>angles",
		"plugin|with|pipes",
	}
	for _, id := range cases {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			continue
		}
		if err := ValidateStorageKey(key); err != nil {
			t.Fatalf("key for %s should be valid: %v", id, err)
		}
	}
}

func TestPrefixConfusion_Detection(t *testing.T) {
	root := "/data/gamehost"
	if err := EnsureWithinRoot(root, "/data/gamehost-evil"); err == nil {
		t.Fatal("should reject path outside root")
	}
	if err := EnsureWithinRoot(root, "/data/gamehost"); err != nil {
		t.Fatalf("should accept exact root: %v", err)
	}
	if err := EnsureWithinRoot(root, "/data/gamehost/plugins"); err != nil {
		t.Fatalf("should accept child path: %v", err)
	}
	if err := EnsureWithinRoot(root, "/data/gamehost/../gamehost-evil"); err == nil {
		t.Fatal("should reject traversal")
	}
}

func TestLongID_ControlledKeyLength(t *testing.T) {
	longID := domain.PluginID(strings.Repeat("a", 500))
	key, err := StorageKeyForPluginID(longID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(key) > maxStorageKeyLength {
		t.Fatalf("key length %d exceeds maximum %d", len(key), maxStorageKeyLength)
	}
}

func TestSpecialCharacters_IDHandling(t *testing.T) {
	cases := []domain.PluginID{
		"日本語プラグイン",
		"плагин",
		"插件",
		"🔌plugin",
		"plugin\x00null",
		"plugin\x01control",
	}
	for _, id := range cases {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			continue
		}
		if err := ValidateStorageKey(key); err != nil {
			t.Fatalf("key for %q should be valid: %v", id, err)
		}
	}
}

func TestStorageKey_NoPathSeparator(t *testing.T) {
	pluginKey, err := StorageKeyForPluginID("test.plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.ContainsAny(string(pluginKey), `/\`) {
		t.Fatalf("plugin key must not contain path separators: %s", pluginKey)
	}
}

func TestPathTraversal_CannotEscapeGameHostRoot(t *testing.T) {
	dm, dataRoot := newTestDirManager(t)
	pluginID := domain.PluginID("../../etc/passwd")
	paths, err := dm.ResolvePluginPaths(pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	absRoot, _ := filepath.Abs(paths.Root)
	absDataRoot, _ := filepath.Abs(dataRoot)
	rel, err := filepath.Rel(absDataRoot, absRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(rel, "..") {
		t.Fatalf("path traversal escape: %s is outside %s (rel: %s)", absRoot, absDataRoot, rel)
	}
}

var _ = runtime.GOOS
