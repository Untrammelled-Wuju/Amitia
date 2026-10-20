package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanonicalReaderFeatureGateAndConcurrencyCancellation(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	disabled := &service{repo: repo, settings: runtimeSettings{ConfigHash: "cfg", CanonicalReaderEnabled: false}}
	if _, err := disabled.Read(context.Background(), "tenant-a", ReadRequest{UnitID: "unit-a"}); err == nil || err.Error() != "canonical reader is disabled" {
		t.Fatalf("disabled reader error = %v", err)
	}

	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	limited := &service{repo: repo, settings: runtimeSettings{ConfigHash: "cfg", CanonicalReaderEnabled: true}, readSem: sem}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := limited.Read(ctx, "tenant-a", ReadRequest{UnitID: "unit-a"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("reader waiting for capacity must honor cancellation, got %v", err)
	}
}

func TestDiagnosticRetentionCleanupIsTenantScopedAndBatched(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	if err := repo.DB().Exec(`CREATE TABLE retrieval_traces (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, created_at DATETIME NOT NULL)`).Error; err != nil {
		t.Fatalf("create trace table: %v", err)
	}
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	fresh := now.Add(-time.Hour)
	rows := []struct {
		id, tenant string
		created    time.Time
	}{
		{"trace-a-1", "tenant-a", old.Add(-time.Hour)},
		{"trace-a-2", "tenant-a", old},
		{"trace-a-new", "tenant-a", fresh},
		{"trace-b-old", "tenant-b", old},
	}
	for _, row := range rows {
		if err := repo.DB().Exec(`INSERT INTO retrieval_traces (id, tenant_id, created_at) VALUES (?, ?, ?)`, row.id, row.tenant, row.created).Error; err != nil {
			t.Fatalf("insert trace: %v", err)
		}
	}

	deleted, err := repo.DeleteRetrievalTracesBefore("tenant-a", now.Add(-24*time.Hour), 1)
	if err != nil || deleted != 1 {
		t.Fatalf("trace cleanup deleted=%d err=%v", deleted, err)
	}
	var tenantAOld, tenantAFresh, tenantBOld int64
	repo.DB().Table("retrieval_traces").Where("tenant_id = ? AND created_at < ?", "tenant-a", now.Add(-24*time.Hour)).Count(&tenantAOld)
	repo.DB().Table("retrieval_traces").Where("id = ?", "trace-a-new").Count(&tenantAFresh)
	repo.DB().Table("retrieval_traces").Where("id = ?", "trace-b-old").Count(&tenantBOld)
	if tenantAOld != 1 || tenantAFresh != 1 || tenantBOld != 1 {
		t.Fatalf("trace retention scope mismatch oldA=%d freshA=%d oldB=%d", tenantAOld, tenantAFresh, tenantBOld)
	}

	benchmarks := []BenchmarkRunRecord{
		{ID: "bench-a-old", TenantID: "tenant-a", CreatedAt: old},
		{ID: "bench-a-new", TenantID: "tenant-a", CreatedAt: fresh},
		{ID: "bench-b-old", TenantID: "tenant-b", CreatedAt: old},
	}
	if err := repo.DB().Create(&benchmarks).Error; err != nil {
		t.Fatalf("insert benchmark history: %v", err)
	}
	deleted, err = repo.DeleteBenchmarkRunsBefore("tenant-a", now.Add(-24*time.Hour), 50)
	if err != nil || deleted != 1 {
		t.Fatalf("benchmark cleanup deleted=%d err=%v", deleted, err)
	}
	var oldA, newA, oldB int64
	repo.DB().Model(&BenchmarkRunRecord{}).Where("id = ?", "bench-a-old").Count(&oldA)
	repo.DB().Model(&BenchmarkRunRecord{}).Where("id = ?", "bench-a-new").Count(&newA)
	repo.DB().Model(&BenchmarkRunRecord{}).Where("id = ?", "bench-b-old").Count(&oldB)
	if oldA != 0 || newA != 1 || oldB != 1 {
		t.Fatalf("benchmark retention scope mismatch oldA=%d newA=%d oldB=%d", oldA, newA, oldB)
	}
}

func TestRuntimeStatusReflectsPhase14Settings(t *testing.T) {
	settings := defaultRuntimeSettings()
	svc := &service{settings: settings}
	status := svc.GetRuntimeStatus()
	if status.ConfigHash != settings.ConfigHash {
		t.Fatalf("config hash = %q, want %q", status.ConfigHash, settings.ConfigHash)
	}
	if !status.RetrievalPolicyEnabled || status.RetrievalPolicyShadow || !status.CanonicalReaderEnabled || !status.AgenticInvestigationEnabled {
		t.Fatalf("unexpected feature status: %+v", status)
	}
	if status.ReaderWorkers != settings.ReaderWorkers {
		t.Fatalf("reader workers = %d, want %d", status.ReaderWorkers, settings.ReaderWorkers)
	}
	if status.RetrievalTraceRetentionHours != int64(settings.RetrievalTraceRetention/time.Hour) {
		t.Fatalf("trace retention = %d", status.RetrievalTraceRetentionHours)
	}
	if status.BenchmarkRetentionHours != int64(settings.BenchmarkRetention/time.Hour) {
		t.Fatalf("benchmark retention = %d", status.BenchmarkRetentionHours)
	}
	if status.DiagnosticCleanupBatchSize != settings.DiagnosticCleanupBatchSize {
		t.Fatalf("cleanup batch = %d, want %d", status.DiagnosticCleanupBatchSize, settings.DiagnosticCleanupBatchSize)
	}
}
