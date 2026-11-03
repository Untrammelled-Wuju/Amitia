package attention

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func attentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&BudgetPolicy{}, &Event{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCheckShadowModeRecordsWithoutSuppressing(t *testing.T) {
	db := attentionTestDB(t)
	service := NewService(db)
	policy := DefaultPolicy("space")
	policy.ShadowMode = true
	policy.MaxUnchangedNotifications = 1
	if _, err := service.PutPolicy(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	first, err := service.Check(context.Background(), CheckInput{SpaceID: "space", SourceKind: "task", SourceID: "one", EventKind: "unchanged", IdempotencyKey: "one"})
	if err != nil || !first.Allowed || first.Shadowed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.Check(context.Background(), CheckInput{SpaceID: "space", SourceKind: "task", SourceID: "two", EventKind: "unchanged", IdempotencyKey: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Allowed || !second.Shadowed || second.Event.DeliveryState != "observed" {
		t.Fatalf("second=%+v", second)
	}
	stats, err := service.Stats(context.Background(), "space")
	if err != nil {
		t.Fatal(err)
	}
	if stats.TodayObserved != 1 || stats.UnchangedNotifications != 2 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestCheckEnforcementSuppressesAndDeduplicates(t *testing.T) {
	db := attentionTestDB(t)
	service := NewService(db)
	policy := DefaultPolicy("space")
	policy.ShadowMode = false
	policy.MaxRepeatedConfirmations = 1
	if _, err := service.PutPolicy(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	first, err := service.Check(context.Background(), CheckInput{SpaceID: "space", SourceKind: "approval", SourceID: "one", EventKind: "confirmation", IdempotencyKey: "approval-one"})
	if err != nil || !first.Allowed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.Check(context.Background(), CheckInput{SpaceID: "space", SourceKind: "approval", SourceID: "two", EventKind: "confirmation", IdempotencyKey: "approval-two"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Allowed || second.Event.DeliveryState != "suppressed" {
		t.Fatalf("second=%+v", second)
	}
	duplicate, err := service.Check(context.Background(), CheckInput{SpaceID: "space", SourceKind: "approval", SourceID: "one", EventKind: "confirmation", IdempotencyKey: "approval-one"})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.Allowed || duplicate.Event.DeliveryState != "suppressed" {
		t.Fatalf("duplicate=%+v", duplicate)
	}
}

func TestHasEventUsesPersistedIdempotencyKey(t *testing.T) {
	db := attentionTestDB(t)
	service := NewService(db)
	if _, err := service.Check(context.Background(), CheckInput{
		SpaceID: "space", SourceKind: "approval", SourceID: "one", EventKind: "notification",
		IdempotencyKey: "attention:approval-one:pending",
	}); err != nil {
		t.Fatal(err)
	}
	found, err := service.HasEvent(context.Background(), "space", "attention:approval-one:pending")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected persisted attention event")
	}
	missing, err := service.HasEvent(context.Background(), "space", "attention:approval-one:failed")
	if err != nil {
		t.Fatal(err)
	}
	if missing {
		t.Fatal("unexpected attention event for a different state")
	}
}
