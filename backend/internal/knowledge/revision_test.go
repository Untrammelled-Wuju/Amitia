package knowledge

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newKnowledgeTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Source{}, &SourceRevision{}, &DocumentUnit{}, &Chunk{}, &IngestJob{}, &IndexOutbox{}, &IndexAlias{}, &BenchmarkRunRecord{}); err != nil {
		t.Fatalf("migrate knowledge schema: %v", err)
	}
	return NewRepository(db)
}

func TestCandidateRevisionIsInvisibleUntilAtomicCutover(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	src := &Source{
		ID: "source-1", TenantID: "tenant-1", LogicalName: "manual", Filename: "manual-v1.txt",
		MimeType: "text/plain", SourceType: "txt", StorageKey: "s/r1", SHA256: "sha-v1", Revision: 1,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: SourceStatusReady, ExpectedUnits: 1, ProcessedUnits: 1, IndexedChunks: 1, CreatedAt: now, UpdatedAt: now,
	}
	job1 := &IngestJob{ID: "job-1", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, Kind: JobKindIngest, Stage: "completed", Status: JobStatusCompleted, CursorJSON: `{"ordinal":1}`, LastCommittedOrdinal: 0, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateSource(src, job1); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"status": RevisionStatusReady, "activated_at": now}); err != nil {
		t.Fatalf("ready r1: %v", err)
	}
	unit1 := DocumentUnit{ID: "u1", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, Ordinal: 0, UnitType: "text", Content: "old active fact", LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: now}
	chunk1 := Chunk{ID: "c1", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, UnitID: unit1.ID, ChunkIndex: 0, Content: "old active fact", EmbeddingText: "old active fact", LocatorJSON: `{}`, MetadataJSON: `{}`, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed", CreatedAt: now, UpdatedAt: now}
	if err := repo.InsertUnitBundle(unit1, []Chunk{chunk1}, nil); err != nil {
		t.Fatalf("insert r1: %v", err)
	}

	rev2 := &SourceRevision{
		ID: sourceRevisionID(src.ID, 2), TenantID: src.TenantID, SourceID: src.ID, Revision: 2,
		Filename: "manual-v2.txt", MimeType: "text/plain", SourceType: "txt", StorageKey: "s/r2", SHA256: "sha-v2",
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: RevisionStatusProcessing, CreatedAt: now, UpdatedAt: now,
	}
	job2 := &IngestJob{ID: "job-2", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 2, Kind: JobKindIngest, Stage: "parse", Status: JobStatusRunning, CursorJSON: `{"ordinal":0}`, LastCommittedOrdinal: -1, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateSourceRevision(rev2, job2); err != nil {
		t.Fatalf("create r2: %v", err)
	}
	unit2 := DocumentUnit{ID: "u2", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 2, Ordinal: 0, UnitType: "text", Content: "new candidate fact", LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: now}
	chunk2 := Chunk{ID: "c2", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 2, UnitID: unit2.ID, ChunkIndex: 0, Content: "new candidate fact", EmbeddingText: "new candidate fact", LocatorJSON: `{}`, MetadataJSON: `{}`, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed", CreatedAt: now, UpdatedAt: now}
	if err := repo.InsertUnitBundle(unit2, []Chunk{chunk2}, nil); err != nil {
		t.Fatalf("insert r2: %v", err)
	}

	newBefore, err := repo.ExactCandidates(src.TenantID, "new candidate", nil, 10)
	if err != nil {
		t.Fatalf("search before cutover: %v", err)
	}
	if len(newBefore) != 0 {
		t.Fatalf("candidate revision leaked before cutover: %#v", newBefore)
	}
	oldBefore, err := repo.ExactCandidates(src.TenantID, "old active", nil, 10)
	if err != nil || len(oldBefore) != 1 || oldBefore[0].SourceRevision != 1 {
		t.Fatalf("active revision missing before cutover: %#v err=%v", oldBefore, err)
	}

	finished := now.Add(time.Minute)
	changed, err := repo.FinalizeIngestJobIfRunning(src.TenantID, job2.ID, src.ID, 2,
		map[string]any{"status": JobStatusCompleted, "stage": "completed", "finished_at": finished},
		map[string]any{"status": RevisionStatusReady, "activated_at": finished},
		map[string]any{"revision": int64(2), "filename": rev2.Filename, "storage_key": rev2.StorageKey, "sha256": rev2.SHA256, "status": SourceStatusReady},
	)
	if err != nil || !changed {
		t.Fatalf("cutover: changed=%v err=%v", changed, err)
	}

	oldAfter, err := repo.ExactCandidates(src.TenantID, "old active", nil, 10)
	if err != nil {
		t.Fatalf("search old after cutover: %v", err)
	}
	if len(oldAfter) != 0 {
		t.Fatalf("old revision remained visible after cutover: %#v", oldAfter)
	}
	newAfter, err := repo.ExactCandidates(src.TenantID, "new candidate", nil, 10)
	if err != nil || len(newAfter) != 1 || newAfter[0].SourceRevision != 2 {
		t.Fatalf("new revision not visible after cutover: %#v err=%v", newAfter, err)
	}
	archived, err := repo.GetSourceRevision(src.TenantID, src.ID, 1)
	if err != nil {
		t.Fatalf("load archived revision: %v", err)
	}
	if archived.RetainUntil == nil || !archived.RetainUntil.After(time.Now().UTC()) {
		t.Fatalf("previous active revision must enter rollback retention window: %#v", archived.RetainUntil)
	}
	oldChunks, err := repo.CountChunks(src.TenantID, src.ID, 1)
	if err != nil || oldChunks != 1 {
		t.Fatalf("cutover must retain old canonical chunks for rollback: chunks=%d err=%v", oldChunks, err)
	}
}

func TestPendingOutboxIsRevisionScoped(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	rows := []IndexOutbox{
		{ID: "o1", TenantID: "tenant-1", AggregateType: "knowledge_source", AggregateID: "source-1", SourceRevision: 1, Operation: "upsert", PayloadJSON: `{}`, Status: "pending", CreatedAt: now, UpdatedAt: now},
		{ID: "o2", TenantID: "tenant-1", AggregateType: "knowledge_source", AggregateID: "source-1", SourceRevision: 2, Operation: "upsert", PayloadJSON: `{}`, Status: "pending", CreatedAt: now, UpdatedAt: now},
	}
	if err := repo.DB().Create(&rows).Error; err != nil {
		t.Fatalf("insert outbox: %v", err)
	}
	for revision, want := range map[int64]string{1: "o1", 2: "o2"} {
		got, err := repo.PendingOutboxesForSourceRevision("tenant-1", "source-1", revision, 10)
		if err != nil {
			t.Fatalf("query revision %d: %v", revision, err)
		}
		if len(got) != 1 || got[0].ID != want {
			t.Fatalf("revision %d outbox mismatch: %#v", revision, got)
		}
	}
}

func seedRollbackPair(t *testing.T, repo *Repository) (*Source, SourceRevision, SourceRevision) {
	t.Helper()
	now := time.Now().UTC().Add(-time.Hour)
	src := &Source{
		ID: "rollback-source", TenantID: "rollback-tenant", LogicalName: "rollback", Filename: "v2.txt",
		MimeType: "text/plain", SourceType: "txt", StorageKey: "rollback/r2", SizeBytes: 20, SHA256: "sha-r2", Revision: 2,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: SourceStatusReady, ExpectedUnits: 1, ProcessedUnits: 1, IndexedChunks: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(src).Error; err != nil {
		t.Fatalf("seed source: %v", err)
	}
	retain := time.Now().UTC().Add(DefaultRevisionRetention)
	activated1 := now
	activated2 := now.Add(10 * time.Minute)
	r1 := SourceRevision{
		ID: sourceRevisionID(src.ID, 1), TenantID: src.TenantID, SourceID: src.ID, Revision: 1,
		Filename: "v1.txt", MimeType: "text/plain", SourceType: "txt", StorageKey: "rollback/r1", SizeBytes: 10, SHA256: "sha-r1",
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: RevisionStatusReady, ExpectedUnits: 1, ProcessedUnits: 1, IndexedChunks: 1, CreatedAt: now, UpdatedAt: now,
		ActivatedAt: &activated1, RetainUntil: &retain,
	}
	r2 := SourceRevision{
		ID: sourceRevisionID(src.ID, 2), TenantID: src.TenantID, SourceID: src.ID, Revision: 2,
		Filename: "v2.txt", MimeType: "text/plain", SourceType: "txt", StorageKey: "rollback/r2", SizeBytes: 20, SHA256: "sha-r2",
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: RevisionStatusReady, ExpectedUnits: 1, ProcessedUnits: 1, IndexedChunks: 1, CreatedAt: now, UpdatedAt: now,
		ActivatedAt: &activated2,
	}
	if err := repo.DB().Create(&[]SourceRevision{r1, r2}).Error; err != nil {
		t.Fatalf("seed revisions: %v", err)
	}
	for revision, text := range map[int64]string{1: "rollback old fact", 2: "rollback current fact"} {
		unit := DocumentUnit{
			ID: "rollback-u" + string(rune('0'+revision)), TenantID: src.TenantID, SourceID: src.ID, SourceRevision: revision,
			Ordinal: 0, UnitType: "text", Content: text, LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: now,
		}
		chunk := Chunk{
			ID: "rollback-c" + string(rune('0'+revision)), TenantID: src.TenantID, SourceID: src.ID, SourceRevision: revision,
			UnitID: unit.ID, ChunkIndex: 0, Content: text, EmbeddingText: text, LocatorJSON: `{}`, MetadataJSON: `{}`,
			EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed", CreatedAt: now, UpdatedAt: now,
		}
		if err := repo.InsertUnitBundle(unit, []Chunk{chunk}, nil); err != nil {
			t.Fatalf("seed revision %d content: %v", revision, err)
		}
	}
	return src, r1, r2
}

func TestRollbackCutoverArchivesFormerActiveRevision(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	job := &IngestJob{
		ID: "rollback-job", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, Kind: JobKindRollback,
		Stage: "queued", Status: JobStatusQueued, CursorJSON: `{"ordinal":0}`, LastCommittedOrdinal: -1,
		ExpectedUnits: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.PrepareRevisionIndexJob(job, 64); err != nil {
		t.Fatalf("prepare rollback: %v", err)
	}
	if err := repo.UpdateJobScoped(src.TenantID, job.ID, map[string]any{"status": JobStatusRunning, "stage": "rollback_index"}); err != nil {
		t.Fatalf("mark rollback running: %v", err)
	}
	changed, err := repo.FinalizeIngestJobIfRunning(
		src.TenantID, job.ID, src.ID, 1,
		map[string]any{"status": JobStatusCompleted, "stage": "completed", "finished_at": now},
		map[string]any{"status": RevisionStatusReady, "activated_at": now, "retain_until": nil},
		map[string]any{"revision": int64(1), "filename": "v1.txt", "storage_key": "rollback/r1", "sha256": "sha-r1", "status": SourceStatusReady},
	)
	if err != nil || !changed {
		t.Fatalf("rollback cutover: changed=%v err=%v", changed, err)
	}
	active, err := repo.GetSource(src.TenantID, src.ID)
	if err != nil || active.Revision != 1 {
		t.Fatalf("rollback did not activate r1: source=%#v err=%v", active, err)
	}
	r1, err := repo.GetSourceRevision(src.TenantID, src.ID, 1)
	if err != nil || r1.RetainUntil != nil {
		t.Fatalf("newly active rollback target must have no GC deadline: %#v err=%v", r1, err)
	}
	r2, err := repo.GetSourceRevision(src.TenantID, src.ID, 2)
	if err != nil || r2.RetainUntil == nil || !r2.RetainUntil.After(time.Now().UTC()) {
		t.Fatalf("former active revision must enter rollback window: %#v err=%v", r2.RetainUntil, err)
	}
}

func TestPurgeRevisionRequiresExpiredRetentionAndEmitsDeleteOutbox(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	if _, err := repo.PurgeSourceRevision(src.TenantID, src.ID, 1, time.Now().UTC()); err == nil {
		t.Fatal("purge inside retention window must fail")
	}
	if _, err := repo.PurgeSourceRevision(src.TenantID, src.ID, 2, time.Now().UTC().Add(30*24*time.Hour)); err == nil {
		t.Fatal("active revision purge must fail")
	}
	expired := time.Now().UTC().Add(-time.Minute)
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"retain_until": expired}); err != nil {
		t.Fatalf("expire revision: %v", err)
	}
	purged, err := repo.PurgeSourceRevision(src.TenantID, src.ID, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("purge expired revision: %v", err)
	}
	if purged.Status != RevisionStatusPurged || purged.PurgedAt == nil {
		t.Fatalf("revision not tombstoned: %#v", purged)
	}
	units, err := repo.CountUnits(src.TenantID, src.ID, 1)
	if err != nil || units != 0 {
		t.Fatalf("purged units remain: units=%d err=%v", units, err)
	}
	chunks, err := repo.CountChunks(src.TenantID, src.ID, 1)
	if err != nil || chunks != 0 {
		t.Fatalf("purged chunks remain: chunks=%d err=%v", chunks, err)
	}
	var outboxes []IndexOutbox
	if err := repo.DB().Where("tenant_id = ? AND aggregate_id = ? AND source_revision = ?", src.TenantID, src.ID, 1).Find(&outboxes).Error; err != nil {
		t.Fatalf("load purge outbox: %v", err)
	}
	if len(outboxes) != 1 || outboxes[0].Operation != "delete" || outboxes[0].Status != "pending" {
		t.Fatalf("purge must emit one durable vector delete outbox: %#v", outboxes)
	}
}

func TestOutboxLeasePreventsConcurrentConsumptionAndStaleCompletion(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	row := IndexOutbox{
		ID: "lease-outbox", TenantID: "lease-tenant", AggregateType: "knowledge_source", AggregateID: "source-1",
		SourceRevision: 1, Operation: "upsert", PayloadJSON: `{}`, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatalf("insert outbox: %v", err)
	}
	first, claimed, err := repo.ClaimNextOutboxForTenant(row.TenantID, now, 5*time.Minute)
	if err != nil || !claimed || first == nil || first.LeaseToken == "" || first.LeaseUntil == nil {
		t.Fatalf("first lease: claimed=%v row=%#v err=%v", claimed, first, err)
	}
	second, secondClaimed, err := repo.ClaimNextOutboxForTenant(row.TenantID, now.Add(time.Second), 5*time.Minute)
	if err != nil {
		t.Fatalf("second lease: %v", err)
	}
	if secondClaimed || second != nil {
		t.Fatalf("unexpired lease was consumed twice: %#v", second)
	}
	if changed, err := repo.FinishOutboxLease(row.TenantID, row.ID, "stale-token", map[string]any{"status": "done"}); err != nil || changed {
		t.Fatalf("stale worker must not finish lease: changed=%v err=%v", changed, err)
	}
	if changed, err := repo.FinishOutboxLease(row.TenantID, row.ID, first.LeaseToken, map[string]any{"status": "done"}); err != nil || !changed {
		t.Fatalf("lease owner failed to finish: changed=%v err=%v", changed, err)
	}
	var got IndexOutbox
	if err := repo.DB().First(&got, "id = ?", row.ID).Error; err != nil {
		t.Fatalf("reload outbox: %v", err)
	}
	if got.Status != "done" || got.LeaseToken != "" || got.LeaseUntil != nil {
		t.Fatalf("finished lease not cleared: %#v", got)
	}
}

func TestExpiredOutboxLeaseCanBeReclaimed(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	expired := now.Add(-time.Minute)
	row := IndexOutbox{
		ID: "expired-outbox", TenantID: "lease-tenant", AggregateType: "knowledge_source", AggregateID: "source-1",
		SourceRevision: 1, Operation: "delete", PayloadJSON: `{}`, Status: "failed", RetryCount: 1,
		LeaseToken: "dead-worker", LeaseUntil: &expired, CreatedAt: now.Add(-time.Hour), UpdatedAt: expired,
	}
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatalf("insert expired outbox: %v", err)
	}
	claimedRow, claimed, err := repo.ClaimNextOutboxForSourceRevision(row.TenantID, row.AggregateID, row.SourceRevision, now, 5*time.Minute)
	if err != nil || !claimed || claimedRow == nil {
		t.Fatalf("reclaim expired lease: claimed=%v row=%#v err=%v", claimed, claimedRow, err)
	}
	if claimedRow.LeaseToken == "" || claimedRow.LeaseToken == "dead-worker" {
		t.Fatalf("reclaim did not rotate lease token: %#v", claimedRow)
	}
}

func TestSourceDetailLifecycleFlagsComeFromCanonicalState(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	svc := &service{repo: repo}
	detail, err := svc.GetSourceDetail(src.TenantID, src.ID)
	if err != nil {
		t.Fatalf("source detail: %v", err)
	}
	if detail.HasActiveJob {
		t.Fatal("seed must not have active job")
	}
	byRevision := map[int64]SourceRevision{}
	for _, revision := range detail.Revisions {
		byRevision[revision.Revision] = revision
	}
	if !byRevision[2].IsActive || byRevision[2].CanRollback || byRevision[2].CanPurge {
		t.Fatalf("active revision lifecycle flags invalid: %#v", byRevision[2])
	}
	if byRevision[1].IsActive || !byRevision[1].CanRollback || byRevision[1].CanPurge {
		t.Fatalf("retained historical revision lifecycle flags invalid: %#v", byRevision[1])
	}
	expired := time.Now().UTC().Add(-time.Minute)
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"retain_until": expired}); err != nil {
		t.Fatalf("expire historical revision: %v", err)
	}
	detail, err = svc.GetSourceDetail(src.TenantID, src.ID)
	if err != nil {
		t.Fatalf("source detail after expiry: %v", err)
	}
	for _, revision := range detail.Revisions {
		if revision.Revision == 1 && !revision.CanPurge {
			t.Fatalf("expired historical revision should be purgeable: %#v", revision)
		}
	}
}

func TestSoftDeleteSourceUsesDurableRevisionScopedVectorDeletes(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	if err := repo.DB().Save(&IndexAlias{
		TenantID: src.TenantID, ActiveVersion: "v2", PreviousVersion: "v1", UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"index_version": "v1"}); err != nil {
		t.Fatalf("set r1 index version: %v", err)
	}
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 2, map[string]any{"index_version": "v2"}); err != nil {
		t.Fatalf("set r2 index version: %v", err)
	}
	staleUpsert := IndexOutbox{
		ID: "stale-upsert", TenantID: src.TenantID, AggregateType: "knowledge_source", AggregateID: src.ID,
		SourceRevision: 2, Operation: "upsert", PayloadJSON: `{}`, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(&staleUpsert).Error; err != nil {
		t.Fatalf("seed stale upsert: %v", err)
	}

	if err := repo.SoftDeleteSource(src.TenantID, src.ID); err != nil {
		t.Fatalf("soft delete source: %v", err)
	}
	if _, err := repo.GetSource(src.TenantID, src.ID); err == nil {
		t.Fatal("tombstoned source must no longer be visible through active source lookup")
	}
	var raw Source
	if err := repo.DB().Where("tenant_id = ? AND id = ?", src.TenantID, src.ID).First(&raw).Error; err != nil {
		t.Fatalf("load tombstoned source: %v", err)
	}
	if raw.DeletedAt == nil || raw.Status != SourceStatusDeleted {
		t.Fatalf("source tombstone not persisted: %#v", raw)
	}
	var revisions []SourceRevision
	if err := repo.DB().Where("tenant_id = ? AND source_id = ?", src.TenantID, src.ID).Find(&revisions).Error; err != nil {
		t.Fatalf("load retained revisions: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected two retained revisions, got %d", len(revisions))
	}
	for _, revision := range revisions {
		if revision.RetainUntil == nil || !revision.RetainUntil.After(now) {
			t.Fatalf("deleted revision %d did not enter retention window: %#v", revision.Revision, revision.RetainUntil)
		}
	}
	var staleCount int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", staleUpsert.ID).Count(&staleCount).Error; err != nil {
		t.Fatalf("count stale upsert: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("stale pending upsert survived source tombstone: %d", staleCount)
	}

	var deletes []IndexOutbox
	if err := repo.DB().Where("tenant_id = ? AND aggregate_id = ? AND operation = ?", src.TenantID, src.ID, "delete").Find(&deletes).Error; err != nil {
		t.Fatalf("load durable deletes: %v", err)
	}
	if len(deletes) != 4 {
		t.Fatalf("expected one delete per revision/index alias pair, got %d: %#v", len(deletes), deletes)
	}
	seen := map[string]bool{}
	for _, row := range deletes {
		if row.Status != "pending" || row.SourceRevision <= 0 {
			t.Fatalf("invalid delete outbox: %#v", row)
		}
		var payload IndexPayload
		if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
			t.Fatalf("decode delete outbox: %v", err)
		}
		if len(payload.ChunkIDs) != 1 {
			t.Fatalf("delete outbox must target retained canonical chunk ids: %#v", payload)
		}
		seen[payload.IndexVersion] = true
	}
	if !seen["v1"] || !seen["v2"] {
		t.Fatalf("delete outbox must cover active and previous index aliases: %#v", seen)
	}
	indexable, err := repo.RevisionIndexable(src.TenantID, src.ID, 2)
	if err != nil {
		t.Fatalf("check tombstoned revision indexability: %v", err)
	}
	if indexable {
		t.Fatal("deleted source revision must be fenced from future vector upserts")
	}
	if err := repo.EnqueueVectorDeleteCorrection(src.TenantID, src.ID, 2, []string{"rollback-c2"}, "v2"); err != nil {
		t.Fatalf("enqueue post-upsert correction: %v", err)
	}
	var correctionCount int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("tenant_id = ? AND aggregate_id = ? AND source_revision = ? AND operation = ? AND status = ?", src.TenantID, src.ID, 2, "delete", "pending").Count(&correctionCount).Error; err != nil {
		t.Fatalf("count corrective deletes: %v", err)
	}
	if correctionCount != 3 {
		t.Fatalf("expected two tombstone deletes plus one post-upsert correction for r2, got %d", correctionCount)
	}
}
