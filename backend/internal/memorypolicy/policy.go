package memorypolicy

import (
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
)

func ForCharacter(db *gorm.DB, id string) (string, bool) {
	if db == nil || strings.TrimSpace(id) == "" {
		return "", false
	}
	var row struct {
		Enabled bool `gorm:"column:global_memory_enabled"`
	}
	err := db.Session(&gorm.Session{NewDB: true, Logger: logger.Default.LogMode(logger.Silent)}).Table("characters").Select("global_memory_enabled").Where("id=? AND deleted_at IS NULL", id).Take(&row).Error
	if err != nil {
		return "", false
	}
	if row.Enabled {
		return "global", true
	}
	return "character", true
}
