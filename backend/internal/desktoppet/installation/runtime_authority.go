package installation

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const activeRuntimeSessionStatuses = "('registering','syncing','ready','degraded','closing')"

type runtimeSessionAuthority struct {
	ID                   string `gorm:"column:id;primaryKey;type:text"`
	SpaceID              string `gorm:"column:space_id;type:text"`
	DeviceID             string `gorm:"column:device_id;type:text"`
	RuntimeID            string `gorm:"column:runtime_id;type:text"`
	ConnectionGeneration int64  `gorm:"column:connection_generation;type:integer"`
	Status               string `gorm:"column:status;type:text"`
	LastHeartbeatAt      string `gorm:"column:last_heartbeat_at;type:text"`
	UpdatedAt            string `gorm:"column:updated_at;type:text"`
}

func (runtimeSessionAuthority) TableName() string { return "desktop_pet_runtime_sessions" }

type runtimeActualStateAuthority struct {
	SpaceID              string `gorm:"column:space_id;primaryKey;type:text"`
	DeviceID             string `gorm:"column:device_id;primaryKey;type:text"`
	RuntimeID            string `gorm:"column:runtime_id;primaryKey;type:text"`
	RuntimeSessionID     string `gorm:"column:runtime_session_id;type:text"`
	ConnectionGeneration int64  `gorm:"column:connection_generation;type:integer"`
	InstallationID       string `gorm:"column:installation_id;type:text"`
	HealthStatus         string `gorm:"column:health_status;type:text"`
	UpdatedAt            string `gorm:"column:updated_at;type:text"`
}

func (runtimeActualStateAuthority) TableName() string {
	return "desktop_pet_runtime_actual_states_v2"
}

func loadActiveRuntimeSessionAuthority(db *gorm.DB, spaceID, deviceID, runtimeID string) (*runtimeSessionAuthority, error) {
	if db == nil {
		return nil, fmt.Errorf("load active runtime session: database handle is nil")
	}
	query := db.Model(&runtimeSessionAuthority{}).
		Where("space_id = ? AND device_id = ?", spaceID, deviceID).
		Where("status IN " + activeRuntimeSessionStatuses)
	if trimmed := strings.TrimSpace(runtimeID); trimmed != "" {
		query = query.Where("runtime_id = ?", trimmed)
	}
	var row runtimeSessionAuthority
	if err := query.Order("connection_generation DESC").Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load active runtime session: %w", err)
	}
	return &row, nil
}

func loadRuntimeActualStateAuthority(db *gorm.DB, spaceID, deviceID, runtimeID string) (*runtimeActualStateAuthority, error) {
	if db == nil {
		return nil, fmt.Errorf("load runtime actual state: database handle is nil")
	}
	trimmed := strings.TrimSpace(runtimeID)
	if trimmed == "" {
		return nil, nil
	}
	var row runtimeActualStateAuthority
	if err := db.Model(&runtimeActualStateAuthority{}).
		Where("space_id = ? AND device_id = ? AND runtime_id = ?", spaceID, deviceID, trimmed).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load runtime actual state: %w", err)
	}
	return &row, nil
}

func runtimeAuthorityMatchesSession(actual *runtimeActualStateAuthority, session *runtimeSessionAuthority) bool {
	if actual == nil || session == nil {
		return false
	}
	if actual.RuntimeSessionID != session.ID {
		return false
	}
	if actual.ConnectionGeneration != session.ConnectionGeneration {
		return false
	}
	return actual.RuntimeID == session.RuntimeID
}

func normalizeRuntimeTimestamp(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano)
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", trimmed, time.Local); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano)
	}
	return trimmed
}
