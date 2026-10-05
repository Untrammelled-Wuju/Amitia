package coordination

import (
	"context"
	"database/sql"
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

func (s *Service) RegisterRequest(ctx context.Context, scope ExecutionScope, hash, conversation string) (RequestLocation, bool, error) {
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
		return location, false, ErrRequestConflict
	}
	if location.OwnerID != scope.ResourceOwnerID || location.RoleID != scope.RoleID || location.ModeRevision != scope.ModeRevision || location.ProviderEpoch != scope.ProviderEpoch {
		return location, false, ErrScopeExpired
	}
	return location, n == 1, nil
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
