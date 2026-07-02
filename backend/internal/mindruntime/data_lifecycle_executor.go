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
	case "surrealdb", "primary", "sqlite":
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
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("memory_events", targetID, nil, "memory_id", "id", "character_id", "source_msg_id", "source_conv_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("memories", targetID, nil, "id", "character_id", "source_msg_id", "source_conv_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("memory_candidates", targetID, nil, "id", "character_id", "conversation_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("retrieval_logs", targetID, []string{"retrieved_memory_ids"}, "id", "request_id", "conversation_id", "character_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("messages", targetID, []string{"content"}, "id", "conversation_id", "character_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupCache(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("memory_embeddings", targetID, nil, "memory_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("retrieval_logs", targetID, []string{"retrieved_memory_ids"}, "id", "request_id", "conversation_id", "character_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupSummaries(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("conversation_summaries", targetID, nil, "id", "conversation_id", "parent_summary_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("memory_summaries", targetID, []string{"summary"}, "id", "target_id", "memory_id", "conversation_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupReflections(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("reflection_candidates", targetID, []string{"source_ids", "evidence"}, "id", "character_id", "target_id", "source_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("reflection_runs", targetID, []string{"source_ids"}, "id", "character_id", "target_id", "source_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("supervisor_decisions", targetID, []string{"target", "reason"}, "id", "target_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("version_history", targetID, []string{"target", "snapshot_hash"}, "id", "target_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) cleanupTraces(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	var cleanupErrs []error
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("runtime_trace_events", targetID, []string{"payload"}, "id", "event_id", "request_id", "conversation_id", "character_id", "parent_id", "causation_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("runtime_replay_records", targetID, []string{"payload"}, "id", "event_id", "request_id", "target_id"))
	cleanupErrs = append(cleanupErrs, e.deleteByColumns("pipeline_checkpoints", targetID, nil, "id", "conversation_id", "character_id", "last_message_id"))
	return joinCleanupErrors(cleanupErrs)
}

func (e *DefaultOutboxCleanupExecutor) deleteIfTableExists(table string, where string, args ...interface{}) error {
	if e.db == nil || !e.db.Migrator().HasTable(table) {
		return nil
	}
	return e.db.Exec("DELETE FROM "+table+" WHERE "+where, args...).Error
}

func (e *DefaultOutboxCleanupExecutor) deleteByColumns(table string, targetID string, likeColumns []string, exactColumns ...string) error {
	if e.db == nil || strings.TrimSpace(targetID) == "" || !e.db.Migrator().HasTable(table) {
		return nil
	}
	whereParts := make([]string, 0, len(exactColumns)+len(likeColumns))
	args := make([]interface{}, 0, len(exactColumns)+len(likeColumns))
	for _, column := range exactColumns {
		if e.db.Migrator().HasColumn(table, column) {
			whereParts = append(whereParts, column+" = ?")
			args = append(args, targetID)
		}
	}
	likeTarget := "%" + escapeLike(targetID) + "%"
	for _, column := range likeColumns {
		if e.db.Migrator().HasColumn(table, column) {
			whereParts = append(whereParts, column+" LIKE ? ESCAPE '\\'")
			args = append(args, likeTarget)
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

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value
}
