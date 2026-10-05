package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const DeliveryFailuresSchema = `CREATE TABLE IF NOT EXISTS kernel_device_owned_delivery_failures (
	owner_id TEXT NOT NULL, request_id TEXT NOT NULL, space_id TEXT NOT NULL,
	initiator_device_id TEXT NOT NULL, target_device_id TEXT NOT NULL, core_id TEXT NOT NULL,
	role_id TEXT NOT NULL, conversation_id TEXT NOT NULL DEFAULT '', payload_hash TEXT NOT NULL,
	error_code TEXT NOT NULL, failed_at TEXT NOT NULL,
	PRIMARY KEY(owner_id,request_id))`

var ErrDeliveryRejected = errors.New("保存请求已被所有者拒绝，请重新加载数据后处理冲突")

type DeliveryFailure struct {
	OwnerID        string `json:"ownerId"`
	RequestID      string `json:"requestId"`
	ConversationID string `json:"conversationId,omitempty"`
	ErrorCode      string `json:"errorCode"`
	FailedAt       string `json:"failedAt"`
}

func (s *Service) RejectPending(ctx context.Context, pending PendingCommit, reason error) error {
	if !errors.Is(reason, ErrResourceVersion) && !errors.Is(reason, ErrRequestConflict) {
		return ErrRequestConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var payload []byte
	var hash string
	err = tx.QueryRowContext(ctx, `SELECT payload,payload_hash FROM kernel_device_owned_outbox WHERE owner_id=? AND request_id=?`, pending.Commit.Scope.ResourceOwnerID, pending.Commit.Scope.RequestID).Scan(&payload, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if hash != pending.Hash || payloadHash(payload) != hash {
		return ErrRequestConflict
	}
	var stored Commit
	if json.Unmarshal(payload, &stored) != nil || stored.Scope != pending.Commit.Scope {
		return ErrRequestConflict
	}
	conversation := ""
	for _, mutation := range stored.Mutations {
		var document struct {
			ConversationID string `json:"conversationId"`
		}
		if json.Unmarshal(mutation.Body, &document) == nil && document.ConversationID != "" && len(document.ConversationID) <= 512 {
			if conversation != "" && conversation != document.ConversationID {
				return ErrRequestConflict
			}
			conversation = document.ConversationID
		}
	}
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT payload_hash FROM kernel_device_owned_delivery_failures WHERE owner_id=? AND request_id=?`, stored.Scope.ResourceOwnerID, stored.Scope.RequestID).Scan(&previous)
	if err == nil && previous != hash {
		return ErrRequestConflict
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_delivery_failures(owner_id,request_id,space_id,initiator_device_id,target_device_id,core_id,role_id,conversation_id,payload_hash,error_code,failed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(owner_id,request_id) DO NOTHING`, stored.Scope.ResourceOwnerID, stored.Scope.RequestID, stored.Scope.SpaceID, stored.Scope.InitiatorDeviceID, stored.Scope.TargetDeviceID, stored.Scope.CoreID, stored.Scope.RoleID, conversation, hash, ProtocolErrorCode(reason), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_owned_outbox WHERE owner_id=? AND request_id=? AND payload_hash=?`, stored.Scope.ResourceOwnerID, stored.Scope.RequestID, hash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_owned_delivery_failures WHERE owner_id=? AND rowid NOT IN (SELECT rowid FROM kernel_device_owned_delivery_failures WHERE owner_id=? ORDER BY failed_at DESC,request_id DESC LIMIT 128)`, stored.Scope.ResourceOwnerID, stored.Scope.ResourceOwnerID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) DeliveryFailures(ctx context.Context, scope ExecutionScope, conversation string) ([]DeliveryFailure, error) {
	if err := s.Validate(ctx, scope); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT owner_id,request_id,conversation_id,error_code,failed_at FROM kernel_device_owned_delivery_failures WHERE owner_id=? AND space_id=? AND initiator_device_id=? AND target_device_id=? AND core_id=? AND role_id=? AND (?='' OR conversation_id=?) ORDER BY failed_at DESC,request_id DESC LIMIT 128`, scope.ResourceOwnerID, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, scope.CoreID, scope.RoleID, conversation, conversation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]DeliveryFailure, 0)
	for rows.Next() {
		var item DeliveryFailure
		if err := rows.Scan(&item.OwnerID, &item.RequestID, &item.ConversationID, &item.ErrorCode, &item.FailedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
