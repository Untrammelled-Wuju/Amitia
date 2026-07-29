package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type OutboxStatus string

const (
	OutboxStatusPending     OutboxStatus = "pending"
	OutboxStatusDispatching OutboxStatus = "dispatching"
	OutboxStatusDispatched  OutboxStatus = "dispatched"
	OutboxStatusFailed      OutboxStatus = "failed"
	OutboxStatusDeadLetter  OutboxStatus = "dead_letter"
	OutboxStatusCancelled   OutboxStatus = "cancelled"
)

type OutboxRecord struct {
	OutboxID             string
	EventID              string
	EventTypeID          EventTypeID
	EventVersion         int
	ProducerID           string
	ProducerType         string
	ProducerGeneration   int64
	AggregateType        string
	AggregateID          string
	AggregateVersion     *int64
	PartitionKey         string
	OrderingKey          string
	IdempotencyKey       string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	TraceID              string
	OperationID          string
	ParentEventID        *string
	Depth                int
	OccurredAt           time.Time
	PublishedAt          *time.Time
	Payload              json.RawMessage
	Metadata             json.RawMessage
	PayloadHash          string
	DefinitionHash       string
	Status               OutboxStatus
	AvailableAt          time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ErrorCode            string
	ErrorMessage         string
	LeaseOwner           string
	LeaseExpiresAt       *time.Time
	DispatchedAt         *time.Time
}

type OutboxStore interface {
	EnqueueTx(ctx context.Context, tx OutboxTx, record OutboxRecord) error
	Enqueue(ctx context.Context, record OutboxRecord) error
	ClaimNext(ctx context.Context, owner string, leaseTTL time.Duration, limit int) ([]OutboxRecord, error)
	RenewLease(ctx context.Context, outboxID, owner string, leaseTTL time.Duration) error
	ReleaseLease(ctx context.Context, outboxID string) error
	ReleaseExpiredLeases(ctx context.Context) (int, error)
	MarkDispatched(ctx context.Context, outboxID string) error
	MarkFailed(ctx context.Context, outboxID, code, message string) error
	MarkDeadLetter(ctx context.Context, outboxID, code, message string) error
	MarkCancelled(ctx context.Context, outboxID, reason string) error
	Get(ctx context.Context, outboxID string) (OutboxRecord, error)
	GetByEventID(ctx context.Context, eventID string) (OutboxRecord, error)
	ListPending(ctx context.Context, limit int) ([]OutboxRecord, error)
	ListByStatus(ctx context.Context, status OutboxStatus, limit, offset int) ([]OutboxRecord, error)
	ListByExtension(ctx context.Context, extensionID string, limit, offset int) ([]OutboxRecord, error)
	CountByStatus(ctx context.Context, status OutboxStatus) (int, error)
	DeleteOlderThan(ctx context.Context, before time.Time, status OutboxStatus) (int, error)
}

type OutboxTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type OutboxRepository struct {
	db *sql.DB
	mu sync.Mutex
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) EnqueueTx(ctx context.Context, tx OutboxTx, record OutboxRecord) error {
	if record.OutboxID == "" {
		return errors.New("event: outbox id required")
	}
	if record.EventID == "" {
		return errors.New("event: event id required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = record.CreatedAt
	if record.Status == "" {
		record.Status = OutboxStatusPending
	}
	if record.AvailableAt.IsZero() {
		record.AvailableAt = record.CreatedAt
	}
	aggVersion := sql.NullInt64{}
	if record.AggregateVersion != nil {
		aggVersion = sql.NullInt64{Int64: *record.AggregateVersion, Valid: true}
	}
	parentID := sql.NullString{}
	if record.ParentEventID != nil {
		parentID = sql.NullString{String: *record.ParentEventID, Valid: true}
	}
	publishedAt := sql.NullTime{}
	if record.PublishedAt != nil {
		publishedAt = sql.NullTime{Time: *record.PublishedAt, Valid: true}
	}
	leaseExpires := sql.NullTime{}
	if record.LeaseExpiresAt != nil {
		leaseExpires = sql.NullTime{Time: *record.LeaseExpiresAt, Valid: true}
	}
	dispatchedAt := sql.NullTime{}
	if record.DispatchedAt != nil {
		dispatchedAt = sql.NullTime{Time: *record.DispatchedAt, Valid: true}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO extension_event_outbox
		(outbox_id, event_id, event_type_id, event_version, producer_id, producer_type, producer_generation,
		 aggregate_type, aggregate_id, aggregate_version, partition_key, ordering_key, idempotency_key,
		 scope_snapshot_id, permission_snapshot_id, trace_id, operation_id, parent_event_id, depth,
		 occurred_at, published_at, payload_json, metadata_json, payload_hash, definition_hash,
		 status, available_at, created_at, updated_at, error_code, error_message, lease_owner, lease_expires_at, dispatched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(idempotency_key) DO NOTHING
	`,
		record.OutboxID, record.EventID, string(record.EventTypeID), record.EventVersion,
		record.ProducerID, record.ProducerType, record.ProducerGeneration,
		record.AggregateType, record.AggregateID, aggVersion,
		record.PartitionKey, record.OrderingKey, record.IdempotencyKey,
		record.ScopeSnapshotID, record.PermissionSnapshotID, record.TraceID, record.OperationID, parentID, record.Depth,
		record.OccurredAt, publishedAt, string(record.Payload), string(record.Metadata),
		record.PayloadHash, record.DefinitionHash,
		string(record.Status), record.AvailableAt, record.CreatedAt, record.UpdatedAt,
		record.ErrorCode, record.ErrorMessage, record.LeaseOwner, leaseExpires, dispatchedAt,
	)
	return err
}

func (r *OutboxRepository) Enqueue(ctx context.Context, record OutboxRecord) error {
	return r.EnqueueTx(ctx, r.db, record)
}

func (r *OutboxRepository) ClaimNext(ctx context.Context, owner string, leaseTTL time.Duration, limit int) ([]OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	expires := now.Add(leaseTTL)
	rows, err := tx.QueryContext(ctx, `
		SELECT outbox_id, event_id, event_type_id, event_version, producer_id, producer_type, producer_generation,
		 aggregate_type, aggregate_id, aggregate_version, partition_key, ordering_key, idempotency_key,
		 scope_snapshot_id, permission_snapshot_id, trace_id, operation_id, parent_event_id, depth,
		 occurred_at, published_at, payload_json, metadata_json, payload_hash, definition_hash,
		 status, available_at, created_at, updated_at, error_code, error_message, lease_owner, lease_expires_at, dispatched_at
		FROM extension_event_outbox
		WHERE status = ? AND (lease_expires_at IS NULL OR lease_expires_at < ?)
		ORDER BY available_at ASC, created_at ASC
		LIMIT ?
	`, string(OutboxStatusPending), now, limit)
	if err != nil {
		return nil, err
	}
	var records []OutboxRecord
	var ids []string
	for rows.Next() {
		rec, err := scanOutboxRecord(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, rec.OutboxID)
		records = append(records, rec)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE extension_event_outbox
			SET status = ?, lease_owner = ?, lease_expires_at = ?, updated_at = ?
			WHERE outbox_id = ? AND status = ?
		`, string(OutboxStatusDispatching), owner, expires, now, id, string(OutboxStatusPending)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for i := range records {
		records[i].Status = OutboxStatusDispatching
		records[i].LeaseOwner = owner
		records[i].LeaseExpiresAt = &expires
	}
	return records, nil
}

func (r *OutboxRepository) RenewLease(ctx context.Context, outboxID, owner string, leaseTTL time.Duration) error {
	now := time.Now().UTC()
	expires := now.Add(leaseTTL)
	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET lease_expires_at = ?, updated_at = ?
		WHERE outbox_id = ? AND lease_owner = ?
	`, expires, now, outboxID, owner)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrLeaseExpired
	}
	return nil
}

func (r *OutboxRepository) ReleaseLease(ctx context.Context, outboxID string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET status = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE outbox_id = ?
	`, string(OutboxStatusPending), now, outboxID)
	return err
}

func (r *OutboxRepository) ReleaseExpiredLeases(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET status = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ?
	`, string(OutboxStatusPending), now, string(OutboxStatusDispatching), now)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func (r *OutboxRepository) MarkDispatched(ctx context.Context, outboxID string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET status = ?, dispatched_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE outbox_id = ?
	`, string(OutboxStatusDispatched), now, now, outboxID)
	return err
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, outboxID, code, message string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET status = ?, error_code = ?, error_message = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE outbox_id = ?
	`, string(OutboxStatusPending), code, message, now, outboxID)
	return err
}

func (r *OutboxRepository) MarkDeadLetter(ctx context.Context, outboxID, code, message string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET status = ?, error_code = ?, error_message = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE outbox_id = ?
	`, string(OutboxStatusDeadLetter), code, message, now, outboxID)
	return err
}

func (r *OutboxRepository) MarkCancelled(ctx context.Context, outboxID, reason string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_outbox
		SET status = ?, error_message = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE outbox_id = ?
	`, string(OutboxStatusCancelled), reason, now, outboxID)
	return err
}

func (r *OutboxRepository) Get(ctx context.Context, outboxID string) (OutboxRecord, error) {
	row := r.db.QueryRowContext(ctx, outboxSelectQuery+" WHERE outbox_id = ?", outboxID)
	return scanOutboxRecordRow(row)
}

func (r *OutboxRepository) GetByEventID(ctx context.Context, eventID string) (OutboxRecord, error) {
	row := r.db.QueryRowContext(ctx, outboxSelectQuery+" WHERE event_id = ?", eventID)
	return scanOutboxRecordRow(row)
}

func (r *OutboxRepository) ListPending(ctx context.Context, limit int) ([]OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, outboxSelectQuery+" WHERE status = ? ORDER BY available_at ASC LIMIT ?", string(OutboxStatusPending), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOutboxRecords(rows)
}

func (r *OutboxRepository) ListByStatus(ctx context.Context, status OutboxStatus, limit, offset int) ([]OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, outboxSelectQuery+" WHERE status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", string(status), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOutboxRecords(rows)
}

func (r *OutboxRepository) ListByExtension(ctx context.Context, extensionID string, limit, offset int) ([]OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, outboxSelectQuery+" WHERE producer_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", extensionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOutboxRecords(rows)
}

func (r *OutboxRepository) CountByStatus(ctx context.Context, status OutboxStatus) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_event_outbox WHERE status = ?`, string(status)).Scan(&count)
	return count, err
}

func (r *OutboxRepository) DeleteOlderThan(ctx context.Context, before time.Time, status OutboxStatus) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM extension_event_outbox WHERE status = ? AND created_at < ?`, string(status), before)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

const outboxSelectQuery = `
SELECT outbox_id, event_id, event_type_id, event_version, producer_id, producer_type, producer_generation,
 aggregate_type, aggregate_id, aggregate_version, partition_key, ordering_key, idempotency_key,
 scope_snapshot_id, permission_snapshot_id, trace_id, operation_id, parent_event_id, depth,
 occurred_at, published_at, payload_json, metadata_json, payload_hash, definition_hash,
 status, available_at, created_at, updated_at, error_code, error_message, lease_owner, lease_expires_at, dispatched_at
FROM extension_event_outbox
`

func scanOutboxRecord(rows *sql.Rows) (OutboxRecord, error) {
	var rec OutboxRecord
	var aggVersion sql.NullInt64
	var parentID sql.NullString
	var publishedAt sql.NullTime
	var leaseExpires sql.NullTime
	var dispatchedAt sql.NullTime
	var payload, metadata, defHash string
	var status string
	var eventTypeID string
	var aggregateType, aggregateID, partitionKey, orderingKey sql.NullString
	var scopeSnapshotID, permissionSnapshotID, traceID, operationID sql.NullString
	var errorCode, errorMessage, leaseOwner sql.NullString
	var err error
	err = rows.Scan(
		&rec.OutboxID, &rec.EventID, &eventTypeID, &rec.EventVersion, &rec.ProducerID, &rec.ProducerType, &rec.ProducerGeneration,
		&aggregateType, &aggregateID, &aggVersion, &partitionKey, &orderingKey, &rec.IdempotencyKey,
		&scopeSnapshotID, &permissionSnapshotID, &traceID, &operationID, &parentID, &rec.Depth,
		&rec.OccurredAt, &publishedAt, &payload, &metadata, &rec.PayloadHash, &defHash,
		&status, &rec.AvailableAt, &rec.CreatedAt, &rec.UpdatedAt, &errorCode, &errorMessage, &leaseOwner, &leaseExpires, &dispatchedAt,
	)
	if err != nil {
		return rec, err
	}
	rec.EventTypeID = EventTypeID(eventTypeID)
	rec.Payload = json.RawMessage(payload)
	rec.Metadata = json.RawMessage(metadata)
	rec.DefinitionHash = defHash
	rec.Status = OutboxStatus(status)
	rec.AggregateType = aggregateType.String
	rec.AggregateID = aggregateID.String
	rec.PartitionKey = partitionKey.String
	rec.OrderingKey = orderingKey.String
	rec.ScopeSnapshotID = scopeSnapshotID.String
	rec.PermissionSnapshotID = permissionSnapshotID.String
	rec.TraceID = traceID.String
	rec.OperationID = operationID.String
	rec.ErrorCode = errorCode.String
	rec.ErrorMessage = errorMessage.String
	rec.LeaseOwner = leaseOwner.String
	if aggVersion.Valid {
		v := aggVersion.Int64
		rec.AggregateVersion = &v
	}
	if parentID.Valid {
		s := parentID.String
		rec.ParentEventID = &s
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		rec.PublishedAt = &t
	}
	if leaseExpires.Valid {
		t := leaseExpires.Time
		rec.LeaseExpiresAt = &t
	}
	if dispatchedAt.Valid {
		t := dispatchedAt.Time
		rec.DispatchedAt = &t
	}
	return rec, nil
}

func scanOutboxRecordRow(row *sql.Row) (OutboxRecord, error) {
	var rec OutboxRecord
	var aggVersion sql.NullInt64
	var parentID sql.NullString
	var publishedAt sql.NullTime
	var leaseExpires sql.NullTime
	var dispatchedAt sql.NullTime
	var payload, metadata, defHash string
	var status string
	var eventTypeID string
	var aggregateType, aggregateID, partitionKey, orderingKey sql.NullString
	var scopeSnapshotID, permissionSnapshotID, traceID, operationID sql.NullString
	var errorCode, errorMessage, leaseOwner sql.NullString
	err := row.Scan(
		&rec.OutboxID, &rec.EventID, &eventTypeID, &rec.EventVersion, &rec.ProducerID, &rec.ProducerType, &rec.ProducerGeneration,
		&aggregateType, &aggregateID, &aggVersion, &partitionKey, &orderingKey, &rec.IdempotencyKey,
		&scopeSnapshotID, &permissionSnapshotID, &traceID, &operationID, &parentID, &rec.Depth,
		&rec.OccurredAt, &publishedAt, &payload, &metadata, &rec.PayloadHash, &defHash,
		&status, &rec.AvailableAt, &rec.CreatedAt, &rec.UpdatedAt, &errorCode, &errorMessage, &leaseOwner, &leaseExpires, &dispatchedAt,
	)
	if err != nil {
		return rec, fmt.Errorf("%w: %v", ErrDeliveryNotFound, err)
	}
	rec.EventTypeID = EventTypeID(eventTypeID)
	rec.Payload = json.RawMessage(payload)
	rec.Metadata = json.RawMessage(metadata)
	rec.DefinitionHash = defHash
	rec.Status = OutboxStatus(status)
	rec.AggregateType = aggregateType.String
	rec.AggregateID = aggregateID.String
	rec.PartitionKey = partitionKey.String
	rec.OrderingKey = orderingKey.String
	rec.ScopeSnapshotID = scopeSnapshotID.String
	rec.PermissionSnapshotID = permissionSnapshotID.String
	rec.TraceID = traceID.String
	rec.OperationID = operationID.String
	rec.ErrorCode = errorCode.String
	rec.ErrorMessage = errorMessage.String
	rec.LeaseOwner = leaseOwner.String
	if aggVersion.Valid {
		v := aggVersion.Int64
		rec.AggregateVersion = &v
	}
	if parentID.Valid {
		s := parentID.String
		rec.ParentEventID = &s
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		rec.PublishedAt = &t
	}
	if leaseExpires.Valid {
		t := leaseExpires.Time
		rec.LeaseExpiresAt = &t
	}
	if dispatchedAt.Valid {
		t := dispatchedAt.Time
		rec.DispatchedAt = &t
	}
	return rec, nil
}

func scanOutboxRecords(rows *sql.Rows) ([]OutboxRecord, error) {
	var records []OutboxRecord
	for rows.Next() {
		rec, err := scanOutboxRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
