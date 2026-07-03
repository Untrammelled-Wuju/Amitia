package mindruntime

import (
	"errors"
	"fmt"
	"strings"

	graph "github.com/u-ai/backend/internal/graph"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type DefaultOutboxCleanupExecutor struct {
	db       *gorm.DB
	graphSvc graph.Service
}

func NewDefaultOutboxCleanupExecutor(db *gorm.DB, graphSvc graph.Service) *DefaultOutboxCleanupExecutor {
	return &DefaultOutboxCleanupExecutor{db: db, graphSvc: graphSvc}
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

func (e *DefaultOutboxCleanupExecutor) cleanupSurrealDB(item OutboxCleanupItem) error {
	if e.graphSvc == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	if err := e.graphSvc.DeleteNode(targetID); err != nil {
		return fmt.Errorf("surrealdb delete node %s: %w", targetID, err)
	}
	return nil
}

func (e *DefaultOutboxCleanupExecutor) cleanupPrimaryStore(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	targetKind := strings.ToLower(strings.TrimSpace(item.TargetKind))

	switch targetKind {
	case "memory":
		var errs []error
		errs = append(errs, e.deleteByExactColumns("memories", targetID, "id"))
		errs = append(errs, e.deleteByExactColumns("memory_events", targetID, "memory_id", "id"))
		return joinCleanupErrors(errs)
	case "message":
		return e.deleteByExactColumns("messages", targetID, "id")
	case "character":
		var errs []error
		errs = append(errs, e.deleteByExactColumns("memory_events", targetID, "character_id"))
		errs = append(errs, e.deleteByExactColumns("memories", targetID, "character_id"))
		errs = append(errs, e.deleteByExactColumns("memory_candidates", targetID, "character_id"))
		errs = append(errs, e.deleteByExactColumns("retrieval_logs", targetID, "character_id"))
		errs = append(errs, e.deleteByExactColumns("messages", targetID, "character_id"))
		return joinCleanupErrors(errs)
	case "conversation":
		var errs []error
		errs = append(errs, e.deleteByExactColumns("messages", targetID, "conversation_id"))
		errs = append(errs, e.deleteByExactColumns("memory_candidates", targetID, "conversation_id"))
		errs = append(errs, e.deleteByExactColumns("retrieval_logs", targetID, "conversation_id"))
		return joinCleanupErrors(errs)
	default:
		return fmt.Errorf("unknown target kind %q for primary store cleanup", targetKind)
	}
}

func (e *DefaultOutboxCleanupExecutor) cleanupCache(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	targetKind := strings.ToLower(strings.TrimSpace(item.TargetKind))

	switch targetKind {
	case "memory":
		return joinCleanupErrors([]error{
			e.deleteByExactColumns("memory_embeddings", targetID, "memory_id"),
		})
	case "character":
		return joinCleanupErrors([]error{
			e.deleteByExactColumns("retrieval_logs", targetID, "character_id"),
		})
	default:
		return fmt.Errorf("cleanup cache: unknown target kind %q", targetKind)
	}
}

func (e *DefaultOutboxCleanupExecutor) cleanupSummaries(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	targetKind := strings.ToLower(strings.TrimSpace(item.TargetKind))

	switch targetKind {
	case "conversation":
		return e.deleteByExactColumns("conversation_summaries", targetID, "conversation_id")
	case "memory":
		return e.deleteByExactColumns("memory_summaries", targetID, "memory_id")
	default:
		return fmt.Errorf("cleanup summaries: unknown target kind %q", targetKind)
	}
}

func (e *DefaultOutboxCleanupExecutor) cleanupReflections(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	targetKind := strings.ToLower(strings.TrimSpace(item.TargetKind))

	switch targetKind {
	case "character":
		var errs []error
		errs = append(errs, e.deleteByExactColumns("reflection_candidates", targetID, "character_id"))
		errs = append(errs, e.deleteByExactColumns("reflection_runs", targetID, "character_id"))
		return joinCleanupErrors(errs)
	case "memory", "message":
		return nil
	default:
		return fmt.Errorf("cleanup reflections: unknown target kind %q", targetKind)
	}
}

func (e *DefaultOutboxCleanupExecutor) cleanupTraces(item OutboxCleanupItem) error {
	if e.db == nil || strings.TrimSpace(item.TargetID) == "" {
		return nil
	}
	targetID := strings.TrimSpace(item.TargetID)
	targetKind := strings.ToLower(strings.TrimSpace(item.TargetKind))

	switch targetKind {
	case "character":
		return joinCleanupErrors([]error{
			e.deleteByExactColumns("runtime_trace_events", targetID, "character_id"),
			e.deleteByExactColumns("pipeline_checkpoints", targetID, "character_id"),
		})
	case "conversation":
		return joinCleanupErrors([]error{
			e.deleteByExactColumns("runtime_trace_events", targetID, "conversation_id"),
			e.deleteByExactColumns("runtime_replay_records", targetID, "request_id"),
			e.deleteByExactColumns("pipeline_checkpoints", targetID, "conversation_id"),
		})
	case "memory", "message":
		return nil
	default:
		return fmt.Errorf("cleanup traces: unknown target kind %q", targetKind)
	}
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
