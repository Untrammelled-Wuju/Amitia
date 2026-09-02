package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type workflowSyncContextKey struct{}

// SuppressWorkflowSync marks a mutation as a replication/apply operation. The
// definition must still be persisted, but it must not be re-enqueued into the
// local sync outbox or it would bounce forever between Cloud and devices.
func SuppressWorkflowSync(ctx context.Context) context.Context {
	return context.WithValue(ctx, workflowSyncContextKey{}, true)
}

func workflowSyncSuppressed(ctx context.Context) bool {
	value, _ := ctx.Value(workflowSyncContextKey{}).(bool)
	return value
}

type WorkflowSyncEventType string

const (
	WorkflowSyncUpsert WorkflowSyncEventType = "upsert"
	WorkflowSyncDelete WorkflowSyncEventType = "delete"
)

type WorkflowSyncStatus string

const (
	WorkflowSyncFastForward WorkflowSyncStatus = "FAST_FORWARD"
	WorkflowSyncConflict    WorkflowSyncStatus = "CONFLICT"
	WorkflowSyncDeviceAhead WorkflowSyncStatus = "DEVICE_AHEAD"
	WorkflowSyncCloudAhead  WorkflowSyncStatus = "CLOUD_AHEAD"
	WorkflowSyncDiverged    WorkflowSyncStatus = "DIVERGED"
	WorkflowSyncDuplicate   WorkflowSyncStatus = "DUPLICATE"
)

type WorkflowSyncOutboxEvent struct {
	EventID        string                `json:"eventId"`
	OwnerUserID    string                `json:"ownerUserId"`
	WorkflowID     string                `json:"workflowId"`
	Revision       int64                 `json:"revision"`
	BaseRevision   int64                 `json:"baseRevision"`
	EventType      WorkflowSyncEventType `json:"eventType"`
	DefinitionHash string                `json:"definitionHash,omitempty"`
	Payload        json.RawMessage       `json:"payload"`
	CreatedAt      time.Time             `json:"createdAt"`
	SentAt         *time.Time            `json:"sentAt,omitempty"`
	AckedAt        *time.Time            `json:"ackedAt,omitempty"`
	RetryCount     int                   `json:"retryCount"`
	NextRetryAt    time.Time             `json:"nextRetryAt"`
}

type WorkflowSyncState struct {
	OwnerUserID    string    `json:"ownerUserId"`
	WorkflowID     string    `json:"workflowId"`
	Revision       int64     `json:"revision"`
	DefinitionHash string    `json:"definitionHash,omitempty"`
	Deleted        bool      `json:"deleted"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type WorkflowSyncApplyResult struct {
	EventID           string             `json:"eventId"`
	WorkflowID        string             `json:"workflowId"`
	Status            WorkflowSyncStatus `json:"status"`
	Accepted          bool               `json:"accepted"`
	CanonicalRevision int64              `json:"canonicalRevision"`
	CanonicalHash     string             `json:"canonicalHash,omitempty"`
}

type WorkflowSyncConflict struct {
	EventID           string                `json:"eventId"`
	SourceDeviceID    string                `json:"sourceDeviceId"`
	OwnerUserID       string                `json:"ownerUserId"`
	WorkflowID        string                `json:"workflowId"`
	Revision          int64                 `json:"revision"`
	BaseRevision      int64                 `json:"baseRevision"`
	EventType         WorkflowSyncEventType `json:"eventType"`
	DefinitionHash    string                `json:"definitionHash,omitempty"`
	Status            WorkflowSyncStatus    `json:"status"`
	CanonicalRevision int64                 `json:"canonicalRevision"`
	Payload           json.RawMessage       `json:"payload,omitempty"`
	ReceivedAt        time.Time             `json:"receivedAt"`
}

func workflowSyncOwner(def workflow.WorkflowDefinition) string {
	if def.Source != "user" || def.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"]))
}

func (r *WorkflowDefinitionRepository) enqueueWorkflowSyncUpsertTx(ctx context.Context, tx *sql.Tx, def workflow.WorkflowDefinition, hash string, now time.Time) error {
	owner := workflowSyncOwner(def)
	if owner == "" || workflowSyncSuppressed(ctx) {
		return nil
	}
	payload, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshal workflow sync definition: %w", err)
	}
	return r.enqueueWorkflowSyncEventTx(ctx, tx, owner, def.ID, WorkflowSyncUpsert, hash, payload, false, now)
}

func (r *WorkflowDefinitionRepository) enqueueWorkflowSyncDeleteTx(ctx context.Context, tx *sql.Tx, owner, workflowID, hash string, now time.Time) error {
	owner = strings.TrimSpace(owner)
	workflowID = strings.TrimSpace(workflowID)
	if owner == "" || workflowID == "" || workflowSyncSuppressed(ctx) {
		return nil
	}
	payload, _ := json.Marshal(map[string]string{"workflowId": workflowID})
	return r.enqueueWorkflowSyncEventTx(ctx, tx, owner, workflowID, WorkflowSyncDelete, hash, payload, true, now)
}

func (r *WorkflowDefinitionRepository) enqueueWorkflowSyncEventTx(ctx context.Context, tx *sql.Tx, owner, workflowID string, eventType WorkflowSyncEventType, hash string, payload []byte, deleted bool, now time.Time) error {
	var currentRevision int64
	var currentHash string
	var currentDeleted int
	err := tx.QueryRowContext(ctx, `
		SELECT revision, definition_hash, deleted
		FROM extension_workflow_sync_state
		WHERE owner_user_id = ? AND workflow_id = ?
	`, owner, workflowID).Scan(&currentRevision, &currentHash, &currentDeleted)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read workflow sync state: %w", err)
	}
	if err == nil && currentHash == hash && (currentDeleted != 0) == deleted {
		return nil
	}
	baseRevision := currentRevision
	revision := baseRevision + 1
	eventID := "wfsync-" + uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO extension_workflow_sync_state
			(owner_user_id, workflow_id, revision, definition_hash, deleted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner_user_id, workflow_id) DO UPDATE SET
			revision = excluded.revision,
			definition_hash = excluded.definition_hash,
			deleted = excluded.deleted,
			updated_at = excluded.updated_at
	`, owner, workflowID, revision, hash, boolToInt(deleted), now); err != nil {
		return fmt.Errorf("advance workflow sync state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO extension_workflow_sync_outbox
			(event_id, owner_user_id, workflow_id, revision, base_revision, event_type, definition_hash,
			 payload_json, created_at, sent_at, acked_at, retry_count, next_retry_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, 0, ?)
	`, eventID, owner, workflowID, revision, baseRevision, string(eventType), hash, payload, now, now); err != nil {
		return fmt.Errorf("enqueue workflow sync outbox: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) ListWorkflowSyncOutbox(ctx context.Context, ownerUserID string, limit int) ([]WorkflowSyncOutboxEvent, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, errors.New("workflow sync owner is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	now := time.Now().UTC()
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_id, owner_user_id, workflow_id, revision, base_revision, event_type, definition_hash,
			payload_json, created_at, sent_at, acked_at, retry_count, next_retry_at
		FROM extension_workflow_sync_outbox
		WHERE owner_user_id = ? AND acked_at IS NULL AND next_retry_at <= ?
		ORDER BY created_at ASC, revision ASC, event_id ASC
		LIMIT ?
	`, ownerUserID, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow sync outbox: %w", err)
	}
	defer rows.Close()
	items := make([]WorkflowSyncOutboxEvent, 0)
	for rows.Next() {
		var item WorkflowSyncOutboxEvent
		var eventType string
		var payload string
		var sentAt, ackedAt sql.NullTime
		if err := rows.Scan(&item.EventID, &item.OwnerUserID, &item.WorkflowID, &item.Revision, &item.BaseRevision,
			&eventType, &item.DefinitionHash, &payload, &item.CreatedAt, &sentAt, &ackedAt, &item.RetryCount, &item.NextRetryAt); err != nil {
			return nil, fmt.Errorf("scan workflow sync outbox: %w", err)
		}
		item.EventType = WorkflowSyncEventType(eventType)
		item.Payload = json.RawMessage(payload)
		if sentAt.Valid {
			value := sentAt.Time
			item.SentAt = &value
		}
		if ackedAt.Valid {
			value := ackedAt.Time
			item.AckedAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowDefinitionRepository) MarkWorkflowSyncSent(ctx context.Context, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, eventID := range eventIDs {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			continue
		}
		_, err := r.db.ExecContext(ctx, `
			UPDATE extension_workflow_sync_outbox
			SET sent_at = ?, retry_count = retry_count + 1,
				next_retry_at = datetime(?, '+' || MIN(300, (1 << MIN(retry_count, 8))) || ' seconds')
			WHERE event_id = ? AND acked_at IS NULL
		`, now, now, eventID)
		if err != nil {
			return fmt.Errorf("mark workflow sync sent: %w", err)
		}
	}
	return nil
}

func (r *WorkflowDefinitionRepository) AckWorkflowSyncEvents(ctx context.Context, ownerUserID string, eventIDs []string) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return errors.New("workflow sync owner is required")
	}
	now := time.Now().UTC()
	for _, eventID := range eventIDs {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			continue
		}
		if _, err := r.db.ExecContext(ctx, `
			UPDATE extension_workflow_sync_outbox
			SET acked_at = ?, next_retry_at = ?
			WHERE owner_user_id = ? AND event_id = ?
		`, now, now, ownerUserID, eventID); err != nil {
			return fmt.Errorf("ack workflow sync event: %w", err)
		}
	}
	return nil
}

func (r *WorkflowDefinitionRepository) SetWorkflowSyncState(ctx context.Context, ownerUserID, workflowID string, revision int64, definitionHash string, deleted bool) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	workflowID = strings.TrimSpace(workflowID)
	if ownerUserID == "" || workflowID == "" || revision <= 0 {
		return errors.New("workflow sync state requires owner, workflow and positive revision")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_workflow_sync_state(owner_user_id, workflow_id, revision, definition_hash, deleted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner_user_id, workflow_id) DO UPDATE SET
			revision = excluded.revision,
			definition_hash = excluded.definition_hash,
			deleted = excluded.deleted,
			updated_at = excluded.updated_at
	`, ownerUserID, workflowID, revision, definitionHash, boolToInt(deleted), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set workflow sync state: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) ListWorkflowSyncStates(ctx context.Context, ownerUserID string) ([]WorkflowSyncState, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT owner_user_id, workflow_id, revision, definition_hash, deleted, updated_at
		FROM extension_workflow_sync_state
		WHERE owner_user_id = ?
		ORDER BY workflow_id
	`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WorkflowSyncState, 0)
	for rows.Next() {
		var item WorkflowSyncState
		var deleted int
		if err := rows.Scan(&item.OwnerUserID, &item.WorkflowID, &item.Revision, &item.DefinitionHash, &deleted, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Deleted = deleted != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

// ApplyWorkflowSyncEvent implements the Cloud inbox + canonical revision gate.
// Inbox insertion and canonical advancement share one transaction, so a crash
// cannot acknowledge an event without also committing its conflict/merge state.
func (r *WorkflowDefinitionRepository) ApplyWorkflowSyncEvent(ctx context.Context, sourceDeviceID string, event WorkflowSyncOutboxEvent) (WorkflowSyncApplyResult, error) {
	result := WorkflowSyncApplyResult{EventID: event.EventID, WorkflowID: event.WorkflowID}
	if strings.TrimSpace(sourceDeviceID) == "" || strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.OwnerUserID) == "" || strings.TrimSpace(event.WorkflowID) == "" {
		return result, errors.New("workflow sync inbox requires device, event, owner and workflow")
	}
	if event.Revision <= 0 || event.BaseRevision < 0 || event.Revision <= event.BaseRevision {
		return result, errors.New("workflow sync revision is invalid")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingStatus string
	var existingAccepted int
	var existingCanonical int64
	err = tx.QueryRowContext(ctx, `
		SELECT status, accepted, canonical_revision
		FROM extension_workflow_sync_inbox
		WHERE event_id = ? AND source_device_id = ?
	`, event.EventID, sourceDeviceID).Scan(&existingStatus, &existingAccepted, &existingCanonical)
	if err == nil {
		result.Status = WorkflowSyncStatus(existingStatus)
		result.Accepted = existingAccepted != 0
		result.CanonicalRevision = existingCanonical
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}

	var currentRevision int64
	var currentHash string
	var currentDeleted int
	canonicalErr := tx.QueryRowContext(ctx, `
		SELECT revision, definition_hash, deleted
		FROM extension_workflow_sync_canonical
		WHERE owner_user_id = ? AND workflow_id = ?
	`, event.OwnerUserID, event.WorkflowID).Scan(&currentRevision, &currentHash, &currentDeleted)
	canonicalExists := canonicalErr == nil
	if canonicalErr != nil && !errors.Is(canonicalErr, sql.ErrNoRows) {
		return result, canonicalErr
	}

	status := WorkflowSyncFastForward
	accepted := false
	switch {
	case !canonicalExists:
		accepted = true
		if event.BaseRevision > 0 {
			status = WorkflowSyncDeviceAhead
		}
	case event.Revision == currentRevision && event.DefinitionHash == currentHash:
		status = WorkflowSyncDuplicate
	case event.Revision < currentRevision:
		status = WorkflowSyncCloudAhead
	case event.Revision == currentRevision:
		status = WorkflowSyncDiverged
	case event.BaseRevision == currentRevision:
		accepted = true
		status = WorkflowSyncFastForward
	case event.BaseRevision < currentRevision:
		status = WorkflowSyncConflict
	case event.BaseRevision > currentRevision:
		status = WorkflowSyncDeviceAhead
	}

	canonicalRevision := currentRevision
	canonicalHash := currentHash
	if accepted {
		deleted := event.EventType == WorkflowSyncDelete
		canonicalRevision = event.Revision
		canonicalHash = event.DefinitionHash
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO extension_workflow_sync_canonical
				(owner_user_id, workflow_id, revision, definition_hash, deleted, source_device_id, payload_json, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(owner_user_id, workflow_id) DO UPDATE SET
				revision = excluded.revision,
				definition_hash = excluded.definition_hash,
				deleted = excluded.deleted,
				source_device_id = excluded.source_device_id,
				payload_json = excluded.payload_json,
				updated_at = excluded.updated_at
		`, event.OwnerUserID, event.WorkflowID, event.Revision, event.DefinitionHash, boolToInt(deleted), sourceDeviceID, event.Payload, time.Now().UTC()); err != nil {
			return result, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO extension_workflow_sync_inbox
			(event_id, source_device_id, owner_user_id, workflow_id, revision, base_revision, event_type,
			 definition_hash, payload_json, status, accepted, canonical_revision, received_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.EventID, sourceDeviceID, event.OwnerUserID, event.WorkflowID, event.Revision, event.BaseRevision,
		string(event.EventType), event.DefinitionHash, event.Payload, string(status), boolToInt(accepted), canonicalRevision, time.Now().UTC()); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	result.Status = status
	result.Accepted = accepted
	result.CanonicalRevision = canonicalRevision
	result.CanonicalHash = canonicalHash
	return result, nil
}

func (r *WorkflowDefinitionRepository) ListWorkflowSyncConflicts(ctx context.Context, ownerUserID string, limit int) ([]WorkflowSyncConflict, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, errors.New("workflow sync owner is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_id, source_device_id, owner_user_id, workflow_id, revision, base_revision,
			event_type, definition_hash, status, canonical_revision, payload_json, received_at
		FROM extension_workflow_sync_inbox
		WHERE owner_user_id = ? AND accepted = 0 AND status IN (?, ?, ?)
		ORDER BY received_at DESC
		LIMIT ?
	`, ownerUserID, string(WorkflowSyncConflict), string(WorkflowSyncDiverged), string(WorkflowSyncCloudAhead), limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow sync conflicts: %w", err)
	}
	defer rows.Close()
	items := make([]WorkflowSyncConflict, 0)
	for rows.Next() {
		var item WorkflowSyncConflict
		var eventType, status, payload string
		if err := rows.Scan(&item.EventID, &item.SourceDeviceID, &item.OwnerUserID, &item.WorkflowID,
			&item.Revision, &item.BaseRevision, &eventType, &item.DefinitionHash, &status,
			&item.CanonicalRevision, &payload, &item.ReceivedAt); err != nil {
			return nil, err
		}
		item.EventType = WorkflowSyncEventType(eventType)
		item.Status = WorkflowSyncStatus(status)
		item.Payload = json.RawMessage(payload)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *WorkflowDefinitionRepository) ListWorkflowSyncCanonical(ctx context.Context, ownerUserID string) ([]WorkflowSyncOutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT workflow_id, revision, definition_hash, deleted, source_device_id, payload_json, updated_at
		FROM extension_workflow_sync_canonical
		WHERE owner_user_id = ?
		ORDER BY workflow_id
	`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WorkflowSyncOutboxEvent, 0)
	for rows.Next() {
		var item WorkflowSyncOutboxEvent
		var deleted int
		var sourceDeviceID string
		var payload string
		if err := rows.Scan(&item.WorkflowID, &item.Revision, &item.DefinitionHash, &deleted, &sourceDeviceID, &payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.OwnerUserID = strings.TrimSpace(ownerUserID)
		item.BaseRevision = item.Revision - 1
		if deleted != 0 {
			item.EventType = WorkflowSyncDelete
		} else {
			item.EventType = WorkflowSyncUpsert
		}
		item.Payload = json.RawMessage(payload)
		_ = sourceDeviceID
		items = append(items, item)
	}
	return items, rows.Err()
}
