package mindruntime

import (
	"errors"
	"fmt"
	"strings"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type DefaultOutboxCleanupExecutor struct {
	db *gorm.DB
}

func NewDefaultOutboxCleanupExecutor(db *gorm.DB) *DefaultOutboxCleanupExecutor {
	return &DefaultOutboxCleanupExecutor{db: db}
}

func (e *DefaultOutboxCleanupExecutor) CleanupOutboxItem(item OutboxCleanupItem) error {
	switch strings.ToLower(strings.TrimSpace(item.Storage)) {
	case "qdrant":
		return e.cleanupQdrant(item)
	case "surrealdb":
		return e.cleanupPrimaryStore(item)
	case "cache":
		return e.cleanupCache(item)
	case "summaries":
		return e.cleanupSummaries(item)
	case "reflections", "traces":
		return nil
	default:
		return fmt.Errorf("unsupported cleanup storage %s", item.Storage)
	}
}

func (e *DefaultOutboxCleanupExecutor) cleanupQdrant(item OutboxCleanupItem) error {
	if qdrantDB.Client == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	var cleanupErrs []error
	for _, collection := range qdrantDB.CollectionNames() {
		if err := qdrantDB.DeleteVectors([]string{item.TargetID}, collection); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("%s: %w", collection, err))
		}
	}
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupPrimaryStore(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	likeTarget := "%" + escapeLike(targetID) + "%"
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteIfTableExists("memory_events", "memory_id = ? OR id = ? OR character_id = ? OR source_msg_id = ? OR source_conv_id = ?", targetID, targetID, targetID, targetID, targetID))
	cleanupErrs = append(cleanupErrs, e.deleteIfTableExists("memories", "id = ? OR character_id = ? OR source_msg_id = ? OR source_conv_id = ?", targetID, targetID, targetID, targetID))
	cleanupErrs = append(cleanupErrs, e.deleteIfTableExists("memory_candidates", "id = ? OR character_id = ? OR conversation_id = ?", targetID, targetID, targetID))
	cleanupErrs = append(cleanupErrs, e.deleteIfTableExists("retrieval_logs", "id = ? OR request_id = ? OR conversation_id = ? OR character_id = ? OR retrieved_memory_ids LIKE ? ESCAPE '\\'", targetID, targetID, targetID, targetID, likeTarget))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupCache(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	likeTarget := "%" + escapeLike(targetID) + "%"
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteIfTableExists("memory_embeddings", "memory_id = ?", targetID))
	cleanupErrs = append(cleanupErrs, e.deleteIfTableExists("retrieval_logs", "id = ? OR request_id = ? OR conversation_id = ? OR character_id = ? OR retrieved_memory_ids LIKE ? ESCAPE '\\'", targetID, targetID, targetID, targetID, likeTarget))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupSummaries(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	return e.deleteIfTableExists("conversation_summaries", "id = ? OR conversation_id = ? OR parent_summary_id = ?", targetID, targetID, targetID)
}

func (e *DefaultOutboxCleanupExecutor) deleteIfTableExists(table string, where string, args ...interface{}) error {
	if e.db == nil || !e.db.Migrator().HasTable(table) {
		return nil
	}
	return e.db.Exec("DELETE FROM "+table+" WHERE "+where, args...).Error
}

func joinCleanupErrors(errs []error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return errors.Join(filtered...)
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value
}
