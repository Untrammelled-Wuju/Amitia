package knowledge

import (
	"strings"
	"testing"
	"time"
)

func phase24InsertOutbox(t *testing.T, repo *Repository, row IndexOutbox) {
	t.Helper()
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPhase24PurgeRetainsOutstandingDeletes(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	for _, entry := range []struct{ id, status string }{
		{"pending-legacy-delete", "pending"},
		{"failed-legacy-delete", "failed"},
		{"completed-legacy-delete", "done"},
	} {
		row := phase23Outbox(entry.id, src.TenantID, src.ID, "delete", 1, now.Add(-time.Hour))
		row.PayloadJSON = `{"chunkIds":["rollback-c1"],"indexVersion":"v-legacy"}`
		row.Status = entry.status
		phase24InsertOutbox(t, repo, row)
	}
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"retain_until": now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PurgeSourceRevision(src.TenantID, src.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pending-legacy-delete", "failed-legacy-delete"} {
		var count int64
		if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", id).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("unfinished cleanup %s lost: count=%d err=%v", id, count, err)
		}
	}
	var completed int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", "completed-legacy-delete").Count(&completed).Error; err != nil || completed != 0 {
		t.Fatalf("completed tombstone not pruned: count=%d err=%v", completed, err)
	}
	var total int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("tenant_id = ? AND aggregate_id = ? AND source_revision = ? AND operation = 'delete'", src.TenantID, src.ID, 1).Count(&total).Error; err != nil || total < 3 {
		t.Fatalf("purge did not retain legacy and enqueue fresh deletes: count=%d err=%v", total, err)
	}
}

func TestPhase24ResetPreservesDeleteAndCreatesVectorCleanup(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	old := phase23Outbox("reset-previous-delete", src.TenantID, src.ID, "delete", 2, now.Add(-time.Minute))
	old.PayloadJSON = `{"chunkIds":["earlier-chunk"],"indexVersion":"v-old"}`
	upsert := phase23Outbox("reset-unclaimed-upsert", src.TenantID, src.ID, "upsert", 2, now)
	phase24InsertOutbox(t, repo, old)
	phase24InsertOutbox(t, repo, upsert)
	if err := repo.ResetRevisionData(src.TenantID, src.ID, 2); err != nil {
		t.Fatal(err)
	}
	var previous, removed int64
	repo.DB().Model(&IndexOutbox{}).Where("id = ?", old.ID).Count(&previous)
	repo.DB().Model(&IndexOutbox{}).Where("id = ?", upsert.ID).Count(&removed)
	if previous != 1 || removed != 0 {
		t.Fatalf("reset changed cleanup or retained stale upsert: old=%d stale=%d", previous, removed)
	}
	var rows []IndexOutbox
	if err := repo.DB().Where("tenant_id = ? AND aggregate_id = ? AND source_revision = ? AND operation = 'delete'", src.TenantID, src.ID, 2).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if strings.Contains(row.PayloadJSON, "rollback-c2") {
			found = true
		}
	}
	if !found {
		t.Fatal("reset deleted a canonical chunk without durable vector cleanup")
	}
	chunks, err := repo.CountChunks(src.TenantID, src.ID, 2)
	if err != nil || chunks != 0 {
		t.Fatalf("reset retained canonical chunks: %d %v", chunks, err)
	}
}

func TestPhase24OldDeleteMustFinishBeforeNewUpsert(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	del := phase23Outbox("delete-before-upsert", "tenant-a", "source-a", "delete", 1, now.Add(-time.Minute))
	upsert := phase23Outbox("upsert-after-delete", "tenant-a", "source-a", "upsert", 1, now)
	phase24InsertOutbox(t, repo, upsert)
	phase24InsertOutbox(t, repo, del)
	first, claimed, err := repo.ClaimNextOutboxForTenant("tenant-a", now, time.Minute)
	if err != nil || !claimed || first.ID != del.ID {
		t.Fatalf("cleanup not claimed before reindex: %#v %v %v", first, claimed, err)
	}
	if second, ok, err := repo.ClaimNextOutboxForTenant("tenant-a", now.Add(time.Second), time.Minute); err != nil || ok || second != nil {
		t.Fatalf("upsert bypassed in-flight old cleanup: %#v %v %v", second, ok, err)
	}
	if done, err := repo.FinishOutboxLease("tenant-a", first.ID, first.LeaseToken, map[string]any{"status": "done"}); err != nil || !done {
		t.Fatalf("cannot complete cleanup: %v %v", done, err)
	}
	next, claimed, err := repo.ClaimNextOutboxForTenant("tenant-a", time.Now().UTC(), time.Minute)
	if err != nil || !claimed || next.ID != upsert.ID {
		t.Fatalf("upsert not released after cleanup: %#v %v %v", next, claimed, err)
	}
}

func TestPhase24RevisionResetRejectedWhileVectorLeaseActive(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	now := time.Now().UTC()
	row := phase23Outbox("active-vector-write", "tenant-a", "source-a", "upsert", 1, now)
	leaseUntil := now.Add(time.Minute)
	row.LeaseUntil = &leaseUntil
	row.LeaseToken = "owner-a"
	phase24InsertOutbox(t, repo, row)
	if err := repo.ResetRevisionData("tenant-a", "source-a", 1); err == nil {
		t.Fatal("reset destroyed a leased outbox")
	}
	if err := repo.ReplaceParsedData("tenant-a", "source-a", 1, nil, nil, nil); err == nil {
		t.Fatal("reparse destroyed a leased outbox")
	}
	job := IngestJob{ID: "retry-vector", TenantID: "tenant-a", SourceID: "source-a", SourceRevision: 1, Kind: JobKindIngest, Status: JobStatusFailed, CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PrepareRetry("tenant-a", job.ID, job.SourceID, 1, ""); err == nil {
		t.Fatal("retry destroyed a leased outbox")
	}
	var stored IndexOutbox
	if err := repo.DB().First(&stored, "id = ?", row.ID).Error; err != nil || stored.LeaseToken != row.LeaseToken {
		t.Fatalf("leased outbox changed: %#v %v", stored, err)
	}
}

func TestPhase24ReplaceParsedDataEnqueuesLegacyVersionCleanup(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	now := time.Now().UTC()
	oldUnit := DocumentUnit{ID: "old-u", TenantID: "tenant-a", SourceID: "source-a", SourceRevision: 1, Content: "old", LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: now}
	oldChunk := Chunk{ID: "old-c", TenantID: "tenant-a", SourceID: "source-a", SourceRevision: 1, UnitID: oldUnit.ID, ChunkIndex: 0, Content: "old", IndexVersion: "v-legacy", CreatedAt: now}
	if err := repo.InsertUnitBundle(oldUnit, []Chunk{oldChunk}, nil); err != nil {
		t.Fatal(err)
	}
	newUnit := oldUnit
	newUnit.ID = "new-u"
	newChunk := oldChunk
	newChunk.ID = "new-c"
	newChunk.UnitID = newUnit.ID
	newChunk.IndexVersion = "v-next"
	row := phase23Outbox("replacement-upsert", "tenant-a", "source-a", "upsert", 1, now.Add(time.Second))
	if err := repo.ReplaceParsedData("tenant-a", "source-a", 1, []DocumentUnit{newUnit}, []Chunk{newChunk}, []IndexOutbox{row}); err != nil {
		t.Fatal(err)
	}
	var rows []IndexOutbox
	if err := repo.DB().Where("tenant_id = ? AND aggregate_id = ? AND source_revision = ? AND operation = 'delete'", "tenant-a", "source-a", 1).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	legacy := false
	for _, item := range rows {
		if strings.Contains(item.PayloadJSON, `"indexVersion":"v-legacy"`) && strings.Contains(item.PayloadJSON, "old-c") {
			legacy = true
		}
	}
	if !legacy {
		t.Fatal("old index version was lost during reparse")
	}
	if _, claimed, err := repo.ClaimNextOutboxForTenant("tenant-a", time.Now().UTC().Add(3*time.Second), time.Minute); err != nil || !claimed {
		t.Fatalf("vector cleanup is not claimable: %v %v", claimed, err)
	}
}

func TestPhase24RetryAllowsFailedRevisionWithoutReleasingLeaseFence(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "failed-source", 1, RevisionStatusFailed)
	now := time.Now().UTC()
	job := IngestJob{ID: "failed-job", TenantID: "tenant-a", SourceID: "failed-source", SourceRevision: 1, Kind: JobKindIngest, Status: JobStatusFailed, CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	changed, err := repo.PrepareRetry("tenant-a", job.ID, job.SourceID, 1, "")
	if err != nil || !changed {
		t.Fatalf("failed revision could not be retried: %v %v", changed, err)
	}
	var stored IngestJob
	if err := repo.DB().First(&stored, "id = ?", job.ID).Error; err != nil || stored.Status != JobStatusQueued {
		t.Fatalf("retry did not queue failed job: %#v %v", stored, err)
	}
	var target SourceRevision
	if err := repo.DB().Where("tenant_id = ? AND source_id = ? AND revision = ?", "tenant-a", "failed-source", 1).First(&target).Error; err != nil || target.Status != RevisionStatusPending {
		t.Fatalf("retry did not reset failed revision: %#v %v", target, err)
	}
}

func TestPhase24ReindexRejectedWhileVectorLeaseActive(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	leased := phase23Outbox("busy-before-reindex", src.TenantID, src.ID, "delete", 1, now)
	leaseUntil := now.Add(time.Minute)
	leased.LeaseUntil = &leaseUntil
	leased.LeaseToken = "owner"
	phase24InsertOutbox(t, repo, leased)
	job := &IngestJob{ID: "reindex-while-deleting", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, Kind: JobKindReindex, Status: JobStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := repo.PrepareRevisionIndexJob(job, 64); err == nil {
		t.Fatal("reindex bypassed in-flight delete")
	}
	var count int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", leased.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("in-flight deletion vanished: count=%d err=%v", count, err)
	}
}
