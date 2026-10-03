package main

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"gorm.io/gorm"
)

func newDesktopPetBehaviorMeshTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDesktopPetBehaviorMeshDurableEventPersistsBeforeOfflineDelivery(t *testing.T) {
	db := newDesktopPetBehaviorMeshTestDB(t)
	publisher, err := newDesktopPetBehaviorMeshPublisher(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event := behavior.BehaviorEventEnvelope{
		EventID:       "event-shared-id",
		EventType:     "chat.response.completed",
		OccurredAt:    now,
		ReceivedAt:    now,
		SpaceID:       "cloud-user-a",
		CharacterID:   "character-a",
		DedupKey:      "dedup-a",
		SchemaVersion: 1,
	}

	// No Device Mesh runtime is attached. Durable publication still succeeds
	// because acceptance is the committed outbox row, not transport success.
	if err := publisher.PublishBehaviorEvent(context.Background(), event); err != nil {
		t.Fatalf("durable event should persist while target is offline: %v", err)
	}

	var row desktopPetBehaviorMeshOutbox
	if err := db.Where("cloud_space_id = ? AND event_id = ?", event.SpaceID, event.EventID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "pending" || row.AttemptCount != 1 {
		t.Fatalf("expected retryable pending row after offline dispatch, got status=%s attempts=%d", row.Status, row.AttemptCount)
	}
	if row.AvailableAt.Before(now) || row.LastError == "" || row.PayloadHash == "" {
		t.Fatalf("retry/idempotency metadata not persisted: %#v", row)
	}

	// Reusing an opaque event ID with different content in the same tenant is
	// an idempotency conflict, not a successful replay.
	conflict := event
	conflict.DedupKey = "dedup-conflict"
	if err := publisher.PublishBehaviorEvent(context.Background(), conflict); err == nil {
		t.Fatal("same-tenant event ID reuse with different payload must be rejected")
	}

	// Idempotent same-tenant publication cannot create a second outbox row.
	if err := publisher.PublishBehaviorEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	var sameTenantCount int64
	if err := db.Model(&desktopPetBehaviorMeshOutbox{}).
		Where("cloud_space_id = ? AND event_id = ?", event.SpaceID, event.EventID).
		Count(&sameTenantCount).Error; err != nil {
		t.Fatal(err)
	}
	if sameTenantCount != 1 {
		t.Fatalf("expected one idempotent same-tenant row, got %d", sameTenantCount)
	}

	// Event IDs are not a cross-tenant primary-key boundary. A different cloud
	// user may legitimately produce the same opaque ID without observing or
	// consuming another tenant's delivery row.
	event.SpaceID = "cloud-user-b"
	event.CharacterID = "character-b"
	event.DedupKey = "dedup-b"
	if err := publisher.PublishBehaviorEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	var totalCount int64
	if err := db.Model(&desktopPetBehaviorMeshOutbox{}).Where("event_id = ?", event.EventID).Count(&totalCount).Error; err != nil {
		t.Fatal(err)
	}
	if totalCount != 2 {
		t.Fatalf("expected tenant-scoped outbox identity, got %d rows", totalCount)
	}
}

func TestResolveDeviceActivePetRequiresCredentialOwnerAndActiveBinding(t *testing.T) {
	db := newDesktopPetBehaviorMeshTestDB(t)
	if err := db.AutoMigrate(&installation.Installation{}, &binding.DeviceActiveInstallationBinding{}); err != nil {
		t.Fatal(err)
	}
	mapper, err := newDesktopPetOwnerMapper(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := mapper.Bind(context.Background(), "cloud-user", "device-a", "local-owner"); err != nil {
		t.Fatal(err)
	}
	repo := installation.NewRepository(db, nil)
	inst := &installation.Installation{
		ID:               "installation-a",
		SpaceID:          "local-owner",
		DeviceID:         "device-a",
		PetID:            "pet-a",
		CurrentReleaseID: "release-a",
		Status:           installation.StatusEnabled,
		DesiredState:     installation.DesiredEnabled,
		IsActive:         1,
		LastEnabledAt:    "2026-08-30 19:00:00",
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatal(err)
	}
	activeBinding := &binding.DeviceActiveInstallationBinding{
		SpaceID:         "local-owner",
		DeviceID:        "device-a",
		InstallationID:  inst.ID,
		PetID:           inst.PetID,
		ReleaseID:       inst.CurrentReleaseID,
		BindingRevision: 7,
	}
	if err := db.Create(activeBinding).Error; err != nil {
		t.Fatal(err)
	}
	services := &AppServices{
		DB:                    db,
		InstallationRepo:      repo,
		DesktopPetOwnerMapper: mapper,
	}

	resolved, err := resolveDeviceActivePet(context.Background(), services, "cloud-user", "device-a", "character-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Active || resolved.LocalOwnerID != "local-owner" || resolved.InstallationID != inst.ID || resolved.BindingRevision != 7 {
		t.Fatalf("unexpected active resolution: %#v", resolved)
	}

	otherContext, err := resolveDeviceActivePet(context.Background(), services, "cloud-user", "device-a", "character-b", "")
	if err != nil {
		t.Fatal(err)
	}
	if !otherContext.Active {
		t.Fatalf("desktop pet resolution must ignore character context: %#v", otherContext)
	}

	staleInstallation, err := resolveDeviceActivePet(context.Background(), services, "cloud-user", "device-a", "character-a", "installation-stale")
	if err != nil {
		t.Fatal(err)
	}
	if staleInstallation.Active {
		t.Fatalf("stale installation fence must not drift to the device's current active installation: %#v", staleInstallation)
	}

	if _, err := resolveDeviceActivePet(context.Background(), services, "cloud-user", "device-b", "character-a", ""); err == nil {
		t.Fatal("unmapped device must fail owner resolution")
	}
}
