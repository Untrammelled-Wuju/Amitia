package release_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/release/build"
	releaserepo "github.com/u-ai/backend/internal/desktoppet/release/repository"
	releasestorage "github.com/u-ai/backend/internal/desktoppet/release/storage"
	"gorm.io/gorm"
)

func openRecoveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recovery.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&release.ReleaseBuildOperation{},
		&release.ReleasePublishJournal{},
		&release.ReleaseData{},
		&release.ReleaseFileData{},
		&release.ImportPackageSnapshot{},
	); err != nil {
		t.Fatalf("auto migrate release recovery tables: %v", err)
	}
	return db
}

func TestImportRecoveryRejectsReleaseFileCountMismatch(t *testing.T) {
	db := openRecoveryTestDB(t)
	repo := releaserepo.NewSQLiteRepository(db)
	storage := releasestorage.NewFileSystemStorage(t.TempDir())
	if err := storage.Validate(); err != nil {
		t.Fatalf("validate storage: %v", err)
	}

	const (
		opID      = "op-recovery-file-count"
		petID     = "pet-recovery-file-count"
		releaseID = "rel-recovery-file-count"
	)
	past := time.Now().UTC().Add(-time.Hour).Format("2006-01-02 15:04:05")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := &release.ReleaseBuildOperation{
		ID:             opID,
		UserID:         "user-1",
		PetID:          petID,
		ReleaseID:      releaseID,
		State:          release.BuildOpStatePublishing,
		Stage:          release.ImportJournalStageFilesPublished,
		LeaseOwner:     "dead-owner",
		LeaseExpiresAt: past,
		UpdatedAt:      now,
	}
	if err := repo.CreateBuildOperation(op); err != nil {
		t.Fatalf("create operation: %v", err)
	}
	journal := &release.ReleasePublishJournal{
		ID:            "journal-recovery-file-count",
		OperationID:   opID,
		ReleaseID:     releaseID,
		PetID:         petID,
		OperationKind: string(release.JournalOperationImport),
		Stage:         release.ImportJournalStageFilesPublished,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repo.CreatePublishJournal(journal); err != nil {
		t.Fatalf("create journal: %v", err)
	}
	if err := repo.CreateRelease(&release.ReleaseData{
		ID:                  releaseID,
		PetID:               petID,
		OwnerUserID:         "user-1",
		Lifecycle:           "building",
		ContentRootHash:     "content-root",
		ManifestHash:        "manifest-hash",
		IntegrityStatus:     string(release.ReleaseIntegrityVerified),
		CompatibilityStatus: string(release.ReleaseCompatCompatible),
		FileCount:           2,
		CreatedAt:           now,
		UpdatedAt:           now,
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if err := repo.CreateReleaseFiles([]release.ReleaseFileData{{
		ID:        "file-1",
		ReleaseID: releaseID,
		Path:      "preview.png",
		SHA256:    "unused-because-count-mismatch",
		Bytes:     1,
		CreatedAt: now,
	}}); err != nil {
		t.Fatalf("create release file: %v", err)
	}
	publishedDir, err := storage.PublishedDir(petID, releaseID)
	if err != nil {
		t.Fatalf("published dir: %v", err)
	}
	if err := os.MkdirAll(publishedDir, 0o700); err != nil {
		t.Fatalf("create published dir: %v", err)
	}

	worker := release.NewReleaseRecoveryWorker(
		repo,
		nil,
		build.NewLeaseManager(),
		build.NewPublishJournalManager(repo),
		storage,
		nil,
	)
	if err := worker.RecoverImportOperations(context.Background()); err != nil {
		t.Fatalf("recover imports: %v", err)
	}

	gotOp, err := repo.GetBuildOperation(opID)
	if err != nil {
		t.Fatalf("reload operation: %v", err)
	}
	if gotOp.Stage != release.ImportJournalStageManualReview {
		t.Fatalf("operation stage=%q, want %q", gotOp.Stage, release.ImportJournalStageManualReview)
	}
	if gotOp.ErrorCode != "PUBLISH_INCONSISTENT" {
		t.Fatalf("operation errorCode=%q, want PUBLISH_INCONSISTENT", gotOp.ErrorCode)
	}
	gotJournal, err := repo.GetPublishJournalByOperation(opID)
	if err != nil {
		t.Fatalf("reload journal: %v", err)
	}
	if gotJournal.Stage != release.ImportJournalStageManualReview {
		t.Fatalf("journal stage=%q, want %q", gotJournal.Stage, release.ImportJournalStageManualReview)
	}
}
