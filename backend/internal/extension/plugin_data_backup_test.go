package extension

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writePluginDataFixture(t *testing.T, root, relative, content string) string {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestCollectPluginDataFilesWalksDeclaredIncludes(t *testing.T) {
	root := t.TempDir()
	writePluginDataFixture(t, root, "dsh-data/storage/workspace.json", `{"initialized":true}`)
	writePluginDataFixture(t, root, "dsh-data/sessions/session-1.jsonl", "{\"id\":1}\n")
	writePluginDataFixture(t, root, "dsh-packages/cache.tgz", "cache")
	entry := pluginDataScopeEntry{ExtensionID: "com.amitia/dsh", Root: root, Includes: []string{"dsh-data", "missing-data"}}
	files, total, err := collectPluginDataFiles(entry)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	expected := []string{"dsh-data/sessions/session-1.jsonl", "dsh-data/storage/workspace.json"}
	if strings.Join(paths, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected plugin data files: %v", paths)
	}
	expectedTotal := int64(len("{\"id\":1}\n") + len(`{"initialized":true}`))
	if total != expectedTotal {
		t.Fatalf("unexpected plugin data size: %d", total)
	}
}

func TestCollectPluginDataFilesRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic link creation requires privileges on windows")
	}
	root := t.TempDir()
	writePluginDataFixture(t, root, "outside/secret.txt", "secret")
	if err := os.MkdirAll(filepath.Join(root, "dsh-data"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside", "secret.txt"), filepath.Join(root, "dsh-data", "link.txt")); err != nil {
		t.Fatal(err)
	}
	entry := pluginDataScopeEntry{ExtensionID: "com.amitia/dsh", Root: root, Includes: []string{"dsh-data"}}
	if _, _, err := collectPluginDataFiles(entry); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symbolic link rejection, got %v", err)
	}
}

func TestPluginDataRecordRoundTrip(t *testing.T) {
	source := t.TempDir()
	writePluginDataFixture(t, source, "dsh-data/storage/workspace.json", `{"initialized":true}`)
	entry := pluginDataScopeEntry{ExtensionID: "com.amitia/dsh", Root: source, Includes: []string{"dsh-data"}}
	files, _, err := collectPluginDataFiles(entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one plugin data file, got %d", len(files))
	}
	record, err := readPluginDataRecord(entry.ExtensionID, source, files[0])
	if err != nil {
		t.Fatal(err)
	}
	if record.Path != "dsh-data/storage/workspace.json" || record.Size != int64(len(`{"initialized":true}`)) || record.SHA256 == "" {
		t.Fatalf("unexpected record: %+v", record)
	}
	target := t.TempDir()
	if err := writePluginDataRecord(target, []string{"dsh-data"}, record); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(target, "dsh-data", "storage", "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != `{"initialized":true}` {
		t.Fatalf("unexpected restored payload: %s", restored)
	}
	info, err := os.Stat(filepath.Join(target, "dsh-data", "storage", "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected restored mode: %v", info.Mode().Perm())
	}
}

func TestWritePluginDataRecordRejectsUndeclaredPathAndTamperedPayload(t *testing.T) {
	source := t.TempDir()
	writePluginDataFixture(t, source, "dsh-data/storage/workspace.json", `{"initialized":true}`)
	entry := pluginDataScopeEntry{ExtensionID: "com.amitia/dsh", Root: source, Includes: []string{"dsh-data"}}
	files, _, err := collectPluginDataFiles(entry)
	if err != nil {
		t.Fatal(err)
	}
	record, err := readPluginDataRecord(entry.ExtensionID, source, files[0])
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := writePluginDataRecord(target, []string{"other-data"}, record); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("expected undeclared path rejection, got %v", err)
	}
	tampered := record
	tampered.SHA256 = strings.Repeat("0", 64)
	if err := writePluginDataRecord(target, []string{"dsh-data"}, tampered); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("expected hash mismatch rejection, got %v", err)
	}
	truncated := record
	truncated.Size = 1
	if err := writePluginDataRecord(target, []string{"dsh-data"}, truncated); err == nil || !strings.Contains(err.Error(), "declares") {
		t.Fatalf("expected size mismatch rejection, got %v", err)
	}
}

func TestPluginDataPathDeclaredRejectsEscapes(t *testing.T) {
	includes := []string{"dsh-data"}
	for _, path := range []string{"../dsh-data/x", "/dsh-data/x", "dsh-data/../x", "dsh-data\\x", "", " dsh-data/x"} {
		if pluginDataPathDeclared(path, includes) {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
	for _, path := range []string{"dsh-data", "dsh-data/storage/workspace.json"} {
		if !pluginDataPathDeclared(path, includes) {
			t.Fatalf("expected %q to be declared", path)
		}
	}
	if pluginDataPathDeclared("other-data/x", includes) {
		t.Fatal("expected unrelated path to be rejected")
	}
}
