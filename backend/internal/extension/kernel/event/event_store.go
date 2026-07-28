package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type SQLiteDeliveryStore struct {
	db *sql.DB
}

func NewSQLiteDeliveryStore(db *sql.DB) *SQLiteDeliveryStore {
	return &SQLiteDeliveryStore{db: db}
}

func (s *SQLiteDeliveryStore) CreateDeliveryTx(ctx context.Context, tx *sql.Tx, delivery Delivery) error {
	return s.createDeliveryWithExec(ctx, tx, delivery)
}

func (s *SQLiteDeliveryStore) CreateDelivery(ctx context.Context, delivery Delivery) error {
	return s.createDeliveryWithExec(ctx, s.db, delivery)
}

func (s *SQLiteDeliveryStore) createDeliveryWithExec(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, delivery Delivery) error {
	now := time.Now().UTC()
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = now
	}
	delivery.UpdatedAt = now
	if delivery.Status == "" {
		delivery.Status = DeliveryStatusPending
	}
	if delivery.AvailableAt.IsZero() {
		delivery.AvailableAt = now
	}
	var leaseExpires sql.NullTime
	if delivery.LeaseExpiresAt != nil {
		leaseExpires = sql.NullTime{Time: *delivery.LeaseExpiresAt, Valid: true}
	}
	var startedAt sql.NullTime
	if delivery.StartedAt != nil {
		startedAt = sql.NullTime{Time: *delivery.StartedAt, Valid: true}
	}
	var finishedAt sql.NullTime
	if delivery.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: *delivery.FinishedAt, Valid: true}
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO extension_event_deliveries
		(delivery_id, event_id, subscription_id, extension_id, module_id, status,
		 partition_key, ordering_key, sequence, attempt, max_attempts, available_at,
		 lease_owner, lease_expires_at, runtime_instance_id, scope_snapshot_id, permission_snapshot_id,
		 projected_payload_hash, subscription_generation, target_generation,
		 started_at, finished_at, error_code, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(delivery_id) DO NOTHING
	`,
		delivery.DeliveryID, delivery.EventID, delivery.SubscriptionID, delivery.ExtensionID, delivery.ModuleID, string(delivery.Status),
		delivery.PartitionKey, delivery.OrderingKey, delivery.Sequence, delivery.Attempt, delivery.MaxAttempts, delivery.AvailableAt,
		delivery.LeaseOwner, leaseExpires, delivery.RuntimeInstanceID, delivery.ScopeSnapshotID, delivery.PermissionSnapshotID,
		delivery.ProjectedPayloadHash, delivery.SubscriptionGeneration, delivery.TargetGeneration,
		startedAt, finishedAt, delivery.ErrorCode, delivery.ErrorMessage, delivery.CreatedAt, delivery.UpdatedAt,
	)
	return err
}

func (s *SQLiteDeliveryStore) ClaimNextDeliveries(ctx context.Context, owner string, leaseTTL time.Duration, limit int) ([]Delivery, error) {
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	expires := now.Add(leaseTTL)
	rows, err := tx.QueryContext(ctx, `
		SELECT delivery_id, event_id, subscription_id, extension_id, module_id, status,
		 partition_key, ordering_key, sequence, attempt, max_attempts, available_at,
		 lease_owner, lease_expires_at, runtime_instance_id, scope_snapshot_id, permission_snapshot_id,
		 projected_payload_hash, subscription_generation, target_generation,
		 started_at, finished_at, error_code, error_message, created_at, updated_at
		FROM extension_event_deliveries
		WHERE status IN (?, ?) AND (lease_expires_at IS NULL OR lease_expires_at < ?)
		ORDER BY available_at ASC, sequence ASC, created_at ASC
		LIMIT ?
	`, string(DeliveryStatusPending), string(DeliveryStatusRetryWait), now, limit)
	if err != nil {
		return nil, err
	}
	var deliveries []Delivery
	var ids []string
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, d.DeliveryID)
		deliveries = append(deliveries, d)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE extension_event_deliveries
			SET status = ?, lease_owner = ?, lease_expires_at = ?, updated_at = ?
			WHERE delivery_id = ? AND status IN (?, ?)
		`, string(DeliveryStatusLeased), owner, expires, now, id, string(DeliveryStatusPending), string(DeliveryStatusRetryWait)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for i := range deliveries {
		deliveries[i].Status = DeliveryStatusLeased
		deliveries[i].LeaseOwner = owner
		deliveries[i].LeaseExpiresAt = &expires
	}
	return deliveries, nil
}

func (s *SQLiteDeliveryStore) RenewDeliveryLease(ctx context.Context, deliveryID, owner string, leaseTTL time.Duration) error {
	now := time.Now().UTC()
	expires := now.Add(leaseTTL)
	res, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_deliveries
		SET lease_expires_at = ?, updated_at = ?
		WHERE delivery_id = ? AND lease_owner = ?
	`, expires, now, deliveryID, owner)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrLeaseExpired
	}
	return nil
}

func (s *SQLiteDeliveryStore) ReleaseDeliveryLease(ctx context.Context, deliveryID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_deliveries
		SET status = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE delivery_id = ? AND status IN (?, ?)
	`, string(DeliveryStatusPending), now, deliveryID, string(DeliveryStatusLeased), string(DeliveryStatusDelivering))
	return err
}

func (s *SQLiteDeliveryStore) ReleaseExpiredDeliveryLeases(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_deliveries
		SET status = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE status IN (?, ?) AND lease_expires_at IS NOT NULL AND lease_expires_at < ?
	`, string(DeliveryStatusPending), now, string(DeliveryStatusLeased), string(DeliveryStatusDelivering), now)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func (s *SQLiteDeliveryStore) UpdateDeliveryStatus(ctx context.Context, deliveryID string, status DeliveryStatus, errorCode, errorMessage string) error {
	now := time.Now().UTC()
	var finishedAt interface{}
	if status == DeliveryStatusSucceeded || status == DeliveryStatusDeadLetter || status == DeliveryStatusCancelled || status == DeliveryStatusSkipped || status == DeliveryStatusFailed {
		finishedAt = now
	} else {
		finishedAt = nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_deliveries
		SET status = ?, error_code = ?, error_message = ?, finished_at = COALESCE(?, finished_at), lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE delivery_id = ?
	`, string(status), errorCode, errorMessage, finishedAt, now, deliveryID)
	return err
}

func (s *SQLiteDeliveryStore) GetDelivery(ctx context.Context, deliveryID string) (Delivery, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT delivery_id, event_id, subscription_id, extension_id, module_id, status,
		 partition_key, ordering_key, sequence, attempt, max_attempts, available_at,
		 lease_owner, lease_expires_at, runtime_instance_id, scope_snapshot_id, permission_snapshot_id,
		 projected_payload_hash, subscription_generation, target_generation,
		 started_at, finished_at, error_code, error_message, created_at, updated_at
		FROM extension_event_deliveries
		WHERE delivery_id = ?
	`, deliveryID)
	return scanDeliveryRow(row)
}

func (s *SQLiteDeliveryStore) ListDeliveries(ctx context.Context, filter DeliveryFilter, limit, offset int) ([]Delivery, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT delivery_id, event_id, subscription_id, extension_id, module_id, status,
		 partition_key, ordering_key, sequence, attempt, max_attempts, available_at,
		 lease_owner, lease_expires_at, runtime_instance_id, scope_snapshot_id, permission_snapshot_id,
		 projected_payload_hash, subscription_generation, target_generation,
		 started_at, finished_at, error_code, error_message, created_at, updated_at
		FROM extension_event_deliveries
		WHERE 1=1
	`
	args := []interface{}{}
	if filter.ExtensionID != "" {
		query += " AND extension_id = ?"
		args = append(args, filter.ExtensionID)
	}
	if filter.SubscriptionID != "" {
		query += " AND subscription_id = ?"
		args = append(args, filter.SubscriptionID)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	if filter.EventID != "" {
		query += " AND event_id = ?"
		args = append(args, filter.EventID)
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

func (s *SQLiteDeliveryStore) ListDeliveriesByEvent(ctx context.Context, eventID string) ([]Delivery, error) {
	return s.ListDeliveries(ctx, DeliveryFilter{EventID: eventID}, 100, 0)
}

func (s *SQLiteDeliveryStore) ListDeliveriesBySubscription(ctx context.Context, subscriptionID string, limit, offset int) ([]Delivery, error) {
	return s.ListDeliveries(ctx, DeliveryFilter{SubscriptionID: subscriptionID}, limit, offset)
}

func (s *SQLiteDeliveryStore) CountDeliveriesByStatus(ctx context.Context, status DeliveryStatus) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_event_deliveries WHERE status = ?`, string(status)).Scan(&count)
	return count, err
}

func (s *SQLiteDeliveryStore) CancelPendingByExtension(ctx context.Context, extensionID, reason string) (int, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_deliveries
		SET status = ?, error_code = ?, error_message = ?, lease_owner = NULL, lease_expires_at = NULL, finished_at = ?, updated_at = ?
		WHERE extension_id = ? AND status IN (?, ?, ?)
	`, string(DeliveryStatusCancelled), reason, reason, now, now, extensionID,
		string(DeliveryStatusPending), string(DeliveryStatusLeased), string(DeliveryStatusRetryWait))
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func (s *SQLiteDeliveryStore) CancelPendingBySubscription(ctx context.Context, subscriptionID, reason string) (int, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_deliveries
		SET status = ?, error_code = ?, error_message = ?, lease_owner = NULL, lease_expires_at = NULL, finished_at = ?, updated_at = ?
		WHERE subscription_id = ? AND status IN (?, ?, ?)
	`, string(DeliveryStatusCancelled), reason, reason, now, now, subscriptionID,
		string(DeliveryStatusPending), string(DeliveryStatusLeased), string(DeliveryStatusRetryWait))
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func scanDelivery(rows *sql.Rows) (Delivery, error) {
	var d Delivery
	var status string
	var leaseExpires sql.NullTime
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var partitionKey, orderingKey, leaseOwner sql.NullString
	var runtimeInstanceID, scopeSnapshotID, permissionSnapshotID sql.NullString
	var projectedPayloadHash, errorCode, errorMessage sql.NullString
	err := rows.Scan(
		&d.DeliveryID, &d.EventID, &d.SubscriptionID, &d.ExtensionID, &d.ModuleID, &status,
		&partitionKey, &orderingKey, &d.Sequence, &d.Attempt, &d.MaxAttempts, &d.AvailableAt,
		&leaseOwner, &leaseExpires, &runtimeInstanceID, &scopeSnapshotID, &permissionSnapshotID,
		&projectedPayloadHash, &d.SubscriptionGeneration, &d.TargetGeneration, &startedAt, &finishedAt, &errorCode, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return d, err
	}
	d.Status = DeliveryStatus(status)
	d.PartitionKey = partitionKey.String
	d.OrderingKey = orderingKey.String
	d.LeaseOwner = leaseOwner.String
	d.RuntimeInstanceID = runtimeInstanceID.String
	d.ScopeSnapshotID = scopeSnapshotID.String
	d.PermissionSnapshotID = permissionSnapshotID.String
	d.ProjectedPayloadHash = projectedPayloadHash.String
	d.ErrorCode = errorCode.String
	d.ErrorMessage = errorMessage.String
	if leaseExpires.Valid {
		t := leaseExpires.Time
		d.LeaseExpiresAt = &t
	}
	if startedAt.Valid {
		t := startedAt.Time
		d.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		d.FinishedAt = &t
	}
	return d, nil
}

func scanDeliveryRow(row *sql.Row) (Delivery, error) {
	var d Delivery
	var status string
	var leaseExpires sql.NullTime
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var partitionKey, orderingKey, leaseOwner sql.NullString
	var runtimeInstanceID, scopeSnapshotID, permissionSnapshotID sql.NullString
	var projectedPayloadHash, errorCode, errorMessage sql.NullString
	err := row.Scan(
		&d.DeliveryID, &d.EventID, &d.SubscriptionID, &d.ExtensionID, &d.ModuleID, &status,
		&partitionKey, &orderingKey, &d.Sequence, &d.Attempt, &d.MaxAttempts, &d.AvailableAt,
		&leaseOwner, &leaseExpires, &runtimeInstanceID, &scopeSnapshotID, &permissionSnapshotID,
		&projectedPayloadHash, &d.SubscriptionGeneration, &d.TargetGeneration, &startedAt, &finishedAt, &errorCode, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return d, fmt.Errorf("%w: %v", ErrDeliveryNotFound, err)
	}
	d.Status = DeliveryStatus(status)
	d.PartitionKey = partitionKey.String
	d.OrderingKey = orderingKey.String
	d.LeaseOwner = leaseOwner.String
	d.RuntimeInstanceID = runtimeInstanceID.String
	d.ScopeSnapshotID = scopeSnapshotID.String
	d.PermissionSnapshotID = permissionSnapshotID.String
	d.ProjectedPayloadHash = projectedPayloadHash.String
	d.ErrorCode = errorCode.String
	d.ErrorMessage = errorMessage.String
	if leaseExpires.Valid {
		t := leaseExpires.Time
		d.LeaseExpiresAt = &t
	}
	if startedAt.Valid {
		t := startedAt.Time
		d.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		d.FinishedAt = &t
	}
	return d, nil
}

func scanDeliveries(rows *sql.Rows) ([]Delivery, error) {
	var deliveries []Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

type SQLiteDeadLetterStore struct {
	db *sql.DB
}

func NewSQLiteDeadLetterStore(db *sql.DB) *SQLiteDeadLetterStore {
	return &SQLiteDeadLetterStore{db: db}
}

func (s *SQLiteDeadLetterStore) CreateDeadLetter(ctx context.Context, record DeadLetterRecord) error {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	originEvent, _ := json.Marshal(record.OriginEvent)
	subSnapshot, _ := json.Marshal(record.SubscriptionSnapshot)
	var lastReplayAt sql.NullTime
	if record.LastReplayAt != nil {
		lastReplayAt = sql.NullTime{Time: *record.LastReplayAt, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO extension_event_dead_letters
		(dead_letter_id, event_id, delivery_id, subscription_id, extension_id, module_id,
		 event_type_id, event_version, reason, error_code, error_message, attempts,
		 partition_key, ordering_key, payload_hash, projected_payload_hash, definition_hash,
		 scope_snapshot_id, permission_snapshot_id, runtime_instance_id, trace_id, operation_id,
		 origin_event_json, subscription_snapshot_json, created_at, replay_count, last_replay_at, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dead_letter_id) DO NOTHING
	`,
		record.DeadLetterID, record.EventID, record.DeliveryID, record.SubscriptionID, record.ExtensionID, record.ModuleID,
		string(record.EventTypeID), record.EventVersion, string(record.Reason), record.ErrorCode, record.ErrorMessage, record.Attempts,
		record.PartitionKey, record.OrderingKey, record.PayloadHash, record.ProjectedPayloadHash, record.DefinitionHash,
		record.ScopeSnapshotID, record.PermissionSnapshotID, record.RuntimeInstanceID, record.TraceID, record.OperationID,
		string(originEvent), string(subSnapshot), record.CreatedAt, record.ReplayCount, lastReplayAt, string(record.Status),
		now,
	)
	return err
}

func (s *SQLiteDeadLetterStore) GetDeadLetter(ctx context.Context, deadLetterID string) (DeadLetterRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT dead_letter_id, event_id, delivery_id, subscription_id, extension_id, module_id,
		 event_type_id, event_version, reason, error_code, error_message, attempts,
		 partition_key, ordering_key, payload_hash, projected_payload_hash, definition_hash,
		 scope_snapshot_id, permission_snapshot_id, runtime_instance_id, trace_id, operation_id,
		 origin_event_json, subscription_snapshot_json, created_at, replay_count, last_replay_at, status
		FROM extension_event_dead_letters
		WHERE dead_letter_id = ?
	`, deadLetterID)
	return scanDeadLetterRow(row)
}

func (s *SQLiteDeadLetterStore) ListDeadLetters(ctx context.Context, filter DeadLetterFilter, limit, offset int) ([]DeadLetterRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT dead_letter_id, event_id, delivery_id, subscription_id, extension_id, module_id,
		 event_type_id, event_version, reason, error_code, error_message, attempts,
		 partition_key, ordering_key, payload_hash, projected_payload_hash, definition_hash,
		 scope_snapshot_id, permission_snapshot_id, runtime_instance_id, trace_id, operation_id,
		 origin_event_json, subscription_snapshot_json, created_at, replay_count, last_replay_at, status
		FROM extension_event_dead_letters
		WHERE 1=1
	`
	args := []interface{}{}
	if filter.ExtensionID != "" {
		query += " AND extension_id = ?"
		args = append(args, filter.ExtensionID)
	}
	if filter.SubscriptionID != "" {
		query += " AND subscription_id = ?"
		args = append(args, filter.SubscriptionID)
	}
	if filter.Reason != "" {
		query += " AND reason = ?"
		args = append(args, string(filter.Reason))
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []DeadLetterRecord
	for rows.Next() {
		r, err := scanDeadLetter(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *SQLiteDeadLetterStore) ListByExtension(ctx context.Context, extensionID string, limit, offset int) ([]DeadLetterRecord, error) {
	return s.ListDeadLetters(ctx, DeadLetterFilter{ExtensionID: extensionID}, limit, offset)
}

func (s *SQLiteDeadLetterStore) MarkReplayed(ctx context.Context, deadLetterID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_dead_letters
		SET replay_count = replay_count + 1, last_replay_at = ?, status = ?, updated_at = ?
		WHERE dead_letter_id = ?
	`, now, string(DeadLetterStatusReplayed), now, deadLetterID)
	return err
}

func (s *SQLiteDeadLetterStore) MarkDiscarded(ctx context.Context, deadLetterID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE extension_event_dead_letters
		SET status = ?, updated_at = ?
		WHERE dead_letter_id = ?
	`, string(DeadLetterStatusDiscarded), now, deadLetterID)
	return err
}

func scanDeadLetter(rows *sql.Rows) (DeadLetterRecord, error) {
	var r DeadLetterRecord
	var eventTypeID, reason, status string
	var originEvent, subSnapshot sql.NullString
	var lastReplayAt sql.NullTime
	var errorCode, errorMessage sql.NullString
	var partitionKey, orderingKey, payloadHash, projectedPayloadHash sql.NullString
	var definitionHash, scopeSnapshotID, permissionSnapshotID sql.NullString
	var runtimeInstanceID, traceID, operationID sql.NullString
	err := rows.Scan(
		&r.DeadLetterID, &r.EventID, &r.DeliveryID, &r.SubscriptionID, &r.ExtensionID, &r.ModuleID,
		&eventTypeID, &r.EventVersion, &reason, &errorCode, &errorMessage, &r.Attempts,
		&partitionKey, &orderingKey, &payloadHash, &projectedPayloadHash, &definitionHash,
		&scopeSnapshotID, &permissionSnapshotID, &runtimeInstanceID, &traceID, &operationID,
		&originEvent, &subSnapshot, &r.CreatedAt, &r.ReplayCount, &lastReplayAt,
		&status,
	)
	if err != nil {
		return r, err
	}
	r.EventTypeID = EventTypeID(eventTypeID)
	r.Reason = DeadLetterReason(reason)
	r.Status = DeadLetterStatus(status)
	if originEvent.Valid {
		r.OriginEvent = json.RawMessage(originEvent.String)
	}
	if subSnapshot.Valid {
		r.SubscriptionSnapshot = json.RawMessage(subSnapshot.String)
	}
	r.ErrorCode = errorCode.String
	r.ErrorMessage = errorMessage.String
	r.PartitionKey = partitionKey.String
	r.OrderingKey = orderingKey.String
	r.PayloadHash = payloadHash.String
	r.ProjectedPayloadHash = projectedPayloadHash.String
	r.DefinitionHash = definitionHash.String
	r.ScopeSnapshotID = scopeSnapshotID.String
	r.PermissionSnapshotID = permissionSnapshotID.String
	r.RuntimeInstanceID = runtimeInstanceID.String
	r.TraceID = traceID.String
	r.OperationID = operationID.String
	if lastReplayAt.Valid {
		t := lastReplayAt.Time
		r.LastReplayAt = &t
	}
	return r, nil
}

func scanDeadLetterRow(row *sql.Row) (DeadLetterRecord, error) {
	var r DeadLetterRecord
	var eventTypeID, reason, status string
	var originEvent, subSnapshot sql.NullString
	var lastReplayAt sql.NullTime
	var errorCode, errorMessage sql.NullString
	var partitionKey, orderingKey, payloadHash, projectedPayloadHash sql.NullString
	var definitionHash, scopeSnapshotID, permissionSnapshotID sql.NullString
	var runtimeInstanceID, traceID, operationID sql.NullString
	err := row.Scan(
		&r.DeadLetterID, &r.EventID, &r.DeliveryID, &r.SubscriptionID, &r.ExtensionID, &r.ModuleID,
		&eventTypeID, &r.EventVersion, &reason, &errorCode, &errorMessage, &r.Attempts,
		&partitionKey, &orderingKey, &payloadHash, &projectedPayloadHash, &definitionHash,
		&scopeSnapshotID, &permissionSnapshotID, &runtimeInstanceID, &traceID, &operationID,
		&originEvent, &subSnapshot, &r.CreatedAt, &r.ReplayCount, &lastReplayAt,
		&status,
	)
	if err != nil {
		return r, fmt.Errorf("%w: %v", ErrDeadLetterNotFound, err)
	}
	r.EventTypeID = EventTypeID(eventTypeID)
	r.Reason = DeadLetterReason(reason)
	r.Status = DeadLetterStatus(status)
	if originEvent.Valid {
		r.OriginEvent = json.RawMessage(originEvent.String)
	}
	if subSnapshot.Valid {
		r.SubscriptionSnapshot = json.RawMessage(subSnapshot.String)
	}
	r.ErrorCode = errorCode.String
	r.ErrorMessage = errorMessage.String
	r.PartitionKey = partitionKey.String
	r.OrderingKey = orderingKey.String
	r.PayloadHash = payloadHash.String
	r.ProjectedPayloadHash = projectedPayloadHash.String
	r.DefinitionHash = definitionHash.String
	r.ScopeSnapshotID = scopeSnapshotID.String
	r.PermissionSnapshotID = permissionSnapshotID.String
	r.RuntimeInstanceID = runtimeInstanceID.String
	r.TraceID = traceID.String
	r.OperationID = operationID.String
	if lastReplayAt.Valid {
		t := lastReplayAt.Time
		r.LastReplayAt = &t
	}
	return r, nil
}

var _ = errors.New
