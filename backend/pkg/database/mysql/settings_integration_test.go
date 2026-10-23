package mysql_test

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
	mysql "github.com/u-ai/backend/pkg/database/mysql"
)

func TestSQLiteRoutingPoolCommitsAppSettingUpdates(t *testing.T) {
	db := mysql.NewSQLite(t.TempDir())
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE app_settings (id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT NOT NULL UNIQUE, value TEXT DEFAULT '', revision INTEGER NOT NULL DEFAULT 1, deleted_at DATETIME, updated_at TEXT DEFAULT '')`).Error; err != nil {
		t.Fatal(err)
	}
	svc := system.NewService(&app.AppContext{DB: db}, "")
	scoped := svc.(interface {
		UpdateMoodDetectionConfigContext(context.Context, map[string]interface{}) (map[string]interface{}, error)
	})
	if _, err := scoped.UpdateMoodDetectionConfigContext(t.Context(), map[string]interface{}{"enabled": true, "threshold": 0.2}); err != nil {
		t.Fatal(err)
	}
	var row struct {
		Value    string
		Revision int64
	}
	if err := db.Table("app_settings").Select("value,revision").Where("key = ?", "mood_detection_enabled").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "true" || row.Revision != 1 {
		t.Fatalf("row=%+v", row)
	}
	restarted := system.NewService(&app.AppContext{DB: db}, "")
	if got := restarted.MoodDetectionConfig(); got["enabled"] != true || got["threshold"] != 0.2 {
		t.Fatalf("restarted config=%+v", got)
	}
}
