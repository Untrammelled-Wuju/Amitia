//go:build windows

package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func r65HashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func createR65Junction(t *testing.T, junctionPath string, targetPath string) {
	t.Helper()
	if err := os.MkdirAll(targetPath, 0o700); err != nil {
		t.Fatal(err)
	}
	command := fmt.Sprintf(`mklink /J "%s" "%s"`, junctionPath, targetPath)
	output, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junctionPath, targetPath).CombinedOutput()
	if err != nil {
		t.Fatalf("create Junction: command=%s err=%v; output=%s", command, err, string(output))
	}
	t.Cleanup(func() { _ = os.Remove(junctionPath) })
}

func r65NewSnapshotStore(t *testing.T) (*ResourceSnapshotStore, string) {
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

func TestR6_5_CapturePlatformPathIdentityRejectsJunction(t *testing.T) {
	_, extRoot := r65NewSnapshotStore(t)

	realDir := filepath.Join(extRoot, "real_target")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}

	junctionPath := filepath.Join(extRoot, "junction_link")
	createR65Junction(t, junctionPath, realDir)

	_, err := capturePlatformPathIdentity(junctionPath, true)
	if err == nil {
		t.Fatal("capturePlatformPathIdentity must reject junction/reparse point")
	}
}

func TestR6_5_PrepareRestorePlatformChainRejectsJunctionInParent(t *testing.T) {
	_, extRoot := r65NewSnapshotStore(t)

	realParent := filepath.Join(extRoot, "real_parent")
	if err := os.MkdirAll(realParent, 0o700); err != nil {
		t.Fatal(err)
	}

	junctionParent := filepath.Join(extRoot, "junction_parent")
	createR65Junction(t, junctionParent, realParent)

	target := filepath.Join(junctionParent, "file.bin")
	_, err := validateRestoreTargetPathPure(target, extRoot)
	if err == nil {
		t.Fatal("must reject target whose ancestor is a junction")
	}
}

func TestR6_5_PublishRestoreWritesPlainFile(t *testing.T) {
	_, extRoot := r65NewSnapshotStore(t)

	body := []byte("plain Windows restore content")
	hash := r65HashContent(body)
	target := filepath.Join(extRoot, "output.bin")

	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepare must succeed: %v", err)
	}

	if err := publishRestoreBytesNoReplace(validated, body, hash); err != nil {
		t.Fatalf("publish must succeed: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content mismatch: %q", got)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("published file must not be a symlink")
	}
}

func TestR6_5_PublishRestoreFileRejectsSourceJunction(t *testing.T) {
	_, extRoot := r65NewSnapshotStore(t)

	body := []byte("source body")
	hash := r65HashContent(body)
	realSourceDir := filepath.Join(extRoot, "plain-source")
	if err := os.MkdirAll(realSourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	realSource := filepath.Join(realSourceDir, "plain.source")
	if err := os.WriteFile(realSource, body, 0o600); err != nil {
		t.Fatal(err)
	}

	junctionSourceDir := filepath.Join(extRoot, "junction-source")
	createR65Junction(t, junctionSourceDir, realSourceDir)
	junctionSource := filepath.Join(junctionSourceDir, "plain.source")

	target := filepath.Join(extRoot, "plain.target")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepare must succeed: %v", err)
	}

	err = publishRestoreFileNoReplace(junctionSource, validated, hash)
	if err == nil {
		t.Fatal("must reject publish from a junction/reparse point source")
	}
}

func TestR6_5_PreparedTempHasValidPlatformIdentity(t *testing.T) {
	_, extRoot := r65NewSnapshotStore(t)

	target := filepath.Join(extRoot, "nested", "prepared.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte("identity check")
	hash := r65HashContent(body)

	tempPath, identity, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), hash)
	if err != nil {
		t.Fatalf("createPreparedRestoreTemp must succeed: %v", err)
	}
	defer os.Remove(tempPath)

	if identity.VolumeSerialNumber == 0 {
		t.Fatal("platform identity volume serial number must not be zero")
	}

	if identity.IsDirectory {
		t.Fatal("platform identity must mark file as non-directory")
	}
}

func TestR6_5_PublishRejectsAfterParentChainReparseSwap(t *testing.T) {
	_, extRoot := r65NewSnapshotStore(t)

	realParent := filepath.Join(extRoot, "real_chain_parent")
	if err := os.MkdirAll(realParent, 0o700); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(realParent, "victim.bin")
	body := []byte("victim content")
	hash := r65HashContent(body)

	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepare must succeed: %v", err)
	}

	if len(validated.PlatformComponents) < 2 {
		t.Fatalf("expected at least root + parent platform components, got %d", len(validated.PlatformComponents))
	}

	rootComponent := validated.PlatformComponents[0]
	if rootComponent.Path != validated.AbsoluteRoot {
		t.Fatalf("first platform component must be root: %s vs %s", rootComponent.Path, validated.AbsoluteRoot)
	}
	if !rootComponent.Identity.IsDirectory {
		t.Fatal("root platform identity must be a directory")
	}

	parentComponent := validated.PlatformComponents[1]
	if !parentComponent.Identity.IsDirectory {
		t.Fatal("parent component identity must be a directory")
	}

	backup := realParent + ".backup"
	if err := os.Rename(realParent, backup); err != nil {
		t.Fatal(err)
	}
	createR65Junction(t, realParent, t.TempDir())

	err = revalidateRestorePathProof(validated, true)
	if err == nil {
		t.Fatal("revalidation must detect that the validated parent was replaced by a junction")
	}

	_ = hash
	_ = body
}

func TestR6_5_RejectsMountedFolderInParentChain(t *testing.T) {
	mountPoint := os.Getenv("AMITIA_R65_MOUNT_POINT")
	mountTarget := os.Getenv("AMITIA_R65_MOUNT_TARGET")
	if mountPoint == "" || mountTarget == "" {
		t.Skip("R6-5 mount point environment is not configured")
	}
	extRoot := filepath.Dir(mountPoint)
	if _, err := validateRestoreTargetPathPure(mountTarget, extRoot); err == nil {
		t.Fatal("mounted folder in restore parent chain must be rejected")
	}
}

func TestR6_5_RejectsMountPointInsertedAfterPrepare(t *testing.T) {
	mountPoint := os.Getenv("AMITIA_R65_MOUNT_POINT")
	if mountPoint == "" {
		t.Skip("R6-5 mount point environment is not configured")
	}
	extRoot := filepath.Dir(mountPoint)
	target := filepath.Join(extRoot, "prepare-before-mount", "file.bin")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer validated.Close()
	if err := os.Remove(filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(mountPoint, filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(filepath.Dir(target), mountPoint)
	if err := revalidateRestorePathProof(validated, true); err == nil {
		t.Fatal("mount point inserted after prepare must be rejected")
	}
}
