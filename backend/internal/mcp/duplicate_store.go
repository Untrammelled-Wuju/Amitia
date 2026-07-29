package mcp

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type DuplicateRecord struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ToolID     string `gorm:"column:tool_id;not null" json:"toolId"`
	ServerID   string `gorm:"column:server_id;not null" json:"serverId"`
	Owner      string `gorm:"column:owner;not null;default:''" json:"owner"`
	Generation int64  `gorm:"column:generation;not null;default:0" json:"generation"`
	DetectedAt string `gorm:"column:detected_at;not null" json:"detectedAt"`
	Resolved   int    `gorm:"column:resolved;not null;default:0" json:"resolved"`
}

func (DuplicateRecord) TableName() string {
	return "mcp_duplicate_tool_registrations"
}

type DuplicateStore struct {
	db *gorm.DB
}

func NewDuplicateStore(db *gorm.DB) *DuplicateStore {
	return &DuplicateStore{db: db}
}

func (s *DuplicateStore) RecordDuplicate(ctx context.Context, toolID, serverID, owner string, generation int64) error {
	record := DuplicateRecord{
		ToolID:     toolID,
		ServerID:   serverID,
		Owner:      owner,
		Generation: generation,
		DetectedAt: time.Now().UTC().Format(time.RFC3339),
		Resolved:   0,
	}
	return s.db.WithContext(ctx).Create(&record).Error
}

func (s *DuplicateStore) CountUnresolved(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&DuplicateRecord{}).Where("resolved = 0").Count(&count).Error
	return count, err
}

func (s *DuplicateStore) ListUnresolved(ctx context.Context) ([]DuplicateRecord, error) {
	var records []DuplicateRecord
	err := s.db.WithContext(ctx).Where("resolved = 0").Order("detected_at DESC").Find(&records).Error
	return records, err
}

func (s *DuplicateStore) ResolveByToolID(ctx context.Context, toolID string) error {
	return s.db.WithContext(ctx).Model(&DuplicateRecord{}).
		Where("tool_id = ? AND resolved = 0", toolID).
		Update("resolved", 1).Error
}
