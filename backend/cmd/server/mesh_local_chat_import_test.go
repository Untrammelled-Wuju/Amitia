package main

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/chat/importbatch"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

func TestManageImportBatchUsesLogicalSpace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&importbatch.Batch{}, &importbatch.Item{}); err != nil {
		t.Fatal(err)
	}
	batch := &importbatch.Batch{
		ID:             "import-one",
		SpaceID:        "space-one",
		DeviceID:       "device-one",
		RoleID:         "role-one",
		Title:          "Imported chat",
		Status:         importbatch.StatusReady,
		IdempotencyKey: "import-one",
		CreatedAt:      "2026-11-04 20:00:00",
		UpdatedAt:      "2026-11-04 20:00:00",
	}
	items := []importbatch.Item{
		{ID: "item-one", BatchID: batch.ID, Sequence: 1, Role: "user", Content: "hello", CreatedAt: batch.CreatedAt},
	}
	if err := db.Create(batch).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	port := &meshLocalDataPort{
		services: &AppServices{DB: db},
		ownerID:  "device-one",
		spaceID:  "space-one",
	}
	result, err := port.manageImportBatch(
		context.Background(),
		coordination.ExecutionScope{SpaceID: "space-one", ResourceOwnerID: "device-one", CoreID: "space-one"},
		"role-one",
		coordination.ChatManagementRequest{ImportBatchID: batch.ID, RequestID: "request-one"},
	)
	if err != nil {
		t.Fatal(err)
	}
	conversation, ok := result["conversation"].(map[string]any)
	if !ok || conversation["title"] != batch.Title {
		t.Fatalf("conversation = %v, want title %q", result["conversation"], batch.Title)
	}
}
