package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const RequestRegistrySchema = `CREATE TABLE IF NOT EXISTS kernel_device_business_requests (
	space_id TEXT NOT NULL, device_id TEXT NOT NULL, request_id TEXT NOT NULL,
	payload_hash TEXT NOT NULL, owner_id TEXT NOT NULL, conversation_id TEXT NOT NULL,
	role_id TEXT NOT NULL, mode_revision INTEGER NOT NULL, provider_epoch INTEGER NOT NULL,
	state TEXT NOT NULL DEFAULT 'started', created_at INTEGER NOT NULL,
	PRIMARY KEY(space_id, device_id, request_id))`

type RequestLocation struct {
	Hash           string
	OwnerID        string
	ConversationID string
	RoleID         string
	ModeRevision   int64
	ProviderEpoch  int64
	State          string
}

func (s *Service) RegisterRequest(ctx context.Context, scope ExecutionScope, hash, conversation string, queuedHash ...string) (RequestLocation, bool, error) {
	if hash == "" || conversation == "" || scope.RequestID == "" {
		return RequestLocation{}, false, ErrRequestConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Validate(ctx, scope); err != nil {
		return RequestLocation{}, false, err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO kernel_device_business_requests(space_id,device_id,request_id,payload_hash,owner_id,conversation_id,role_id,mode_revision,provider_epoch,created_at) VALUES(?,?,?,?,?,?,?,?,?,unixepoch()) ON CONFLICT DO NOTHING`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, hash, scope.ResourceOwnerID, conversation, scope.RoleID, scope.ModeRevision, scope.ProviderEpoch)
	if err != nil {
		return RequestLocation{}, false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return RequestLocation{}, false, err
	}
	var location RequestLocation
	err = s.db.QueryRowContext(ctx, `SELECT payload_hash,owner_id,conversation_id,role_id,mode_revision,provider_epoch,state FROM kernel_device_business_requests WHERE space_id=? AND device_id=? AND request_id=?`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID).Scan(&location.Hash, &location.OwnerID, &location.ConversationID, &location.RoleID, &location.ModeRevision, &location.ProviderEpoch, &location.State)
	if err != nil {
		return location, false, err
	}
	if location.Hash != hash {
		if location.State == "queued" && len(queuedHash) == 1 && queuedHash[0] == location.Hash && location.OwnerID == scope.ResourceOwnerID && location.RoleID == scope.RoleID && location.ConversationID == conversation && location.ModeRevision == scope.ModeRevision && location.ProviderEpoch == scope.ProviderEpoch {
			result, err := s.db.ExecContext(ctx, `UPDATE kernel_device_business_requests SET state='started',payload_hash=? WHERE space_id=? AND device_id=? AND request_id=? AND state='queued' AND payload_hash=?`, hash, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, queuedHash[0])
			if err != nil {
				return location, false, err
			}
			claimed, err := result.RowsAffected()
			location.Hash, location.State = hash, "started"
			return location, claimed == 1, err
		}
		return location, false, ErrRequestConflict
	}
	if location.OwnerID != scope.ResourceOwnerID || location.RoleID != scope.RoleID || location.ModeRevision != scope.ModeRevision || location.ProviderEpoch != scope.ProviderEpoch {
		return location, false, ErrScopeExpired
	}
	return location, n == 1, nil
}

type QueuedRequestLocation struct {
	Scope          ExecutionScope
	ConversationID string
	Cursor         int64
}

func (s *Service) RegisterQueuedRequest(ctx context.Context, scope ExecutionScope, hash, conversation string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Validate(ctx, scope); err != nil {
		return err
	}
	encoded, err := json.Marshal(scope)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO kernel_device_business_requests(space_id,device_id,request_id,payload_hash,owner_id,conversation_id,role_id,mode_revision,provider_epoch,state,created_at,queued_scope_json) VALUES(?,?,?,?,?,?,?,?,?,'queued',unixepoch(),?) ON CONFLICT DO NOTHING`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, hash, scope.ResourceOwnerID, conversation, scope.RoleID, scope.ModeRevision, scope.ProviderEpoch, string(encoded))
	if err != nil {
		return err
	}
	var saved, storedHash, state string
	err = s.db.QueryRowContext(ctx, `SELECT queued_scope_json,payload_hash,state FROM kernel_device_business_requests WHERE space_id=? AND device_id=? AND request_id=?`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID).Scan(&saved, &storedHash, &state)
	if err != nil {
		return err
	}
	if saved == "" && state != "queued" {
		return nil
	}
	if saved != string(encoded) || state == "queued" && storedHash != hash {
		return ErrRequestConflict
	}
	return nil
}

func (s *Service) PendingQueuedRequests(ctx context.Context, after int64) ([]QueuedRequestLocation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT rowid,queued_scope_json,conversation_id FROM kernel_device_business_requests WHERE rowid>? AND queued_scope_json<>'' ORDER BY rowid LIMIT 64`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []QueuedRequestLocation{}
	for rows.Next() {
		var location QueuedRequestLocation
		var raw string
		if err := rows.Scan(&location.Cursor, &raw, &location.ConversationID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &location.Scope); err != nil {
			return nil, err
		}
		result = append(result, location)
	}
	return result, rows.Err()
}

func (s *Service) FinishQueuedRequest(ctx context.Context, scope ExecutionScope) error {
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_device_business_requests SET queued_scope_json='',state=CASE WHEN state='queued' THEN 'started' ELSE state END WHERE space_id=? AND device_id=? AND request_id=? AND owner_id=?`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, scope.ResourceOwnerID)
	return err
}

func (s *Service) ResetUnstartedQueuedRequest(ctx context.Context, scope ExecutionScope, hash string) error {
	if err := s.Validate(ctx, scope); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_device_business_requests SET state='queued',payload_hash=? WHERE space_id=? AND device_id=? AND request_id=? AND owner_id=? AND queued_scope_json<>'' AND state='started'`, hash, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, scope.ResourceOwnerID)
	return err
}

func (s *Service) CompleteRequest(ctx context.Context, scope ExecutionScope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Validate(ctx, scope); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE kernel_device_business_requests SET state='completed' WHERE space_id=? AND device_id=? AND request_id=? AND owner_id=? AND mode_revision=? AND provider_epoch=?`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, scope.ResourceOwnerID, scope.ModeRevision, scope.ProviderEpoch)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return errors.Join(ErrScopeExpired, sql.ErrNoRows)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_memory_jobs(space_id,device_id,request_id,target_id,next_attempt_at) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, scope.TargetDeviceID, time.Now().Add(10*time.Second).Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
