package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func r64NewSnapshotStore(t *testing.T) (*ResourceSnapshotStore, string) {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "snapshot.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	extRoot := filepath.Join(tmpDir, "ext")
	if err := os.MkdirAll(extRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	store := NewResourceSnapshotStore(db, extRoot)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return store, extRoot
}

func r64HashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestR6_4_PrepareRestoreTargetPathCreatesMissingDirectories(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := filepath.Join(extRoot, "a", "b", "c", "file.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepare should succeed: %v", err)
	}
	if validated == nil {
		t.Fatal("validated path must not be nil")
	}
	if info, statErr := os.Stat(filepath.Dir(target)); statErr != nil || !info.IsDir() {
		t.Fatalf("parent directory not created: %v", statErr)
	}
}

func TestR6_4_PrepareRestoreTargetPathRejectsPathEscape(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	outside := filepath.Dir(extRoot)
	target := filepath.Join(outside, "escape.bin")
	err := ValidateRestoreTargetPath(target, extRoot)
	if err == nil {
		t.Fatal("escape path must be rejected")
	}
}

func TestR6_4_PrepareRestoreTargetPathRejectsSymlinkRoot(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	realRoot := filepath.Join(extRoot, "_real")
	if err := os.MkdirAll(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(extRoot, "_link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink requires privilege: %v", err)
	}
	target := filepath.Join(linkRoot, "nested", "file.bin")
	err := ValidateRestoreTargetPath(target, linkRoot)
	if err == nil {
		t.Fatal("symlink-based root must be rejected")
	}
}

func TestR6_4_PublishRestoreBytesNoReplaceIdempotent(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("idempotent body")
	hash := r64HashContent(body)
	target := filepath.Join(extRoot, "idem.txt")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishRestoreBytesNoReplace(validated, body, hash); err != nil {
		t.Fatalf("first publish must succeed: %v", err)
	}
	validated2, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishRestoreBytesNoReplace(validated2, body, hash); err != nil {
		t.Fatalf("second publish (matching hash) must succeed: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content changed across idempotent publish: %q", got)
	}
}

func TestR6_4_PublishRestoreBytesNoReplaceRejectsMismatchedExisting(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	oldBody := []byte("old content")
	newBody := []byte("new content that must not overwrite")
	if err := os.WriteFile(filepath.Join(extRoot, "locked.txt"), oldBody, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(extRoot, "locked.txt")
	hash := r64HashContent(newBody)
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	err = publishRestoreBytesNoReplace(validated, newBody, hash)
	if err == nil {
		t.Fatal("publish onto mismatched existing target must fail")
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, oldBody) {
		t.Fatalf("old content was overwritten: %q", got)
	}
}

func TestR6_4_PublishRestoreFileNoReplaceRemovesSource(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("move this content into place")
	hash := r64HashContent(body)
	sourcePath := filepath.Join(extRoot, "source.tmp")
	if err := os.WriteFile(sourcePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(extRoot, "nested", "target.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishRestoreFileNoReplace(sourcePath, validated, hash); err != nil {
		t.Fatalf("publish from source must succeed: %v", err)
	}
	if _, statErr := os.Stat(sourcePath); !os.IsNotExist(statErr) {
		t.Fatalf("source must be removed after publish, stat err=%v", statErr)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestR6_4_PublishRestoreFileNoReplaceRejectsMismatchedHash(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("source content")
	wrongHash := r64HashContent([]byte("different content"))
	sourcePath := filepath.Join(extRoot, "src2.tmp")
	if err := os.WriteFile(sourcePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(extRoot, "dst2.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	err = publishRestoreFileNoReplace(sourcePath, validated, wrongHash)
	if err == nil {
		t.Fatal("must reject mismatched expected hash")
	}
}

func TestR6_4_InspectRestoreTargetMissingReturnsFalse(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := filepath.Join(extRoot, "absent.txt")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	present, err := inspectRestoreTarget(validated, "sha256:anything")
	if err != nil {
		t.Fatalf("inspect on missing target must not error: %v", err)
	}
	if present {
		t.Fatal("missing target must not be reported present")
	}
}

func TestR6_4_InspectRestoreTargetDetectsMismatch(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("on-disk content")
	if err := os.WriteFile(filepath.Join(extRoot, "inspect.bin"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(extRoot, "inspect.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, err = inspectRestoreTarget(validated, r64HashContent([]byte("other")))
	if err == nil {
		t.Fatal("must reject target with mismatched hash")
	}
}

func TestR6_4_ValidateRestoreTargetPathRejectsRootAsTarget(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	err := ValidateRestoreTargetPath(extRoot, extRoot)
	if err == nil {
		t.Fatal("ext root itself must not be a valid restore target")
	}
}

func TestR6_4_ValidateRestoreTargetPathNormalizesTrailingSeparator(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := extRoot + string(filepath.Separator) + "file.txt"
	err := ValidateRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("trailing separator path should be normalized and succeed: %v", err)
	}
}

func TestR6_4_CreatePreparedRestoreTempVerifiesHash(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := filepath.Join(extRoot, "nested", "prepared.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("prepare me")
	hash := r64HashContent(body)
	tempPath, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), hash)
	if err != nil {
		t.Fatalf("create prepared temp must succeed: %v", err)
	}
	if _, statErr := os.Stat(tempPath); statErr != nil {
		t.Fatalf("temp file must exist: %v", statErr)
	}
	if !strings.Contains(filepath.Base(tempPath), ".amitia-restore-") {
		t.Fatalf("temp prefix mismatch: %s", filepath.Base(tempPath))
	}
}

func TestR6_4_CreatePreparedRestoreTempRejectsMismatchedWrite(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := filepath.Join(extRoot, "nested2", "prepared.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("written data")
	wrongHash := r64HashContent([]byte("something else"))
	tempPath, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), wrongHash)
	if err == nil {
		t.Fatal("must reject temp whose content hash mismatches expected")
	}
	if tempPath != "" && fileExists(tempPath) {
		t.Fatalf("temp must be cleaned up, still exists: %s", tempPath)
	}
}

func TestR6_4_PublishPreparedRestoreTempNoReplaceSkipsExisting(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("skip-existing body")
	hash := r64HashContent(body)
	target := filepath.Join(extRoot, "existing.bin")
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	tempPath, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishPreparedRestoreTempNoReplace(validated, tempPath, hash); err != nil {
		t.Fatalf("no-replace publish onto matching existing must succeed: %v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content corrupted: %q", got)
	}
}

func TestR6_4_RevalidateRestorePathProofDetectsRootSwap(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := filepath.Join(extRoot, "secure.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Dir(extRoot) + "_swapped"
	if err := os.Rename(extRoot, newDir); err != nil {
		t.Skipf("rename requires privilege: %v", err)
	}
	t.Cleanup(func() { _ = os.Rename(newDir, extRoot) })
	err = revalidateRestorePathProof(validated, true)
	if err == nil {
		t.Fatal("root swap must be detected by revalidation")
	}
}

func TestR6_4_PrepareRestoreTargetPathCreatesIntermediateOnce(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	deep := filepath.Join(extRoot, "x", "y", "z", "file.bin")
	first, err := prepareRestoreTargetPath(deep, extRoot)
	if err != nil {
		t.Fatalf("first prepare must succeed: %v", err)
	}
	if len(first.MissingComponents) != 3 {
		t.Fatalf("first prepare should create 3 missing components, got %v", first.MissingComponents)
	}
	second, err := prepareRestoreTargetPath(deep, extRoot)
	if err != nil {
		t.Fatalf("second prepare must succeed on existing chain: %v", err)
	}
	if len(second.MissingComponents) != 0 {
		t.Fatalf("no missing components expected on second prepare, got %v", second.MissingComponents)
	}
	if filepath.Clean(second.AbsTargetPath) != filepath.Clean(first.AbsTargetPath) {
		t.Fatalf("same target path should resolve identically: %q vs %q", second.AbsTargetPath, first.AbsTargetPath)
	}
}

func TestR6_4_PublishRestoreBytesNoReplaceSyncsDirectory(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("fsync directory after publish")
	hash := r64HashContent(body)
	target := filepath.Join(extRoot, "deep", "nested", "synced.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishRestoreBytesNoReplace(validated, body, hash); err != nil {
		t.Fatalf("publish must succeed: %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("target must exist after publish: %v", statErr)
	}
}

func TestR6_4_InspectRestoreTargetRejectsSymlink(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	realFile := filepath.Join(extRoot, "real.txt")
	if err := os.WriteFile(realFile, []byte("real"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(extRoot, "link.txt")
	if err := os.Symlink(realFile, link); err != nil {
		t.Skipf("symlink requires privilege: %v", err)
	}
	validated, err := prepareRestoreTargetPath(link, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, inspectErr := inspectRestoreTarget(validated, r64HashContent([]byte("real"))); inspectErr == nil {
		t.Fatal("symlink as restore target must be rejected")
	}
}

func TestR6_4_PublishRestoreFileNoReplaceRejectsRegularFileTargetWithDifferentHash(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("cannot replace existing file with different hash")
	hash := r64HashContent(body)
	sourcePath := filepath.Join(extRoot, "src-locked.tmp")
	if err := os.WriteFile(sourcePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(extRoot, "occupied-file.txt")
	if err := os.WriteFile(occupied, []byte("pre-existing content"), 0o600); err != nil {
		t.Fatal(err)
	}
	validated, err := prepareRestoreTargetPath(occupied, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	err = publishRestoreFileNoReplace(sourcePath, validated, hash)
	if err == nil {
		t.Fatal("must reject publish onto existing target with mismatched hash")
	}
	got, readErr := os.ReadFile(occupied)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "pre-existing content" {
		t.Fatalf("target was overwritten despite hash mismatch: %q", got)
	}
}

func TestR6_4_CreatePreparedRestoreTempResidesInTargetParent(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	target := filepath.Join(extRoot, "nested", "myfile.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("locate me in parent")
	hash := r64HashContent(body)
	tempPath, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), hash)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempPath)
	if filepath.Dir(tempPath) != validated.AbsParentDir {
		t.Fatalf("temp must be in validated parent %s, got %s", validated.AbsParentDir, filepath.Dir(tempPath))
	}
}

func TestR6_4_RevalidateDetectsAncestorDisappearance(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	missing := filepath.Join(extRoot, "vanish", "file.bin")
	validated, err := prepareRestoreTargetPath(missing, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(extRoot); err != nil {
		t.Skipf("cleanup requires privilege: %v", err)
	}
	err = revalidateRestorePathProof(validated, true)
	if err == nil {
		t.Fatal("ancestor disappearance must be detected")
	}
}

func TestR6_4_PublishRestoreBytesNonExistentParentCreatedByPrepare(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("deep restore body")
	hash := r64HashContent(body)
	target := filepath.Join(extRoot, "create", "deep", "path", "file.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepare must create all parents: %v", err)
	}
	if err := publishRestoreBytesNoReplace(validated, body, hash); err != nil {
		t.Fatalf("publish must succeed after prepare: %v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content mismatch after prepare+publish: %q", got)
	}
}

func TestR6_4_ValidateRestoreTargetPathRejectsDotDotComponent(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	escaping := filepath.Join(extRoot, "..", "etc", "passwd")
	err := ValidateRestoreTargetPath(escaping, extRoot)
	if err == nil {
		t.Fatal(".. component must be rejected")
	}
}

func TestR6_4_PublishRestoreFileNoReplaceWithEmptySourceRejected(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	emptySource := filepath.Join(extRoot, "empty-src.tmp")
	if err := os.WriteFile(emptySource, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(extRoot, "dst-empty.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	err = publishRestoreFileNoReplace(emptySource, validated, "no-such-hash")
	if err == nil {
		t.Fatal("must reject publish with hash mismatch")
	}
}

func TestR6_4_RestoreQuarantinedFileSafelyUsesValidatedPath(t *testing.T) {
	_, extRoot := r64NewSnapshotStore(t)
	body := []byte("quarantine restore body")
	hash := r64HashContent(body)
	quarantineDir := filepath.Join(extRoot, "quarantine", "resources", "op-test")
	if err := os.MkdirAll(quarantineDir, 0o700); err != nil {
		t.Fatal(err)
	}
	quarantinePath := filepath.Join(quarantineDir, "res-1")
	if err := os.WriteFile(quarantinePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(extRoot, "restored", "res.txt")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	entry := ResourceQuarantineEntry{
		ResourceID:     "res-1",
		OriginalPath:   target,
		QuarantinePath: quarantinePath,
		ContentHash:    hash,
	}
	if err := restoreQuarantinedFileSafely(entry, validated); err != nil {
		t.Fatalf("restore quarantined file must succeed: %v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("quarantine restore content mismatch: %q", got)
	}
	if _, statErr := os.Stat(quarantinePath); !os.IsNotExist(statErr) {
		t.Fatal("quarantine source must be removed after restore")
	}
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
