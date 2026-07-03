package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/companion"
	"gorm.io/gorm"
)

type randomBurstContextRecorder struct {
	companion.Service
	ctxs []context.Context
	ids  []string
}

func (r *randomBurstContextRecorder) RandomBurstTriggerContext(ctx context.Context, characterID string) map[string]interface{} {
	r.ctxs = append(r.ctxs, ctx)
	r.ids = append(r.ids, characterID)
	return map[string]interface{}{"triggered": false}
}

func TestRandomBurstCronPassesStopContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cron.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.Exec("CREATE TABLE characters (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO characters (id) VALUES ('char-1')").Error; err != nil {
		t.Fatal(err)
	}

	stopCh := make(chan struct{})
	ctx, cancel := contextFromStopCh(stopCh)
	defer cancel()

	recorder := &randomBurstContextRecorder{}
	cron := &ProactiveCron{db: db, compSvc: recorder}
	cron.triggerRandomBurst(ctx)
	if len(recorder.ctxs) != 1 {
		t.Fatalf("expected one random burst call, got %d", len(recorder.ctxs))
	}
	if recorder.ids[0] != "char-1" {
		t.Fatalf("expected char-1, got %q", recorder.ids[0])
	}

	close(stopCh)
	<-recorder.ctxs[0].Done()
	if recorder.ctxs[0].Err() != context.Canceled {
		t.Fatalf("expected stop context cancellation, got %v", recorder.ctxs[0].Err())
	}
}
