package notificationruntime

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openNotificationTestDB(t *testing.T) (*gorm.DB, error) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notifications.db")), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	connection, err := db.DB()
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, nil
}
