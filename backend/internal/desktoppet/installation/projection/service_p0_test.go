package projection

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProjectionCreatesIdentityAndNeverRegressesAppliedRevision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&InstallationRuntimeProjection{}); err != nil {
		t.Fatalf("migrate projection: %v", err)
	}
	svc := NewService(db)
	ctx := context.Background()
	if err := svc.HandleRuntimeHeartbeat(ctx, "u", "d", "r", &coordinator.RuntimeHeartbeat{
		InstallationID: "inst", PetID: "pet", AppliedDesiredRevision: 10, AppliedSettingsRevision: 7,
		ActualReleaseID: "rel", ActualHealth: "healthy", Timestamp: "2026-08-18T00:00:00Z",
	}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if err := svc.HandleCommandResult(ctx, "u", "d", &coordinator.CommandResult{Success: true, AppliedRevision: 3, Timestamp: "2026-08-18T00:00:01Z"}); err != nil {
		t.Fatalf("command result: %v", err)
	}
	var got InstallationRuntimeProjection
	if err := db.Where("user_id = ? AND device_id = ?", "u", "d").Take(&got).Error; err != nil {
		t.Fatalf("load projection: %v", err)
	}
	if got.ID == "" {
		t.Fatal("projection id must be generated")
	}
	if got.AppliedDesiredRevision != 10 {
		t.Fatalf("desired revision regressed: %d", got.AppliedDesiredRevision)
	}
	if got.AppliedSettingsRevision != 7 {
		t.Fatalf("settings revision regressed: %d", got.AppliedSettingsRevision)
	}
}
