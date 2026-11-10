package installation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRuntimeAuthorityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "runtime-authority.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Installation{}, &desired.RuntimeDesiredState{}, &projection.InstallationRuntimeProjection{}, &runtimeSessionAuthority{}, &runtimeActualStateAuthority{}); err != nil {
		t.Fatalf("migrate runtime authority schema: %v", err)
	}
	if err := db.Create(&desired.RuntimeDesiredState{
		ID:              "desired-one",
		SpaceID:         "space-one",
		DeviceID:        "device-one",
		RuntimeID:       "runtime-one",
		InstallationID:  "installation-one",
		PetID:           "pet-one",
		ReleaseID:       "release-one",
		DesiredEnabled:  true,
		DesiredVisible:  true,
		DesiredRevision: 9,
		UpdatedAt:       "2026-10-23T12:00:00Z",
	}).Error; err != nil {
		t.Fatalf("create desired state: %v", err)
	}
	if err := db.Create(&projection.InstallationRuntimeProjection{
		ID:                      "projection-one",
		SpaceID:                 "space-one",
		DeviceID:                "device-one",
		RuntimeID:               "runtime-one",
		InstallationID:          "installation-one",
		PetID:                   "pet-one",
		AppliedDesiredRevision:  8,
		AppliedSettingsRevision: 2,
		ActualReleaseID:         "release-one",
		ActualVisible:           1,
		ActualActionKey:         "idle_normal",
		ActualHealth:            "syncing",
		RuntimeSyncState:        "applied",
		LastHeartbeatAt:         "2026-10-23T08:00:01Z",
		UpdatedAt:               "2026-10-23T08:00:05Z",
	}).Error; err != nil {
		t.Fatalf("create runtime projection: %v", err)
	}
	return db
}

func createRuntimeSession(t *testing.T, db *gorm.DB, sessionID string, generation int64, heartbeat string) {
	t.Helper()
	if err := db.Create(&runtimeSessionAuthority{
		ID:                   sessionID,
		SpaceID:              "space-one",
		DeviceID:             "device-one",
		RuntimeID:            "runtime-one",
		ConnectionGeneration: generation,
		Status:               "ready",
		LastHeartbeatAt:      heartbeat,
		UpdatedAt:            heartbeat,
	}).Error; err != nil {
		t.Fatalf("create runtime session: %v", err)
	}
}

func createRuntimeActualState(t *testing.T, db *gorm.DB, sessionID string, generation int64, health, installationID string) {
	t.Helper()
	if err := db.Create(&runtimeActualStateAuthority{
		SpaceID:              "space-one",
		DeviceID:             "device-one",
		RuntimeID:            "runtime-one",
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		InstallationID:       installationID,
		HealthStatus:         health,
		UpdatedAt:            "2026-10-23 08:00:09",
	}).Error; err != nil {
		t.Fatalf("create runtime actual state: %v", err)
	}
}

func TestRuntimeViewUsesSessionHeartbeatAndAuthoritativeHealth(t *testing.T) {
	db := newRuntimeAuthorityTestDB(t)
	createRuntimeSession(t, db, "rtsess-one", 4, "2026-10-23 08:10:11")
	createRuntimeActualState(t, db, "rtsess-one", 4, "healthy", "installation-one")

	handler := NewCoordinatorHandler(nil, NewRepository(db, nil), nil)
	view, err := handler.loadRuntimeView(context.Background(), "space-one", "device-one", "installation-one")
	if err != nil {
		t.Fatalf("load runtime view: %v", err)
	}
	if view == nil {
		t.Fatal("runtime view is nil")
	}
	wantHeartbeat := time.Date(2026, 10, 23, 8, 10, 11, 0, time.Local).UTC().Format(time.RFC3339Nano)
	if view.LastHeartbeatAt != wantHeartbeat {
		t.Fatalf("lastHeartbeatAt=%q want=%q", view.LastHeartbeatAt, wantHeartbeat)
	}
	if view.LastSnapshotAt != "2026-10-23T08:00:01Z" {
		t.Fatalf("lastSnapshotAt=%q want snapshot capture time", view.LastSnapshotAt)
	}
	if view.ActualHealth != "healthy" {
		t.Fatalf("actualHealth=%q want authoritative healthy", view.ActualHealth)
	}
	if view.RuntimeGeneration != 4 {
		t.Fatalf("runtimeGeneration=%d want=4", view.RuntimeGeneration)
	}
}

func TestRuntimeViewRejectsStaleGenerationAuthority(t *testing.T) {
	db := newRuntimeAuthorityTestDB(t)
	createRuntimeSession(t, db, "rtsess-current", 6, "2026-10-23 08:10:11")
	createRuntimeActualState(t, db, "rtsess-previous", 5, "failed", "installation-one")

	handler := NewCoordinatorHandler(nil, NewRepository(db, nil), nil)
	view, err := handler.loadRuntimeView(context.Background(), "space-one", "device-one", "installation-one")
	if err != nil {
		t.Fatalf("load runtime view: %v", err)
	}
	if view == nil {
		t.Fatal("runtime view is nil")
	}
	if view.ActualHealth != "syncing" {
		t.Fatalf("actualHealth=%q want projection fallback for stale authority", view.ActualHealth)
	}
	if view.RuntimeGeneration != 6 {
		t.Fatalf("runtimeGeneration=%d want=6", view.RuntimeGeneration)
	}
}

func TestRuntimeViewIgnoresAuthorityOfOtherInstallation(t *testing.T) {
	db := newRuntimeAuthorityTestDB(t)
	createRuntimeSession(t, db, "rtsess-one", 3, "2026-10-23 08:10:11")
	createRuntimeActualState(t, db, "rtsess-one", 3, "healthy", "installation-other")

	handler := NewCoordinatorHandler(nil, NewRepository(db, nil), nil)
	view, err := handler.loadRuntimeView(context.Background(), "space-one", "device-one", "installation-one")
	if err != nil {
		t.Fatalf("load runtime view: %v", err)
	}
	if view == nil {
		t.Fatal("runtime view is nil")
	}
	if view.ActualHealth != "syncing" {
		t.Fatalf("actualHealth=%q want projection fallback for foreign installation", view.ActualHealth)
	}
}

func TestNormalizeRuntimeTimestampAcceptsBothStoredFormats(t *testing.T) {
	cases := map[string]string{
		"2026-10-23T08:00:01Z": "2026-10-23T08:00:01Z",
		"2026-10-23 16:00:01":  time.Date(2026, 10, 23, 16, 0, 1, 0, time.Local).UTC().Format(time.RFC3339Nano),
		"":                     "",
		"not-a-timestamp":      "not-a-timestamp",
	}
	for input, want := range cases {
		if got := normalizeRuntimeTimestamp(input); got != want {
			t.Fatalf("normalizeRuntimeTimestamp(%q)=%q want=%q", input, got, want)
		}
	}
}
