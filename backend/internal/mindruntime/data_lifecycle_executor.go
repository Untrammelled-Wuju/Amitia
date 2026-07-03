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
		return e.cleanupSurrealDB(item)
	case "primary", "sqlite":
		return e.cleanupPrimaryStore(item)
	case "cache":
		return e.cleanupCache(item)
	case "summaries":
		return e.cleanupSummaries(item)
	case "reflections":
		return e.cleanupReflections(item)
	case "traces":
		return e.cleanupTraces(item)
	default:
		return fmt.Errorf("unsupported cleanup storage %s", item.Storage)
	}
}

func (e *DefaultOutboxCleanupExecutor) cleanupQdrant(item OutboxCleanupItem) error {
	if qdrantDB.Client == nil || strings.TrimSpace(item.TargetID) == "" {
		return fmt.Errorf("qdrant client not connected, cannot complete cleanup for %s", item.TargetID)
	}
	var cleanupErrs []error
	for _, collection := range qdrantDB.CollectionNames() {
		if err := qdrantDB.DeleteVectors([]string{item.TargetID}, collection); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("%s: %w", collection, err))
		}
	}
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupSurrealDB(item OutboxCleanupItem) error {
	return fmt.Errorf("surrealdb cleanup not yet implemented for %s", item.TargetID)
}

func (e *DefaultOutboxCleanupExecutor) cleanupPrimaryStore(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("memory_events", targetID, "memory_id", "id", "character_id", "source_msg_id", "source_conv_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("memories", targetID, "id", "character_id", "source_msg_id", "source_conv_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("memory_candidates", targetID, "id", "character_id", "conversation_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("retrieval_logs", targetID, "id", "request_id", "conversation_id", "character_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("messages", targetID, "id", "conversation_id", "character_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupCache(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("memory_embeddings", targetID, "memory_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("retrieval_logs", targetID, "id", "request_id", "conversation_id", "character_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupSummaries(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("conversation_summaries", targetID, "id", "conversation_id", "parent_summary_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("memory_summaries", targetID, "id", "target_id", "memory_id", "conversation_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupReflections(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("reflection_candidates", targetID, "id", "character_id", "target_id", "source_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("reflection_runs", targetID, "id", "character_id", "target_id", "source_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("supervisor_decisions", targetID, "id", "target_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("version_history", targetID, "id", "target_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupTraces(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("runtime_trace_events", targetID, "id", "event_id", "request_id", "conversation_id", "character_id", "parent_id", "causation_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("runtime_replay_records", targetID, "id", "event_id", "request_id", "target_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByExactColumns("pipeline_checkpoints", targetID, "id", "conversation_id", "character_id", "last_message_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) deleteIfTableExists(table string, where string, args ...interface{}) error {
	if e.db == nil || !e.db.Migrator().HasTable(table) {
		return nil
	}
	return e.db.Exec("DELETE FROM "+table+" WHERE "+where, args...).Error
}

func (e *DefaultOutboxCleanupExecutor) deleteByExactColumns(table string, targetID string, exactColumns ...string) error {
	if e.db == nil || strings.TrimSpace(targetID) == "" || !e.db.Migrator().HasTable(table) {
		return nil
	}
	whereParts := make([]string, 0, len(exactColumns))
	args := make([]interface{}, 0, len(exactColumns))
	for _, column := range exactColumns {
		if e.db.Migrator().HasColumn(table, column) {
			whereParts = append(whereParts, column+" = ?")
			args = append(args, targetID)
		}
	}
	if len(whereParts) == 0 {
		return nil
	}
	return e.deleteIfTableExists(table, strings.Join(whereParts, " OR "), args...)
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
