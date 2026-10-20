package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhase25OutboxHealthTenantIsolationAndDeadLetters(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	repo.settings.OutboxMaxRetries = 3
	now := time.Now().UTC()
	rows := []IndexOutbox{
		phase23Outbox("health-pending-upsert", "tenant-a", "source-a", "upsert", 1, now.Add(-time.Minute)),
		phase23Outbox("health-old-pending", "tenant-a", "source-a", "upsert", 1, now.Add(-time.Hour)),
		phase23Outbox("health-retrying-delete", "tenant-a", "source-a", "delete", 1, now.Add(-time.Minute)),
		phase23Outbox("health-exhausted-delete", "tenant-a", "source-a", "delete", 1, now.Add(-2*time.Hour)),
		phase23Outbox("health-exhausted-upsert", "tenant-a", "source-a", "upsert", 1, now.Add(-time.Hour)),
		phase23Outbox("health-active-lease", "tenant-a", "source-a", "delete", 1, now.Add(-time.Minute)),
		phase23Outbox("health-foreign-exhausted", "tenant-b", "source-b", "delete", 1, now.Add(-5*time.Hour)),
		phase23Outbox("health-completed", "tenant-a", "source-a", "delete", 1, now.Add(-3*time.Hour)),
	}
	rows[2].Status, rows[2].RetryCount = "failed", 2
	rows[3].Status, rows[3].RetryCount = "failed", 3
	rows[4].Status, rows[4].RetryCount = "failed", 3
	rows[5].LeaseUntil = ptrPhase25Time(now.Add(time.Minute))
	rows[5].LeaseToken = "running"
	rows[6].Status, rows[6].RetryCount = "failed", 3
	rows[7].Status = "done"
	for _, row := range rows {
		if err := repo.DB().Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	health, err := repo.GetIndexOutboxHealth(context.Background(), "tenant-a", now)
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != "blocked" || health.ExhaustedDeletes != 1 || health.ExhaustedUpserts != 1 || health.RetryingDeletes != 1 || health.PendingUpserts != 2 || health.PendingDeletes != 1 || health.ActiveLeases != 1 || health.OverdueOperations != 1 {
		t.Fatalf("unexpected tenant-a counters: %+v", health)
	}
	if health.OldestOutstandingSeconds < 7000 || health.OldestOutstandingSeconds > 7300 {
		t.Fatalf("unexpected oldest pending age: %+v", health)
	}
	foreign, err := repo.GetIndexOutboxHealth(context.Background(), "tenant-b", now)
	if err != nil || foreign.ExhaustedDeletes != 1 || foreign.PendingDeletes != 0 || foreign.ExhaustedUpserts != 0 || foreign.OldestOutstandingSeconds < 17000 {
		t.Fatalf("tenant isolation failed: %+v %v", foreign, err)
	}
	empty, err := repo.GetIndexOutboxHealth(context.Background(), "tenant-c", now)
	if err != nil || empty.Status != "healthy" || empty.OldestOutstandingSeconds != 0 || empty.ExhaustedDeletes != 0 {
		t.Fatalf("empty scope invalid: %+v %v", empty, err)
	}
}

func TestPhase25OutboxHealthRequeueRecoveryAndScopeValidation(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	repo.settings.OutboxMaxRetries = 2
	now := time.Now().UTC()
	blocked := phase23Outbox("health-requeue", "tenant-a", "source-a", "delete", 1, now.Add(-time.Hour))
	blocked.Status, blocked.RetryCount, blocked.LastError = "failed", 2, "internal secret from worker"
	if err := repo.DB().Create(&blocked).Error; err != nil {
		t.Fatal(err)
	}
	initial, err := repo.GetIndexOutboxHealth(context.Background(), "tenant-a", now)
	if err != nil || initial.Status != "blocked" || initial.ExhaustedDeletes != 1 {
		t.Fatalf("initial: %+v %v", initial, err)
	}
	n, err := repo.RequeueExhaustedVectorDeletes("tenant-a", 1, now)
	if err != nil || n != 1 {
		t.Fatalf("requeue: %d %v", n, err)
	}
	recovered, err := repo.GetIndexOutboxHealth(context.Background(), "tenant-a", now)
	if err != nil || recovered.ExhaustedDeletes != 0 || recovered.PendingDeletes != 1 || recovered.Status != "degraded" {
		t.Fatalf("requeue health: %+v %v", recovered, err)
	}
	if _, err := repo.GetIndexOutboxHealth(context.Background(), " ", now); err == nil {
		t.Fatal("empty tenant accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.GetIndexOutboxHealth(ctx, "tenant-a", now); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel ignored: %v", err)
	}
}

func ptrPhase25Time(t time.Time) *time.Time { return &t }
