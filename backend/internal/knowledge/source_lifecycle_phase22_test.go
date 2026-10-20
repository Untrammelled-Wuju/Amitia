package knowledge

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestPhase22SourceDeletionCancelsJobsInsideTombstoneTransaction(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, RevisionStatusReady)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-B", 1, RevisionStatusReady)
	now := time.Now().UTC()
	jobs := []IngestJob{
		{ID: "queued-A", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, Kind: JobKindIngest, Status: JobStatusQueued, CreatedAt: now, UpdatedAt: now},
		{ID: "running-A", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, Kind: JobKindReindex, Status: JobStatusRunning, CreatedAt: now, UpdatedAt: now},
		{ID: "finished-A", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, Kind: JobKindIngest, Status: JobStatusCompleted, CreatedAt: now, UpdatedAt: now},
		{ID: "queued-B", TenantID: "tenant-A", SourceID: "source-B", SourceRevision: 1, Kind: JobKindIngest, Status: JobStatusQueued, CreatedAt: now, UpdatedAt: now},
	}
	if err := repo.DB().Create(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDeleteSource("tenant-A", "source-A"); err != nil {
		t.Fatal(err)
	}
	for id, expected := range map[string]string{"queued-A": JobStatusCancelled, "running-A": JobStatusCancelled, "finished-A": JobStatusCompleted, "queued-B": JobStatusQueued} {
		job, err := repo.GetJob("tenant-A", id)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status != expected {
			t.Fatalf("job %s status = %s, want %s", id, job.Status, expected)
		}
		if expected == JobStatusCancelled && (job.FinishedAt == nil || job.Stage != "cancelled") {
			t.Fatalf("deleted source job %s has incomplete cancellation: %+v", id, job)
		}
	}
	if _, claimed, err := repo.ClaimJob("queued-A", now.Add(time.Second), now.Add(-time.Minute)); err != nil || claimed {
		t.Fatalf("deleted source job reclaimed: claimed=%v err=%v", claimed, err)
	}
	if _, claimed, err := repo.ClaimJob("queued-B", now.Add(time.Second), now.Add(-time.Minute)); err != nil || !claimed {
		t.Fatalf("live source job rejected: claimed=%v err=%v", claimed, err)
	}
	if _, err := repo.GetSource("tenant-A", "source-B"); err != nil {
		t.Fatalf("unrelated source hidden: %v", err)
	}
}

func TestPhase22TombstoneFencesDelayedCanonicalWrites(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, RevisionStatusReady)
	now := time.Now().UTC()
	unit := DocumentUnit{ID: "unit-before", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, Content: "original", LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: now}
	if err := repo.InsertUnitBundle(unit, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDeleteSource("tenant-A", "source-A"); err != nil {
		t.Fatal(err)
	}
	lateUnit := unit
	lateUnit.ID = "unit-after"
	for name, operation := range map[string]func() error{
		"stream_insert": func() error { return repo.InsertUnitBundle(lateUnit, nil, nil) },
		"reparse": func() error {
			return repo.ReplaceParsedData("tenant-A", "source-A", 1, []DocumentUnit{lateUnit}, nil, nil)
		},
		"checkpoint_reset": func() error { return repo.ResetRevisionData("tenant-A", "source-A", 1) },
	} {
		if err := operation(); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("deleted source accepted %s: %v", name, err)
		}
	}
	var count int64
	if err := repo.DB().Model(&DocumentUnit{}).Where("tenant_id = ? AND source_id = ?", "tenant-A", "source-A").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("deletion race altered canonical data: count=%d err=%v", count, err)
	}
}

func TestPhase22RevisionAndTenantMustOwnCanonicalWrite(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, RevisionStatusReady)
	now := time.Now().UTC()
	unit := DocumentUnit{ID: "unit", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, Content: "source", LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: now}
	chunk := Chunk{ID: "chunk", TenantID: "tenant-B", SourceID: "source-A", SourceRevision: 1, UnitID: "unit", Content: "source", CreatedAt: now}
	if err := repo.InsertUnitBundle(unit, []Chunk{chunk}, nil); err == nil {
		t.Fatal("cross-tenant chunk accepted")
	}
	outbox := &IndexOutbox{ID: "cross-tenant-outbox", TenantID: "tenant-B", AggregateType: "knowledge_source", AggregateID: "source-A", SourceRevision: 1, Operation: "upsert", Status: "pending", CreatedAt: now, UpdatedAt: now}
	if err := repo.InsertUnitBundle(unit, nil, outbox); err == nil {
		t.Fatal("cross-tenant vector outbox accepted")
	}
	foreign := unit
	foreign.TenantID = "tenant-B"
	if err := repo.InsertUnitBundle(foreign, nil, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign tenant accepted source write: %v", err)
	}
	if err := repo.UpdateSourceRevision("tenant-A", "source-A", 1, map[string]any{"status": RevisionStatusPurged, "purged_at": now}); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertUnitBundle(unit, nil, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("purged revision accepted source write: %v", err)
	}
	if err := repo.ResetRevisionData("tenant-A", "source-A", 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("purged revision accepted reset: %v", err)
	}
	var count int64
	if err := repo.DB().Model(&DocumentUnit{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rejected writes created canonical data: %d %v", count, err)
	}
}

func TestPhase22OrphanQueuedJobCannotBeClaimed(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	job := IngestJob{ID: "orphan-job", TenantID: "tenant-A", SourceID: "missing-source", SourceRevision: 1, Status: JobStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := repo.ClaimJob(job.ID, now, now.Add(-time.Minute)); err != nil || claimed {
		t.Fatalf("orphan source job claimed: %v %v", claimed, err)
	}
}

func TestPhase22TombstoneRetainsPriorPurgedRevisionVectorDeletes(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"status": RevisionStatusPurged, "purged_at": now}); err != nil {
		t.Fatal(err)
	}
	prior := IndexOutbox{ID: "pending-purge-delete", TenantID: src.TenantID, AggregateType: "knowledge_source", AggregateID: src.ID, SourceRevision: 1, Operation: "delete", PayloadJSON: `{"chunkIds":["c1"],"indexVersion":"v1"}`, Status: "pending", CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(&prior).Error; err != nil {
		t.Fatal(err)
	}
	staleUpsert := IndexOutbox{ID: "pending-old-upsert", TenantID: src.TenantID, AggregateType: "knowledge_source", AggregateID: src.ID, SourceRevision: 1, Operation: "upsert", PayloadJSON: `{}`, Status: "pending", CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(&staleUpsert).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDeleteSource(src.TenantID, src.ID); err != nil {
		t.Fatal(err)
	}
	var row IndexOutbox
	if err := repo.DB().First(&row, "id = ?", prior.ID).Error; err != nil || row.Operation != "delete" {
		t.Fatalf("prior purge delete lost: row=%+v err=%v", row, err)
	}
	var staleCount int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", staleUpsert.ID).Count(&staleCount).Error; err != nil || staleCount != 0 {
		t.Fatalf("stale upsert survived tombstone: count=%d err=%v", staleCount, err)
	}
	var currentDeletes int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("tenant_id = ? AND aggregate_id = ? AND source_revision = ? AND operation = 'delete'", src.TenantID, src.ID, 2).Count(&currentDeletes).Error; err != nil || currentDeletes == 0 {
		t.Fatalf("active revision got no vector cleanup: count=%d err=%v", currentDeletes, err)
	}
}
