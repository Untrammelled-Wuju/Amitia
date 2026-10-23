package mindruntime

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func usageTestStore(t *testing.T) (*ModelUsageStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "usage.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	return store, db
}

func TestModelUsagePersistsScopesAndResetsWithoutErasing(t *testing.T) {
	store, db := usageTestStore(t)
	record := func(device, request string, tokens int64, reported bool) {
		t.Helper()
		id, err := store.Begin(t.Context(), ModelUsageRecord{SpaceID: "core", DeviceID: device, RequestID: request, Model: "model", Provider: "provider", Source: "web"})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Finish(t.Context(), id, ModelUsageRecord{Status: "succeeded", TotalTokens: tokens, UsageReported: reported}); err != nil {
			t.Fatal(err)
		}
		if err := store.Finish(t.Context(), id, ModelUsageRecord{Status: "failed", TotalTokens: 999}); err != nil {
			t.Fatal(err)
		}
	}
	record("phone", "request", 120, true)
	record("phone", "request", 30, true)
	record("desktop", "other", 70, true)
	rows, _, err := store.Aggregate(t.Context(), UsageFilter{SpaceID: "core", DeviceID: "phone"}, "overview")
	if err != nil || len(rows) != 1 || rows[0].Calls != 2 || rows[0].Tokens != 150 {
		t.Fatalf("bad device usage: %+v %v", rows, err)
	}
	cutoff, err := store.Clear(t.Context(), UsageFilter{SpaceID: "core", DeviceID: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	record("phone", "after", 20, true)
	rows, baseline, err := store.Aggregate(t.Context(), UsageFilter{SpaceID: "core", DeviceID: "phone"}, "overview")
	if err != nil || baseline.Sequence != cutoff || rows[0].Calls != 1 || rows[0].Tokens != 20 {
		t.Fatalf("bad reset: %+v %+v %v", rows, baseline, err)
	}
	rows, _, _ = store.Aggregate(t.Context(), UsageFilter{SpaceID: "core", DeviceID: "desktop"}, "overview")
	if rows[0].Tokens != 70 {
		t.Fatal("other device baseline modified")
	}
	var count int64
	db.Model(&ModelUsageRecord{}).Count(&count)
	if count != 4 {
		t.Fatal("reset deleted original calls")
	}
	reopened, err := NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	rows, _, _ = reopened.Aggregate(t.Context(), UsageFilter{SpaceID: "core", DeviceID: "phone"}, "overview")
	if rows[0].Tokens != 20 {
		t.Fatal("reset or calls lost after restart")
	}
}

func TestModelUsageUnknownZeroFailuresAndRestart(t *testing.T) {
	store, db := usageTestStore(t)
	id, err := store.Begin(t.Context(), ModelUsageRecord{Model: "unknown", Source: "system"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(t.Context(), id, ModelUsageRecord{Status: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	id, _ = store.Begin(t.Context(), ModelUsageRecord{Model: "zero", Source: "system"})
	if err := store.Finish(t.Context(), id, ModelUsageRecord{Status: "succeeded", UsageReported: true}); err != nil {
		t.Fatal(err)
	}
	store.Begin(t.Context(), ModelUsageRecord{Model: "inflight", Source: "system"})
	reopened, err := NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	rows, _, err := reopened.Aggregate(t.Context(), UsageFilter{SpaceID: "core"}, "overview")
	if err != nil || rows[0].Unknown != 2 || rows[0].Interrupted != 1 || rows[0].Running != 0 || rows[0].Calls != 3 {
		t.Fatalf("unknown or interrupted calls misreported: %+v %v", rows, err)
	}
}
