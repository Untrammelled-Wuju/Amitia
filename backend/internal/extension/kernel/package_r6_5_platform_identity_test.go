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

func r65NewSnapshotStoreGeneric(t *testing.T) (*ResourceSnapshotStore, string) {
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

func r65HashContentGeneric(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestR6_5_CaptureRegularFilePlatformIdentityValid(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	targetFile := filepath.Join(extRoot, "regular.txt")
	content := []byte("regular file content")
	if err := os.WriteFile(targetFile, content, 0o600); err != nil {
		t.Fatal(err)
	}

	identity, err := captureRegularFilePlatformIdentity(targetFile)
	if err != nil {
		t.Fatalf("captureRegularFilePlatformIdentity must succeed on regular file: %v", err)
	}

	if identity.IsDirectory {
		t.Fatal("identity must not mark a regular file as directory")
	}

	if err := validatePlatformPathIdentity(targetFile, identity, false); err != nil {
		t.Fatalf("identity must validate against itself: %v", err)
	}
}

func TestR6_5_PrepareRestoreIncludesRootInPlatformComponents(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	target := filepath.Join(extRoot, "file.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepareRestoreTargetPath must succeed: %v", err)
	}

	if len(validated.PlatformComponents) == 0 {
		t.Fatal("platform components must not be empty")
	}

	rootComponent := validated.PlatformComponents[0]
	if rootComponent.Path != validated.AbsoluteRoot {
		t.Fatalf("first platform component path must be root: %q vs %q",
			rootComponent.Path, validated.AbsoluteRoot)
	}

	if !rootComponent.Identity.IsDirectory {
		t.Fatal("root identity must be a directory")
	}
}

func TestR6_5_PrepareRestoreDeepNestedCapturesAllAncestors(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	target := filepath.Join(extRoot, "a", "b", "c", "file.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatalf("prepareRestoreTargetPath must succeed: %v", err)
	}

	expectedDepth := 4
	if len(validated.PlatformComponents) != expectedDepth {
		t.Fatalf("expected %d platform components, got %d",
			expectedDepth, len(validated.PlatformComponents))
	}

	for index, component := range validated.PlatformComponents {
		if strings.TrimSpace(component.Path) == "" {
			t.Fatalf("platform component %d has empty path", index)
		}
		if !component.Identity.IsDirectory {
			t.Fatalf("platform component %d (%s) must be a directory",
				index, component.Path)
		}
	}

	for index := 1; index < len(validated.PlatformComponents); index++ {
		previous := validated.PlatformComponents[index-1].Path
		current := validated.PlatformComponents[index].Path
		if !strings.HasPrefix(current, previous) {
			t.Fatalf("platform component %d (%s) must be nested inside component %d (%s)",
				index, current, index-1, previous)
		}
	}
}

func TestR6_5_RevalidatePlatformRejectsWhenRootIdentityChanges(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	realRoot := filepath.Join(extRoot, "real")
	if err := os.MkdirAll(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(realRoot, "file.bin")
	body := []byte("root identity change test")
	hash := r65HashContentGeneric(body)

	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}

	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	validated.PlatformComponents[0].Identity = platformPathIdentity{
		IsDirectory: true,
	}

	err = revalidateRestorePathProof(validated, true)
	if err == nil {
		t.Fatal("revalidation must detect corrupted root identity")
	}

	_ = hash
}

func TestR6_5_CreatePreparedRestoreTempReturnsIdentity(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	target := filepath.Join(extRoot, "prepared.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte("identity capture test")
	hash := r65HashContentGeneric(body)

	tempPath, identity, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), hash)
	if err != nil {
		t.Fatalf("createPreparedRestoreTemp must succeed: %v", err)
	}
	defer os.Remove(tempPath)

	if identity.IsDirectory {
		t.Fatal("temp file identity must not be directory")
	}

	if err := validatePlatformPathIdentity(tempPath, identity, false); err != nil {
		t.Fatalf("temp must validate against returned identity: %v", err)
	}
}

func TestR6_5_PublishPreparedRestoreVerifiesPlatformIdentity(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	target := filepath.Join(extRoot, "nested", "published.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte("platform identity preservation test")
	hash := r65HashContentGeneric(body)

	tempPath, identity, err := createPreparedRestoreTemp(validated, bytes.NewReader(body), hash)
	if err != nil {
		t.Fatalf("createPreparedRestoreTemp must succeed: %v", err)
	}

	if err := publishPreparedRestoreTempNoReplace(validated, tempPath, identity, hash); err != nil {
		t.Fatalf("publishPreparedRestoreTempNoReplace must succeed: %v", err)
	}

	publishedIdentity, err := captureRegularFilePlatformIdentity(target)
	if err != nil {
		t.Fatalf("capture regular file identity on published target: %v", err)
	}

	if !identity.same(publishedIdentity) {
		t.Fatalf("published file must share temp identity; temp=%+v published=%+v",
			identity, publishedIdentity)
	}
}

func TestR6_5_PublishRestoreFileNoReplaceRemovesSource(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	body := []byte("move file content")
	hash := r65HashContentGeneric(body)
	source := filepath.Join(extRoot, "source.bin")
	if err := os.WriteFile(source, body, 0o600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(extRoot, "nested", "moved.bin")
	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	if err := publishRestoreFileNoReplace(extRoot, source, validated, hash); err != nil {
		t.Fatalf("publishRestoreFileNoReplace must succeed: %v", err)
	}

	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source file must be removed after successful publish")
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestR6_5_PublishRestoreBytesCreatesFileSyncsDirectory(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	body := []byte("deep creation with fsync")
	hash := r65HashContentGeneric(body)
	target := filepath.Join(extRoot, "level1", "level2", "level3", "file.bin")

	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	if err := publishRestoreBytesNoReplace(validated, body, hash); err != nil {
		t.Fatalf("publishRestoreBytesNoReplace must succeed: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content mismatch: %q", got)
	}

	publishedIdentity, err := captureRegularFilePlatformIdentity(target)
	if err != nil {
		t.Fatalf("capture published identity: %v", err)
	}
	if publishedIdentity.IsDirectory {
		t.Fatal("published file must not be a directory")
	}
}

func TestR6_5_InspectRestoreTargetValidatesPlatformIdentity(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	body := []byte("inspect target content")
	hash := r65HashContentGeneric(body)
	target := filepath.Join(extRoot, "inspect.bin")
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}

	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	present, err := inspectRestoreTarget(validated, hash)
	if err != nil {
		t.Fatalf("inspectRestoreTarget must succeed: %v", err)
	}
	if !present {
		t.Fatal("target with matching hash must be reported present")
	}
}

func TestR6_5_RevalidateDetectsCorruptedPlatformChain(t *testing.T) {
	_, extRoot := r65NewSnapshotStoreGeneric(t)

	realParent := filepath.Join(extRoot, "real_nested")
	if err := os.MkdirAll(realParent, 0o700); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(realParent, "victim.bin")
	body := []byte("chain corruption test")

	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}

	validated, err := prepareRestoreTargetPath(target, extRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(validated.PlatformComponents) < 2 {
		t.Fatalf("expected at least 2 platform components, got %d",
			len(validated.PlatformComponents))
	}

	validationErr := revalidateRestorePathProof(validated, true)
	if validationErr != nil {
		t.Fatalf("revalidation must pass on clean state: %v", validationErr)
	}

	for index := range validated.PlatformComponents {
		if validated.PlatformComponents[index].Identity.IsDirectory {
			validated.PlatformComponents[index].Identity = platformPathIdentity{
				IsDirectory: true,
			}
			break
		}
	}

	err = revalidateRestorePathProof(validated, true)
	if err == nil {
		t.Fatal("revalidation must detect corrupted platform component identity")
	}
}
