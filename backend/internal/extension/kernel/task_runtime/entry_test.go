package task_runtime

import (
	"context"
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

func TestPinnedTaskBundleDetectsDependencyChangesAndUnsafeTrees(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{"src/task.cjs": "module.exports=()=>require('../dependency.cjs');", "dependency.cjs": "module.exports='original';"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	definition := &TaskDefinition{Entry: "src/task.cjs"}
	if err := PinTaskEntry(t.Context(), root, definition); err != nil {
		t.Fatal(err)
	}
	entry, err := ResolveTaskEntry(t.Context(), root, definition)
	if err != nil {
		t.Fatal(err)
	}
	actualRoot, err := TaskBundleRoot(entry, definition)
	actualRootInfo, actualRootErr := os.Stat(actualRoot)
	expectedRootInfo, expectedRootErr := os.Stat(root)
	if err != nil || actualRootErr != nil || expectedRootErr != nil || !os.SameFile(actualRootInfo, expectedRootInfo) {
		t.Fatalf("bundle root %q: %v", actualRoot, err)
	}
	original := definition.BundleHash
	if err := os.WriteFile(filepath.Join(root, "dependency.cjs"), []byte("module.exports='changed';"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := TaskBundleHash(t.Context(), root)
	if err != nil || changed == original {
		t.Fatalf("dependency change not detected: %v", err)
	}
	if err := PinTaskEntry(t.Context(), root, definition); err == nil {
		t.Fatal("changed dependency installation accepted")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := TaskBundleHash(cancelled, root); err == nil {
		t.Fatal("cancelled hashing accepted")
	}
	if _, err := TaskBundleHash(t.Context(), entry); err == nil {
		t.Fatal("file accepted as bundle directory")
	}
	if err := os.Symlink(entry, filepath.Join(root, "linked.cjs")); err == nil {
		if _, err := TaskBundleHash(t.Context(), root); err == nil {
			t.Fatal("symbolic link accepted in bundle")
		}
	}
}
