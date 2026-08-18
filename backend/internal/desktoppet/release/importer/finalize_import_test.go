package importer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/release"
	releaserepo "github.com/u-ai/backend/internal/desktoppet/release/repository"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newFinalizeTestImporter(t *testing.T) (*PackageImporter, *gorm.DB, release.ReleaseRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "finalize.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&release.ReleaseData{},
		&release.ReleaseBuildOperation{},
		&release.ImportPackageSnapshot{},
		&security.ImportStaging{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := releaserepo.NewSQLiteRepository(db)
	stagingRepo := security.NewImportStagingRepository(db)
	return NewPackageImporterWithStaging(repo, nil, nil, nil, stagingRepo), db, repo
}

func seedFinalizeRecords(t *testing.T, db *gorm.DB) (*release.ReleaseData, *release.ReleaseBuildOperation, *release.ImportPackageSnapshot, *security.ImportStaging, *ImportPackageRequest) {
	t.Helper()
	releaseRecord := &release.ReleaseData{ID: "release-1", PetID: "pet-1", OwnerUserID: "user-1", Lifecycle: string(release.ReleaseLifecycleBuilding), IntegrityStatus: string(release.ReleaseIntegrityUnknown)}
	buildOp := &release.ReleaseBuildOperation{ID: "op-1", UserID: "user-1", PetID: "pet-1", ReleaseID: "release-1", State: release.BuildOpStateBuilding, Stage: release.ImportJournalStageFilesPublished}
	snapshot := &release.ImportPackageSnapshot{ID: "snapshot-1", ImportStagingID: "staging-1", UserID: "user-1", PetID: "pet-1", ReleaseID: "release-1", OperationID: "op-1", Status: release.ImportSnapshotPublished}
	staging := &security.ImportStaging{ID: "staging-1", OwnerUserID: "user-1", Status: security.StagingStatusConsuming, StateRevision: 2}
	for _, record := range []any{releaseRecord, buildOp, snapshot, staging} {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed %T: %v", record, err)
		}
	}
	request := &ImportPackageRequest{UserID: "user-1", ImportStagingID: "staging-1", ExpectedStagingRevision: 2}
	return releaseRecord, buildOp, snapshot, staging, request
}

func TestFinalizeImportCommitsReleaseOperationSnapshotAndStagingTogether(t *testing.T) {
	importer, db, _ := newFinalizeTestImporter(t)
	releaseRecord, buildOp, snapshot, _, req := seedFinalizeRecords(t, db)

	if err := importer.finalizeImport(context.Background(), req, releaseRecord, buildOp, snapshot, nil); err != nil {
		t.Fatalf("finalize import: %v", err)
	}

	var gotRelease release.ReleaseData
	var gotOp release.ReleaseBuildOperation
	var gotSnapshot release.ImportPackageSnapshot
	var gotStaging security.ImportStaging
	if err := db.First(&gotRelease, "id = ?", releaseRecord.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotOp, "id = ?", buildOp.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotSnapshot, "id = ?", snapshot.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotStaging, "id = ?", req.ImportStagingID).Error; err != nil {
		t.Fatal(err)
	}

	if gotRelease.Lifecycle != string(release.ReleaseLifecycleReady) || gotOp.State != release.BuildOpStateCompleted || gotSnapshot.Status != release.ImportSnapshotCompleted || gotStaging.Status != security.StagingStatusConsumed {
		t.Fatalf("finalize not atomic: release=%s op=%s snapshot=%s staging=%s", gotRelease.Lifecycle, gotOp.State, gotSnapshot.Status, gotStaging.Status)
	}
}

func TestFinalizeImportRollsBackWhenSnapshotUpdateFails(t *testing.T) {
	importer, db, _ := newFinalizeTestImporter(t)
	releaseRecord, buildOp, snapshot, _, req := seedFinalizeRecords(t, db)
	if err := db.Exec(`CREATE TRIGGER fail_snapshot_update BEFORE UPDATE ON desktop_pet_import_package_snapshots BEGIN SELECT RAISE(FAIL, 'forced snapshot failure'); END;`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if err := importer.finalizeImport(context.Background(), req, releaseRecord, buildOp, snapshot, nil); err == nil {
		t.Fatal("expected finalize failure")
	}
	assertFinalizeDBRolledBack(t, db)
}

func TestFinalizeImportRollsBackWhenStagingCASFails(t *testing.T) {
	importer, db, _ := newFinalizeTestImporter(t)
	releaseRecord, buildOp, snapshot, _, req := seedFinalizeRecords(t, db)
	req.ExpectedStagingRevision = 999

	if err := importer.finalizeImport(context.Background(), req, releaseRecord, buildOp, snapshot, nil); err == nil {
		t.Fatal("expected staging CAS failure")
	}
	assertFinalizeDBRolledBack(t, db)
}

func assertFinalizeDBRolledBack(t *testing.T, db *gorm.DB) {
	t.Helper()
	var gotRelease release.ReleaseData
	var gotOp release.ReleaseBuildOperation
	var gotSnapshot release.ImportPackageSnapshot
	var gotStaging security.ImportStaging
	if err := db.First(&gotRelease, "id = ?", "release-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotOp, "id = ?", "op-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotSnapshot, "id = ?", "snapshot-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotStaging, "id = ?", "staging-1").Error; err != nil {
		t.Fatal(err)
	}
	if gotRelease.Lifecycle != string(release.ReleaseLifecycleBuilding) || gotOp.State != release.BuildOpStateBuilding || gotSnapshot.Status != release.ImportSnapshotPublished || gotStaging.Status != security.StagingStatusConsuming {
		t.Fatalf("transaction leaked partial finalization: release=%s op=%s snapshot=%s staging=%s", gotRelease.Lifecycle, gotOp.State, gotSnapshot.Status, gotStaging.Status)
	}
}
