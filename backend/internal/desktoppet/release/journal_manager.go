// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package release

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type journalManager struct {
	db *gorm.DB
}

func NewJournalManager(db *gorm.DB) JournalManagerPort {
	return &journalManager{db: db}
}

func (m *journalManager) GetByOperation(operationID string) (*ReleasePublishJournal, error) {
	if operationID == "" {
		return nil, fmt.Errorf("journal manager: operation id is empty")
	}
	var journal ReleasePublishJournal
	if err := m.db.Where("operation_id = ?", operationID).Take(&journal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("journal manager: get by operation: %w", err)
	}
	return &journal, nil
}

func (m *journalManager) MarkFailed(journal *ReleasePublishJournal, errMsg string) error {
	if journal == nil {
		return fmt.Errorf("journal manager: journal is nil")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	journal.Stage = JournalStageFailed
	journal.ErrorMessage = errMsg
	journal.UpdatedAt = now
	if err := m.db.Model(&ReleasePublishJournal{}).Where("id = ?", journal.ID).Updates(map[string]interface{}{
		"stage":         JournalStageFailed,
		"error_message": errMsg,
		"updated_at":    now,
	}).Error; err != nil {
		return fmt.Errorf("journal manager: mark failed: %w", err)
	}
	return nil
}

func (m *journalManager) UpdateStage(journal *ReleasePublishJournal, stage, contentRootHash, stagingPath, publishedPath string) error {
	if journal == nil {
		return fmt.Errorf("journal manager: journal is nil")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"stage":      stage,
		"updated_at": now,
	}
	if contentRootHash != "" {
		updates["content_root_hash"] = contentRootHash
	}
	if stagingPath != "" {
		updates["staging_path"] = stagingPath
	}
	if publishedPath != "" {
		updates["published_path"] = publishedPath
	}
	if err := m.db.Model(&ReleasePublishJournal{}).Where("id = ?", journal.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("journal manager: update stage: %w", err)
	}
	journal.Stage = stage
	journal.ContentRootHash = contentRootHash
	if stagingPath != "" {
		journal.StagingPath = stagingPath
	}
	if publishedPath != "" {
		journal.PublishedPath = publishedPath
	}
	journal.UpdatedAt = now
	return nil
}

var _ JournalManagerPort = (*journalManager)(nil)
