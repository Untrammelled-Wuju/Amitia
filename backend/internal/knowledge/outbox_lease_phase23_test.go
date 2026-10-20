package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func phase23Outbox(id, tenant, source, operation string, revision int64, now time.Time) IndexOutbox {
	return IndexOutbox{
		ID: id, TenantID: tenant, AggregateType: "knowledge_source", AggregateID: source,
		SourceRevision: revision, Operation: operation, PayloadJSON: `{"chunkIds":[]}`,
		Status: "pending", CreatedAt: now, UpdatedAt: now,
	}
}

func TestPhase23LeaseRenewalAndExpiryFence(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	row := phase23Outbox("renew-me", "tenant-a", "source-a", "upsert", 1, now)
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := repo.ClaimNextOutboxForTenant(row.TenantID, now, 3*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim failed: %v %v", ok, err)
	}
	for name, args := range map[string]struct{ tenant, token string }{
		"foreign_tenant": {"tenant-b", claimed.LeaseToken},
		"stale_token":    {row.TenantID, "no-longer-owner"},
	} {
		updated, err := repo.RenewOutboxLease(args.tenant, row.ID, args.token, now.Add(time.Second), 3*time.Second)
		if err != nil || updated {
			t.Fatalf("%s renewed foreign lease: %v %v", name, updated, err)
		}
	}
	updated, err := repo.RenewOutboxLease(row.TenantID, row.ID, claimed.LeaseToken, now.Add(time.Second), 3*time.Second)
	if err != nil || !updated {
		t.Fatalf("owner could not renew: %v %v", updated, err)
	}
	if _, claimedAgain, err := repo.ClaimNextOutboxForTenant(row.TenantID, now.Add(3500*time.Millisecond), 3*time.Second); err != nil || claimedAgain {
		t.Fatalf("renewed lease reclaimed too early: claimed=%v err=%v", claimedAgain, err)
	}
	updated, err = repo.RenewOutboxLease(row.TenantID, row.ID, claimed.LeaseToken, now.Add(5*time.Second), 3*time.Second)
	if err != nil || updated {
		t.Fatalf("expired lease renewed: %v %v", updated, err)
	}
	reclaimed, claimedAgain, err := repo.ClaimNextOutboxForTenant(row.TenantID, now.Add(5*time.Second), 3*time.Second)
	if err != nil || !claimedAgain || reclaimed.LeaseToken == claimed.LeaseToken {
		t.Fatalf("expired lease not reclaimed: %#v %v %v", reclaimed, claimedAgain, err)
	}
	if updated, err := repo.RenewOutboxLease(row.TenantID, row.ID, claimed.LeaseToken, now.Add(5*time.Second), 3*time.Second); err != nil || updated {
		t.Fatalf("stale owner renewed reclaimed lease: %v %v", updated, err)
	}
}

func TestPhase23ExpiredOwnerCannotFinishUnclaimedLease(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	row := phase23Outbox("expired-finish", "tenant-a", "source-a", "delete", 1, now)
	row.LeaseToken = "old-owner"
	expired := now.Add(-time.Second)
	row.LeaseUntil = &expired
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	changed, err := repo.FinishOutboxLease(row.TenantID, row.ID, row.LeaseToken, map[string]any{"status": "done"})
	if err != nil || changed {
		t.Fatalf("expired lease marked done without reclaim: %v %v", changed, err)
	}
}

func TestPhase23DeletionOrdersLeasedUpsertBeforeDelete(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	src, _, _ := seedRollbackPair(t, repo)
	now := time.Now().UTC()
	upsert := phase23Outbox("leased-upsert", src.TenantID, src.ID, "upsert", 2, now.Add(-time.Minute))
	stale := phase23Outbox("unclaimed-upsert", src.TenantID, src.ID, "upsert", 2, now)
	if err := repo.DB().Create(&[]IndexOutbox{upsert, stale}).Error; err != nil {
		t.Fatal(err)
	}
	claim, ok, err := repo.ClaimNextOutboxForSourceRevision(src.TenantID, src.ID, 2, now, time.Minute)
	if err != nil || !ok || claim.ID != upsert.ID {
		t.Fatalf("wrong task leased: %+v %v %v", claim, ok, err)
	}
	if err := repo.SoftDeleteSource(src.TenantID, src.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", upsert.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("in-flight upsert lease deleted: %d %v", count, err)
	}
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", stale.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unleased upsert survived: %d %v", count, err)
	}
	if next, ok, err := repo.ClaimNextOutboxForSourceRevision(src.TenantID, src.ID, 2, now.Add(time.Second), time.Minute); err != nil || ok || next != nil {
		t.Fatalf("delete ran ahead of active upsert: %+v %v %v", next, ok, err)
	}
	if done, err := repo.FinishOutboxLease(src.TenantID, claim.ID, claim.LeaseToken, map[string]any{"status": "done"}); err != nil || !done {
		t.Fatalf("leased upsert could not finalize: %v %v", done, err)
	}
	claimedDelete, ok, err := repo.ClaimNextOutboxForSourceRevision(src.TenantID, src.ID, 2, time.Now().UTC(), time.Minute)
	if err != nil || !ok || claimedDelete == nil || claimedDelete.Operation != "delete" {
		t.Fatalf("durable vector delete not claimable after upsert: %+v %v %v", claimedDelete, ok, err)
	}
}

func TestPhase23WorkerRenewsLeaseDuringLongOperation(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	row := phase23Outbox("long-running-upsert", "tenant-a", "source-a", "upsert", 1, now)
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	lease := 330 * time.Millisecond
	claimed, ok, err := repo.ClaimNextOutboxForTenant(row.TenantID, now, lease)
	if err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	idx := &Indexer{repo: repo}
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- idx.withOutboxLease(context.Background(), *claimed, lease, func(ctx context.Context) error {
			close(started)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(720 * time.Millisecond):
				return nil
			}
		})
	}()
	<-started
	time.Sleep(420 * time.Millisecond)
	if _, duplicate, err := repo.ClaimNextOutboxForTenant(row.TenantID, time.Now().UTC(), lease); err != nil || duplicate {
		t.Fatalf("active operation lost its lease: duplicate=%v err=%v", duplicate, err)
	}
	if err := <-finished; err != nil {
		t.Fatalf("long operation lost heartbeat: %v", err)
	}
}

func TestPhase23LeaseRevocationCancelsInFlightWork(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	row := phase23Outbox("revoked-upsert", "tenant-a", "source-a", "upsert", 1, now)
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	lease := 270 * time.Millisecond
	claimed, ok, err := repo.ClaimNextOutboxForTenant(row.TenantID, now, lease)
	if err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	idx := &Indexer{repo: repo}
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- idx.withOutboxLease(context.Background(), *claimed, lease, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	<-started
	if err := repo.DB().Model(&IndexOutbox{}).Where("id = ?", row.ID).Update("lease_token", "new-owner").Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, errOutboxLeaseLost) {
			t.Fatalf("revoked worker returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("revoked worker did not stop")
	}
}

func TestPhase23ManualReconcileRequeuesOnlyExhaustedDeletes(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	now := time.Now().UTC()
	max := repo.settings.OutboxMaxRetries
	if max <= 0 {
		max = defaultRuntimeSettings().OutboxMaxRetries
	}
	rows := []IndexOutbox{
		phase23Outbox("exhausted-delete-a", "tenant-a", "source-a", "delete", 1, now),
		phase23Outbox("exhausted-delete-b", "tenant-a", "source-a", "delete", 1, now),
		phase23Outbox("foreign-delete", "tenant-b", "source-a", "delete", 1, now),
		phase23Outbox("exhausted-upsert", "tenant-a", "source-a", "upsert", 1, now),
		phase23Outbox("retryable-delete", "tenant-a", "source-a", "delete", 1, now),
	}
	for i := range rows {
		rows[i].Status = "failed"
		rows[i].RetryCount = max
		rows[i].LastError = "qdrant unavailable"
	}
	rows[4].RetryCount = max - 1
	lease := now.Add(time.Minute)
	rows[1].LeaseUntil = &lease
	rows[1].LeaseToken = "active-owner"
	if err := repo.DB().Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	changed, err := repo.RequeueExhaustedVectorDeletes("tenant-a", 20, now)
	if err != nil || changed != 1 {
		t.Fatalf("retry reset changed %d rows: %v", changed, err)
	}
	want := map[string]struct {
		status  string
		retries int
	}{
		"exhausted-delete-a": {"pending", 0},
		"exhausted-delete-b": {"failed", max},
		"foreign-delete":     {"failed", max},
		"exhausted-upsert":   {"failed", max},
		"retryable-delete":   {"failed", max - 1},
	}
	for id, expected := range want {
		var got IndexOutbox
		if err := repo.DB().First(&got, "id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		if got.Status != expected.status || got.RetryCount != expected.retries {
			t.Fatalf("row %s unexpectedly changed: %#v", id, got)
		}
	}
	changed, err = repo.RequeueExhaustedVectorDeletes("tenant-a", 20, now)
	if err != nil || changed != 0 {
		t.Fatalf("reset not idempotent: %d %v", changed, err)
	}
}
