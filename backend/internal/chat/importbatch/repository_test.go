package importbatch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "imports.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Batch{}, &Item{}); err != nil {
		t.Fatal(err)
	}
	return NewRepository(db)
}

func TestCreateOrGetPersistsStagingAndIsIdempotent(t *testing.T) {
	repo := newTestRepository(t)
	batch := &Batch{ID: "imp-one", SpaceID: "space", DeviceID: "device", RoleID: "role", Title: "Imported", Status: StatusReady, IdempotencyKey: "content-hash", CreatedAt: "2026-10-23 12:00:00", UpdatedAt: "2026-10-23 12:00:00"}
	items := []Item{{ID: "imp-one-item-000001", BatchID: batch.ID, Sequence: 1, Role: "user", Content: "hello", CreatedAt: batch.CreatedAt}}
	created, storedItems, duplicate, err := repo.CreateOrGet(context.Background(), batch, items)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || created.MessageCount != 1 || len(storedItems) != 1 {
		t.Fatalf("created=%+v items=%d duplicate=%v", created, len(storedItems), duplicate)
	}
	replay := &Batch{ID: "imp-two", SpaceID: "space", DeviceID: "device", RoleID: "role", Title: "Imported", Status: StatusReady, IdempotencyKey: "content-hash", CreatedAt: batch.CreatedAt, UpdatedAt: batch.UpdatedAt}
	existing, existingItems, duplicate, err := repo.CreateOrGet(context.Background(), replay, items)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate || existing.ID != batch.ID || len(existingItems) != 1 {
		t.Fatalf("existing=%+v items=%d duplicate=%v", existing, len(existingItems), duplicate)
	}
}

func TestPromotionIsStableAndCannotChangeTarget(t *testing.T) {
	repo := newTestRepository(t)
	batch := &Batch{ID: "imp-one", SpaceID: "space", DeviceID: "device", RoleID: "role", Title: "Imported", Status: StatusReady, IdempotencyKey: "hash", CreatedAt: "2026-10-23 12:00:00", UpdatedAt: "2026-10-23 12:00:00"}
	if _, _, _, err := repo.CreateOrGet(context.Background(), batch, []Item{{ID: "item", BatchID: batch.ID, Sequence: 1, Role: "user", Content: "hello", CreatedAt: batch.CreatedAt}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPromoted(context.Background(), "space", batch.ID, "role", "import-conversation", "device", "core"); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPromoted(context.Background(), "space", batch.ID, "role", "import-conversation", "device", "core"); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPromoted(context.Background(), "space", batch.ID, "role", "other-conversation", "device", "core"); !errors.Is(err, ErrAlreadyPromoted) {
		t.Fatalf("expected promoted conflict, got %v", err)
	}
	stored, _, err := repo.Get(context.Background(), "space", batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != StatusPromoted || stored.PromotedConversationID != "import-conversation" {
		t.Fatalf("batch=%+v", stored)
	}
}
