package temporal

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type temporalCandidateCollector struct {
	mu         sync.Mutex
	candidates []ProactiveCandidate
	seen       map[string]bool
	err        error
}

func (c *temporalCandidateCollector) PublishTemporalCandidate(_ context.Context, candidate ProactiveCandidate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.seen[candidate.IdempotencyKey] {
		return nil
	}
	c.seen[candidate.IdempotencyKey] = true
	c.candidates = append(c.candidates, candidate)
	return nil
}

func TestProcessDueAnchorCreatesIdempotentEventAndCandidate(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	service, repo := temporalTestService(t, now)
	collector := &temporalCandidateCollector{}
	service.SetCandidatePublisher(collector)
	occurrence := now.Add(-10 * time.Minute)
	anchor := Anchor{ID: "due-anchor", ScopeType: OwnerSpace, SpaceID: defaultSpaceOwnerID(), AnchorType: "birthday", Title: "生日", TimeKind: "instant", InstantAtUTC: &occurrence, Timezone: "Asia/Shanghai", DurationSeconds: 3600, Importance: 80, Status: "active", AllowProactiveMention: true, NextOccurrenceAtUTC: &occurrence, CreatedAtUTC: now.Add(-time.Hour), UpdatedAtUTC: now.Add(-time.Hour)}
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
	anchor := Anchor{ID: "recurring-anchor", ScopeType: OwnerSpace, SpaceID: defaultSpaceOwnerID(), AnchorType: "custom", Title: "每日提醒", TimeKind: "recurring", LocalDate: "2026-07-16", LocalTime: "12:00", Timezone: "UTC", RRule: "FREQ=DAILY", DurationSeconds: 1800, Importance: 50, Status: "active", AllowProactiveMention: true, NextOccurrenceAtUTC: &occurrence, CreatedAtUTC: occurrence, UpdatedAtUTC: occurrence}
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
	events, err := repo.ListEvents(defaultSpaceOwnerID(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expired recovery created events: %#v", events)
	}
}

func TestSchedulerStartProcessesDueAnchorsImmediately(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	service, repo := temporalTestService(t, now)
	occurrence := now.Add(-10 * time.Minute)
	anchor := Anchor{ID: "scheduler-start-anchor", ScopeType: OwnerSpace, SpaceID: defaultSpaceOwnerID(), AnchorType: "custom", Title: "启动处理", TimeKind: "instant", InstantAtUTC: &occurrence, Timezone: "UTC", DurationSeconds: 3600, Importance: 50, Status: "active", NextOccurrenceAtUTC: &occurrence, CreatedAtUTC: now, UpdatedAtUTC: now}
	if err := repo.SaveAnchor(&anchor); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	scheduler := NewScheduler(service)
	scheduler.Start(ctx)
	t.Cleanup(func() {
		cancel()
		scheduler.Stop()
	})
	events, err := repo.ListEvents(defaultSpaceOwnerID(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].AnchorID != anchor.ID {
		t.Fatalf("scheduler start did not process due anchor: %#v", events)
	}
}

func TestCandidatePublishFailureKeepsAnchorDueForIdempotentRetry(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	service, repo := temporalTestService(t, now)
	collector := &temporalCandidateCollector{err: errors.New("candidate sink unavailable")}
	service.SetCandidatePublisher(collector)
	occurrence := now.Add(-10 * time.Minute)
	anchor := Anchor{ID: "candidate-retry-anchor", ScopeType: OwnerSpace, SpaceID: defaultSpaceOwnerID(), AnchorType: "custom", Title: "候选重试", TimeKind: "instant", InstantAtUTC: &occurrence, Timezone: "UTC", DurationSeconds: 3600, Importance: 50, Status: "active", AllowProactiveMention: true, NextOccurrenceAtUTC: &occurrence, CreatedAtUTC: now, UpdatedAtUTC: now}
	if err := repo.SaveAnchor(&anchor); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessDueAnchors(context.Background(), false); !errors.Is(err, collector.err) {
		t.Fatalf("candidate publish error was not propagated: %v", err)
	}
	saved, err := repo.GetAnchor(anchor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.NextOccurrenceAtUTC == nil || !saved.NextOccurrenceAtUTC.Equal(occurrence) || saved.Status != "active" {
		t.Fatalf("anchor advanced after candidate failure: %#v", saved)
	}
	events, err := repo.ListEvents(defaultSpaceOwnerID(), "", 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("event after first attempt: %#v error=%v", events, err)
	}
	collector.mu.Lock()
	collector.err = nil
	collector.mu.Unlock()
	result, err := service.ProcessDueAnchors(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deduplicated != 1 {
		t.Fatalf("retry did not detect the existing event: %#v", result)
	}
	collector.mu.Lock()
	published := append([]ProactiveCandidate(nil), collector.candidates...)
	collector.mu.Unlock()
	if len(published) != 1 || published[0].IdempotencyKey != events[0].IdempotencyKey {
		t.Fatalf("candidate was not published idempotently: %#v", published)
	}
	saved, err = repo.GetAnchor(anchor.ID)
	if err != nil || saved.Status != "archived" || saved.LastOccurrenceAtUTC == nil {
		t.Fatalf("anchor did not advance after successful retry: %#v error=%v", saved, err)
	}
}
