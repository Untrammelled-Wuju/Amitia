package temporal

import (
	"context"
	"sync"
	"testing"
	"time"
)

type temporalCandidateCollector struct {
	mu         sync.Mutex
	candidates []ProactiveCandidate
}

func (c *temporalCandidateCollector) PublishTemporalCandidate(_ context.Context, candidate ProactiveCandidate) error {
	c.mu.Lock()
	c.candidates = append(c.candidates, candidate)
	c.mu.Unlock()
	return nil
}

func TestProcessDueAnchorCreatesIdempotentEventAndCandidate(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	service, repo := temporalTestService(t, now)
	collector := &temporalCandidateCollector{}
	service.SetCandidatePublisher(collector)
	occurrence := now.Add(-10 * time.Minute)
	anchor := Anchor{ID: "due-anchor", ScopeType: OwnerUser, UserID: DefaultUserOwnerID, AnchorType: "birthday", Title: "生日", TimeKind: "instant", InstantAtUTC: &occurrence, Timezone: "Asia/Shanghai", DurationSeconds: 3600, Importance: 80, Status: "active", AllowProactiveMention: true, NextOccurrenceAtUTC: &occurrence, CreatedAtUTC: now.Add(-time.Hour), UpdatedAtUTC: now.Add(-time.Hour)}
	if err := repo.SaveAnchor(&anchor); err != nil {
		t.Fatal(err)
	}
	result, err := service.ProcessDueAnchors(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Emitted != 1 || len(collector.candidates) != 1 {
		t.Fatalf("unexpected first processing result %#v candidates=%d", result, len(collector.candidates))
	}
	anchor.Status = "active"
	anchor.NextOccurrenceAtUTC = &occurrence
	if err := repo.SaveAnchor(&anchor); err != nil {
		t.Fatal(err)
	}
	result, err = service.ProcessDueAnchors(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deduplicated != 1 || len(collector.candidates) != 1 {
		t.Fatalf("expected idempotent processing, got %#v candidates=%d", result, len(collector.candidates))
	}
}

func TestRecoverySkipsExpiredOccurrenceAndAdvancesRecurrence(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	service, repo := temporalTestService(t, now)
	collector := &temporalCandidateCollector{}
	service.SetCandidatePublisher(collector)
	occurrence := now.Add(-48 * time.Hour)
	anchor := Anchor{ID: "recurring-anchor", ScopeType: OwnerUser, UserID: DefaultUserOwnerID, AnchorType: "custom", Title: "每日提醒", TimeKind: "recurring", LocalDate: "2026-07-16", LocalTime: "12:00", Timezone: "UTC", RRule: "FREQ=DAILY", DurationSeconds: 1800, Importance: 50, Status: "active", AllowProactiveMention: true, NextOccurrenceAtUTC: &occurrence, CreatedAtUTC: occurrence, UpdatedAtUTC: occurrence}
	if err := repo.SaveAnchor(&anchor); err != nil {
		t.Fatal(err)
	}
	result, err := service.ProcessDueAnchors(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.SkippedExpired != 1 || result.Emitted != 0 || len(collector.candidates) != 0 {
		t.Fatalf("recovery must not backfill expired proactive work: %#v", result)
	}
	saved, err := repo.GetAnchor(anchor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.NextOccurrenceAtUTC == nil || saved.NextOccurrenceAtUTC.Before(now) {
		t.Fatalf("expected next occurrence at or after now, got %v", saved.NextOccurrenceAtUTC)
	}
	events, err := repo.ListEvents(DefaultUserOwnerID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expired recovery created events: %#v", events)
	}
}
