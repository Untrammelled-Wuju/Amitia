package system

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openIntegrityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db") + "?_pragma=foreign_keys(ON)"
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return db
}

func TestCheckDBIntegrityReturnsHealthyContractForSQLite(t *testing.T) {
	service := &service{db: openIntegrityTestDB(t)}
	result := service.CheckDBIntegrity()
	if result["ok"] != true || result["status"] != "ok" {
		t.Fatalf("result=%+v", result)
	}
	errors, ok := result["errors"].([]string)
	if !ok || len(errors) != 0 {
		t.Fatalf("errors=%#v", result["errors"])
	}
}

func TestCheckDBIntegrityReportsForeignKeyViolation(t *testing.T) {
	db := openIntegrityTestDB(t)
	for _, statement := range []string{
		`CREATE TABLE parent_record(id INTEGER PRIMARY KEY)`,
		`CREATE TABLE child_record(id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent_record(id))`,
		`INSERT INTO child_record(id, parent_id) VALUES(1, 99)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := &service{db: db}
	result := service.CheckDBIntegrity()
	if result["ok"] != false || result["status"] != "degraded" {
		t.Fatalf("result=%+v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, "child_record") || !strings.Contains(text, "parent_record") {
		t.Fatalf("missing foreign key detail: %s", text)
	}
}
