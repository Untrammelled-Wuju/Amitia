package security

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDesktopSessionStoreFailureIsNotReportedAsInvalidSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sessions.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service := &DesktopSessionService{db: db}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = service.ValidateSessionWithContext(context.Background(), "session-token")
	if !errors.Is(err, ErrDesktopSessionStoreUnavailable) {
		t.Fatalf("store error classification = %v", err)
	}
	if errors.Is(err, ErrDesktopSessionInvalid) {
		t.Fatalf("store failure was classified as invalid session: %v", err)
	}
}

func TestDesktopSessionMissingRecordIsInvalidSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sessions.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DesktopSession{}); err != nil {
		t.Fatal(err)
	}
	service := &DesktopSessionService{db: db}
	_, err = service.ValidateSessionWithContext(context.Background(), "session-token")
	if !errors.Is(err, ErrDesktopSessionInvalid) {
		t.Fatalf("missing session classification = %v", err)
	}
}

func TestDesktopSessionPruneRevokesExpiredAndStaleRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sessions.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DesktopSession{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	rows := []DesktopSession{
		{ID: "expired", TokenHash: "expired", Status: DesktopSessionStatusActive, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute), LastUsedAt: now},
		{ID: "stale", TokenHash: "stale", Status: DesktopSessionStatusActive, CreatedAt: now.Add(-25 * time.Hour), ExpiresAt: now.Add(time.Hour), LastUsedAt: now},
		{ID: "active", TokenHash: "active", Status: DesktopSessionStatusActive, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), LastUsedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&DesktopSessionService{db: db}).pruneStaleSessions(now); err != nil {
		t.Fatal(err)
	}
	var active []DesktopSession
	if err := db.Where("status = ?", DesktopSessionStatusActive).Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != "active" {
		t.Fatalf("active sessions after prune = %#v", active)
	}
}
