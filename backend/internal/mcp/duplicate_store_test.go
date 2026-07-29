package mcp

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDuplicateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DuplicateRecord{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDuplicateStore_RecordAndCount(t *testing.T) {
	db := setupDuplicateTestDB(t)
	store := NewDuplicateStore(db)
	ctx := context.Background()

	count, err := store.CountUnresolved(ctx)
	if err != nil {
		t.Fatalf("CountUnresolved failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unresolved, got %d", count)
	}

	if err := store.RecordDuplicate(ctx, "mcp.test.tool1", "test-server", "owner1", 1); err != nil {
		t.Fatalf("RecordDuplicate failed: %v", err)
	}

	count, err = store.CountUnresolved(ctx)
	if err != nil {
		t.Fatalf("CountUnresolved failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unresolved after record, got %d", count)
	}
}

func TestDuplicateStore_ResolveByToolID(t *testing.T) {
	db := setupDuplicateTestDB(t)
	store := NewDuplicateStore(db)
	ctx := context.Background()

	if err := store.RecordDuplicate(ctx, "mcp.test.tool1", "test-server", "owner1", 1); err != nil {
		t.Fatalf("RecordDuplicate failed: %v", err)
	}

	if err := store.ResolveByToolID(ctx, "mcp.test.tool1"); err != nil {
		t.Fatalf("ResolveByToolID failed: %v", err)
	}

	count, err := store.CountUnresolved(ctx)
	if err != nil {
		t.Fatalf("CountUnresolved failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unresolved after resolve, got %d", count)
	}
}

func TestDuplicateStore_ListUnresolvedDetails(t *testing.T) {
	db := setupDuplicateTestDB(t)
	store := NewDuplicateStore(db)
	ctx := context.Background()

	if err := store.RecordDuplicate(ctx, "mcp.server1.tool1", "server1", "owner1", 1); err != nil {
		t.Fatalf("RecordDuplicate failed: %v", err)
	}
	if err := store.RecordDuplicate(ctx, "mcp.server2.tool2", "server2", "owner2", 2); err != nil {
		t.Fatalf("RecordDuplicate failed: %v", err)
	}

	records, err := store.ListUnresolved(ctx)
	if err != nil {
		t.Fatalf("ListUnresolved failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	for _, r := range records {
		if r.ToolID == "" {
			t.Fatalf("record missing ToolID: %+v", r)
		}
		if r.ServerID == "" {
			t.Fatalf("record missing ServerID: %+v", r)
		}
		if r.DetectedAt == "" {
			t.Fatalf("record missing DetectedAt: %+v", r)
		}
		if r.Resolved != 0 {
			t.Fatalf("expected resolved=0, got %d", r.Resolved)
		}
	}

	if err := store.ResolveByToolID(ctx, "mcp.server1.tool1"); err != nil {
		t.Fatalf("ResolveByToolID failed: %v", err)
	}

	records, err = store.ListUnresolved(ctx)
	if err != nil {
		t.Fatalf("ListUnresolved failed after resolve: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after resolve, got %d", len(records))
	}
	if records[0].ToolID != "mcp.server2.tool2" {
		t.Fatalf("expected remaining record ToolID=mcp.server2.tool2, got %s", records[0].ToolID)
	}
}

func TestDuplicateStore_PersistenceAcrossReconnect(t *testing.T) {
	db := setupDuplicateTestDB(t)
	ctx := context.Background()

	store1 := NewDuplicateStore(db)
	if err := store1.RecordDuplicate(ctx, "mcp.persist.tool1", "persist-server", "owner1", 1); err != nil {
		t.Fatalf("RecordDuplicate failed: %v", err)
	}

	store2 := NewDuplicateStore(db)
	count, err := store2.CountUnresolved(ctx)
	if err != nil {
		t.Fatalf("CountUnresolved failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unresolved from new store instance (persistence), got %d", count)
	}

	records, err := store2.ListUnresolved(ctx)
	if err != nil {
		t.Fatalf("ListUnresolved failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record from new store instance, got %d", len(records))
	}
	if records[0].ToolID != "mcp.persist.tool1" {
		t.Fatalf("expected ToolID=mcp.persist.tool1, got %s", records[0].ToolID)
	}
	if records[0].ServerID != "persist-server" {
		t.Fatalf("expected ServerID=persist-server, got %s", records[0].ServerID)
	}
	if records[0].Owner != "owner1" {
		t.Fatalf("expected Owner=owner1, got %s", records[0].Owner)
	}
	if records[0].Generation != 1 {
		t.Fatalf("expected Generation=1, got %d", records[0].Generation)
	}
}
