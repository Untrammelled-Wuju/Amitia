package kernel

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func packageArtifactTestRepository(t *testing.T) (*PackageRepository, *PackageArtifactStore) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "kernel.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := kernelsqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return NewPackageRepository(db), NewPackageArtifactStore(t.TempDir())
}

func putPackageArtifactForTest(t *testing.T, repository *PackageRepository, store *PackageArtifactStore, content []byte, createdAt time.Time) PackageArtifact {
	t.Helper()
	artifact, err := store.PutArchive(context.Background(), bytes.NewReader(content), int64(len(content)+1))
	if err != nil {
		t.Fatalf("put archive: %v", err)
	}
	artifact.ExtensionID = "com.amitia.test"
	artifact.Version = "1.0.0"
	artifact.ManifestHash = "manifest"
	artifact.ContentTreeHash = "tree"
	artifact.ArtifactHash = "artifact"
	artifact.SignatureStatus = "valid"
	artifact.TrustDecision = "trusted"
	artifact.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	artifact.VerifiedAt = artifact.CreatedAt
	artifact.LastVerifiedAt = artifact.CreatedAt
	artifact.VerificationStatus = "valid"
	if err := repository.PutArtifact(context.Background(), artifact); err != nil {
		t.Fatalf("put artifact row: %v", err)
	}
	return artifact
}

func TestPackageArtifactStoreContentAddressedDeduplication(t *testing.T) {
	store := NewPackageArtifactStore(t.TempDir())
	content := []byte("same immutable archive")
	const workers = 32
	results := make(chan PackageArtifact, workers)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			artifact, err := store.PutArchive(context.Background(), bytes.NewReader(content), 1024)
			if err != nil {
				errors <- err
				return
			}
			results <- artifact
		}()
	}
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Fatalf("concurrent put: %v", err)
	}
	var expected PackageArtifact
	for artifact := range results {
		if expected.ArtifactID == "" {
			expected = artifact
		}
		if artifact.ArtifactID != expected.ArtifactID || artifact.ArchivePath != expected.ArchivePath {
			t.Fatalf("dedup mismatch: %#v %#v", expected, artifact)
		}
	}
	if filepath.Base(filepath.Dir(filepath.Dir(expected.ArchivePath))) != expected.ArchiveHash[7:9] {
		t.Fatalf("unexpected content addressed path: %s", expected.ArchivePath)
	}
	var files int
	err := filepath.WalkDir(filepath.Join(store.root, "artifacts", "sha256"), func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files++
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk store: %v", err)
	}
	if files != 1 {
		t.Fatalf("expected one archive, got %d", files)
	}
}

func TestPackageArtifactStoreResolvesLegacyPath(t *testing.T) {
	root := t.TempDir()
	store := NewPackageArtifactStore(root)
	content := []byte("legacy archive")
	artifact, err := store.PutArchive(context.Background(), bytes.NewReader(content), 1024)
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(root, "artifacts", "legacy", "1.0.0", filepath.Base(artifact.ArchivePath))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(artifact.ArchivePath, legacyPath); err != nil {
		t.Fatal(err)
	}
	artifact.ArchivePath = legacyPath
	if err := store.VerifyArchive(artifact); err != nil {
		t.Fatalf("verify legacy path: %v", err)
	}
}

func TestPackageArtifactReferencesConcurrentAndNonNegative(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("reference archive"), time.Now().Add(-time.Hour))
	const workers = 24
	var group sync.WaitGroup
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := repository.AcquireArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "same-owner", time.Time{})
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("acquire reference: %v", err)
		}
	}
	count, err := repository.CountActiveArtifactReferences(context.Background(), artifact.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected idempotent count 1, got %d", count)
	}
	if err := repository.ReleaseArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "same-owner"); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReleaseArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "same-owner"); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.GetArtifact(context.Background(), artifact.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ReferenceCount != 0 {
		t.Fatalf("reference count became invalid: %d", stored.ReferenceCount)
	}
}

func TestPackageArtifactPreviewCancelAndExpiryRelease(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("preview archive"), time.Now().Add(-time.Hour))
	now := time.Now().UTC()
	makePreview := func(id string, expires time.Time) PackagePreviewSession {
		return PackagePreviewSession{SessionID: id, UserID: "1", ScopeType: "global", ArtifactID: artifact.ArtifactID,
			ExtensionID: artifact.ExtensionID, Version: artifact.Version, Status: "ready", ArchiveHash: artifact.ArchiveHash,
			ManifestHash: artifact.ManifestHash, ContentTreeHash: artifact.ContentTreeHash, RiskFlagsJSON: "[]",
			RequiredConfirmationsJSON: "[]", DependencyResultJSON: "[]", PreviewResultJSON: "{}",
			VerificationReportJSON: "{}", PolicyVersion: "v1", VerifiedAt: now.Format(time.RFC3339Nano),
			ExpiresAt: expires.Format(time.RFC3339Nano), CreatedAt: now.Format(time.RFC3339Nano)}
	}
	if err := repository.PutPreview(context.Background(), makePreview("cancelled-preview", now.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := repository.CancelPreview(context.Background(), "cancelled-preview", "1", "global", ""); err != nil {
		t.Fatal(err)
	}
	if err := repository.PutPreview(context.Background(), makePreview("expired-preview", now.Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}
	expired, err := repository.ExpirePackagePreviews(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if expired != 1 {
		t.Fatalf("expected one expired preview, got %d", expired)
	}
	count, err := repository.CountActiveArtifactReferences(context.Background(), artifact.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("preview references leaked: %d", count)
	}
}

func TestPackageArtifactGarbageCollectionAndVerification(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("gc archive"), time.Now().Add(-48*time.Hour))
	lifecycle := NewPackageArtifactLifecycle(repository, store)
	if _, err := repository.AcquireArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "hold", time.Time{}); err != nil {
		t.Fatal(err)
	}
	result, err := lifecycle.CollectGarbage(context.Background(), time.Now(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 0 {
		t.Fatal("referenced artifact was deleted")
	}
	if err := repository.ReleaseArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "hold"); err != nil {
		t.Fatal(err)
	}
	result, err = lifecycle.CollectGarbage(context.Background(), time.Now().Add(time.Second), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != artifact.ArtifactID {
		t.Fatalf("artifact not collected: %#v", result)
	}
	if _, err := os.Stat(artifact.ArchivePath); !os.IsNotExist(err) {
		t.Fatalf("archive still exists: %v", err)
	}

	verified := putPackageArtifactForTest(t, repository, store, []byte("verification archive"), time.Now().Add(-48*time.Hour))
	if err := os.WriteFile(verified.ArchivePath, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	verification, err := lifecycle.VerifyDueArtifacts(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if verification.Corrupted[verified.ArtifactID] == "" {
		t.Fatal("tampered artifact was not reported")
	}
	stored, err := repository.GetArtifact(context.Background(), verified.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.VerificationStatus != "corrupted" || stored.LastVerifiedAt == "" {
		t.Fatalf("verification state not persisted: %#v", stored)
	}
}

func TestPackageArtifactGarbageCollectionFailureIsObservable(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("gc failure archive"), time.Now().Add(-48*time.Hour))
	externalPath := filepath.Join(t.TempDir(), filepath.Base(artifact.ArchivePath))
	if err := os.Rename(artifact.ArchivePath, externalPath); err != nil {
		t.Fatal(err)
	}
	artifact.ArchivePath = externalPath
	if err := repository.PutArtifact(context.Background(), artifact); err != nil {
		t.Fatal(err)
	}
	lifecycle := NewPackageArtifactLifecycle(repository, store)
	result, err := lifecycle.CollectGarbage(context.Background(), time.Now(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed[artifact.ArtifactID] == "" {
		t.Fatal("gc failure was not reported")
	}
	stored, err := repository.GetArtifact(context.Background(), artifact.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RetentionState != "gc_failed" || stored.GCError == "" || stored.GCAttemptedAt == "" {
		t.Fatalf("gc failure state not persisted: %#v", stored)
	}
	if _, err := os.Stat(externalPath); err != nil {
		t.Fatalf("external file unexpectedly changed: %v", err)
	}
}

func TestPackageArtifactMigrationIsRepeatable(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if err := kernelsqlite.Migrate(context.Background(), db); err != nil {
			t.Fatalf("migration run %d: %v", i+1, err)
		}
	}
	for _, column := range []string{"reference_count", "retention_state", "retention_until", "last_verified_at", "verification_status", "gc_error", "gc_attempted_at", "deleted_at"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('extension_package_artifacts') WHERE name=?`, column).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing migrated column %s", column)
		}
	}
	var table string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='extension_package_artifact_references'`).Scan(&table); err != nil {
		t.Fatal(err)
	}
}
