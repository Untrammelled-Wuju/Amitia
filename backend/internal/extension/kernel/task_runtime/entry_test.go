package task_runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestTaskEntryRequiresInstalledBoundaryAndMatchingSourceHash(t *testing.T) {
	root := t.TempDir()
	content := []byte("module.exports = async () => ({success:true});")
	entry := filepath.Join(root, "task.cjs")
	if err := os.WriteFile(entry, content, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	def := TaskDefinition{Entry: "task.cjs", EntryHash: "sha256:" + hex.EncodeToString(digest[:])}
	resolved, err := ResolveTaskEntry(t.Context(), root, &def)
	if err != nil {
		t.Fatalf("entry=%s error=%v", resolved, err)
	}
	actualInfo, actualErr := os.Stat(resolved)
	expectedInfo, expectedErr := os.Stat(entry)
	if actualErr != nil || expectedErr != nil || !os.SameFile(actualInfo, expectedInfo) {
		t.Fatal("任务入口解析到其他文件")
	}
	for _, value := range []TaskDefinition{{Entry: entry, EntryHash: def.EntryHash}, {Entry: "../task.cjs", EntryHash: def.EntryHash}, {Entry: "task.cjs"}, {Entry: "task.cjs", EntryHash: "sha256:invalid"}} {
		if _, err := ResolveTaskEntry(t.Context(), root, &value); err == nil {
			t.Fatalf("invalid entry accepted: %+v", value)
		}
	}
	if err := os.WriteFile(entry, []byte("changed source"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveTaskEntry(t.Context(), root, &def); err == nil {
		t.Fatal("changed source accepted")
	}
}
