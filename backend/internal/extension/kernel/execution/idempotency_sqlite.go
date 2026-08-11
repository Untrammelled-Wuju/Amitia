package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type sqliteIdempotencyStorage struct {
	db       Executor
	now      func() time.Time
	retention time.Duration
}

func NewExecutionIdempotencyStorage(db Executor) IdempotencyStorage {
	return &sqliteIdempotencyStorage{
		db:       db,
		now:      func() time.Time { return time.Now().UTC() },
		retention: defaultRetentionWindow,
	}
}

func (s *sqliteIdempotencyStorage) Reserve(ctx context.Context, rec IdempotencyReservation) error {
	now := s.now()
	ex := s.getExecutor(ctx)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_execution_idempotency
			(idempotency_key, request_fingerprint, state, work_result_json, safe_to_replay, revision, reservation_json, owner_instance_id, created_at, updated_at, expires_at, released_at)
		VALUES (?, ?, ?, NULL, 1, 1, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT(idempotency_key) DO NOTHING
	`,
		rec.IdempotencyKey,
		rec.RequestFingerprint,
		string(rec.State),
		rawJSON(rec.Reservation),
		rec.OwnerInstanceID,
		now,
		now,
		now.Add(s.retention),
	)
	if err != nil {
		return fmt.Errorf("idempotency: reserve: %w", err)
	}
	return nil
}

func (s *sqliteIdempotencyStorage) Find(ctx context.Context, key string) (*IdempotencyRecord, error) {
	ex := s.getExecutor(ctx)
	row := ex.QueryRowContext(ctx, `
		SELECT idempotency_key, request_fingerprint, state, work_result_json, safe_to_replay, revision, reservation_json, owner_instance_id, created_at, updated_at, expires_at, released_at
		FROM extension_execution_idempotency
		WHERE idempotency_key = ?
	`, key)

	var rec IdempotencyRecord
	var stateStr string
	var workResult sql.NullString
	var reservation sql.NullString
	var ownerID sql.NullString
	var releasedAt sql.NullTime
	var safeInt int
	err := row.Scan(
		&rec.IdempotencyKey,
		&rec.RequestFingerprint,
		&stateStr,
		&workResult,
		&safeInt,
		&rec.Revision,
		&reservation,
		&ownerID,
		&rec.CreatedAt,
		&rec.UpdatedAt,
		&rec.ExpiresAt,
		&releasedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("idempotency: find %s: %w", key, err)
	}
	rec.State = IdempotencyState(stateStr)
	rec.SafeToReplay = safeInt != 0
	if workResult.Valid && workResult.String != "" {
		rec.WorkResultJSON = json.RawMessage(workResult.String)
	}
	if reservation.Valid && reservation.String != "" {
		rec.Reservation = json.RawMessage(reservation.String)
	}
	if ownerID.Valid {
		rec.OwnerInstanceID = ownerID.String
	}
	if releasedAt.Valid {
		t := releasedAt.Time
		rec.ReleasedAt = &t
	}
	return &rec, nil
}

func (s *sqliteIdempotencyStorage) Complete(ctx context.Context, key string, result json.RawMessage) (bool, error) {
	return s.updateState(ctx, key, IdempotencyStateDone, &result)
}

func (s *sqliteIdempotencyStorage) MarkIndeterminate(ctx context.Context, key string) (bool, error) {
	return s.updateState(ctx, key, IdempotencyStateIndeterminate, nil)
}

func (s *sqliteIdempotencyStorage) Release(ctx context.Context, key string) (bool, error) {
	now := s.now()
	ex := s.getExecutor(ctx)
	res, err := ex.ExecContext(ctx, `
		UPDATE extension_execution_idempotency
		SET state = ?,
		    released_at = ?,
		    revision = revision + 1,
		    updated_at = ?
		WHERE idempotency_key = ?
		  AND state IN (?, ?)
	`, string(IdempotencyStateReleased), now, now, key,
		string(IdempotencyStateReserved), string(IdempotencyStateIndeterminate))
	if err != nil {
		return false, fmt.Errorf("idempotency: release %s: %w", key, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *sqliteIdempotencyStorage) updateState(ctx context.Context, key string, state IdempotencyState, result *json.RawMessage) (bool, error) {
	now := s.now()
	ex := s.getExecutor(ctx)
	var res sql.Result
	var err error
	if result != nil {
		res, err = ex.ExecContext(ctx, `
			UPDATE extension_execution_idempotency
			SET state = ?,
			    work_result_json = ?,
			    revision = revision + 1,
			    updated_at = ?
			WHERE idempotency_key = ?
			  AND state IN (?, ?)
		`, string(state), rawJSON(*result), now, key,
			string(IdempotencyStateReserved), string(IdempotencyStateIndeterminate))
	} else {
		res, err = ex.ExecContext(ctx, `
			UPDATE extension_execution_idempotency
			SET state = ?,
			    revision = revision + 1,
			    updated_at = ?
			WHERE idempotency_key = ?
			  AND state IN (?, ?)
		`, string(state), now, key,
			string(IdempotencyStateReserved), string(IdempotencyStateIndeterminate))
	}
	if err != nil {
		return false, fmt.Errorf("idempotency: update state %s: %w", key, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *sqliteIdempotencyStorage) DeleteExpiredCAS(ctx context.Context, now time.Time) (int64, error) {
	ex := s.getExecutor(ctx)
	res, err := ex.ExecContext(ctx, `
		DELETE FROM extension_execution_idempotency
		WHERE expires_at < ?
		  AND state IN (?, ?)
	`, now, string(IdempotencyStateReleased), string(IdempotencyStateDone))
	if err != nil {
		return 0, fmt.Errorf("idempotency: delete expired: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *sqliteIdempotencyStorage) getExecutor(ctx context.Context) Executor {
	if ex, ok := ctx.Value(idempotencyTxKey{}).(Executor); ok && ex != nil {
		return ex
	}
	return s.db
}

type idempotencyTxKey struct{}

func rawJSON(b json.RawMessage) *string {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	return &s
}
