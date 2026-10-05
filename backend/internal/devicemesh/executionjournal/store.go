package executionjournal

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

const Schema = `CREATE TABLE IF NOT EXISTS kernel_device_execution_journal (
	device_id TEXT NOT NULL, action_id TEXT NOT NULL, payload_hash TEXT NOT NULL,
	core_id TEXT NOT NULL, status TEXT NOT NULL, result BLOB, updated_at TEXT NOT NULL,
	PRIMARY KEY(device_id,action_id))`

const FenceSchema = `CREATE TABLE IF NOT EXISTS kernel_device_execution_fences (
	device_id TEXT NOT NULL, workflow_id TEXT NOT NULL, node_id TEXT NOT NULL,
	token INTEGER NOT NULL, PRIMARY KEY(device_id,workflow_id,node_id))`

var ErrUncertain = errors.New("设备动作已发起或结果不明，禁止自动重复执行，请先核查动作结果")
var ErrConflict = errors.New("设备动作编号对应的服务提供者或参数已变化，拒绝重复执行")
var ErrFence = errors.New("设备动作的执行租约已过期")

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func identity(invoke protocol.RuntimeInvokePayload) (string, string, error) {
	action := invoke.IdempotencyKey
	if action == "" {
		action = invoke.InvocationID
	}
	if action == "" || len(action) > 512 || invoke.DeviceID == "" || invoke.SpaceID == "" || invoke.Handler == "" || (len(invoke.Input) > 0 && !json.Valid(invoke.Input)) {
		return "", "", ErrConflict
	}
	encoded, err := json.Marshal(struct {
		Core        string
		RuntimeType string
		Handler     string
		Provider    string
		Workflow    string
		Node        string
		Attempt     int
		Input       json.RawMessage
		Scope       json.RawMessage `json:",omitempty"`
	}{invoke.SpaceID.String(), invoke.RuntimeType, invoke.Handler, invoke.ProviderID, invoke.WorkflowRunID, invoke.WorkflowNodeID, invoke.LogicalAttempt, invoke.Input, invoke.OwnedExecutionScope})
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(encoded)
	return action, hex.EncodeToString(digest[:]), nil
}

func (s *Store) Execute(ctx context.Context, invoke protocol.RuntimeInvokePayload, run func() (*protocol.RuntimeResultPayload, error)) (result *protocol.RuntimeResultPayload, err error) {
	action, digest, err := identity(invoke)
	if err != nil {
		return nil, err
	}
	var ownedScope coordination.ExecutionScope
	if len(invoke.OwnedExecutionScope) > 0 && (len(invoke.OwnedExecutionScope) > 64<<10 || json.Unmarshal(invoke.OwnedExecutionScope, &ownedScope) != nil) {
		return nil, ErrConflict
	}
	if ownedScope.Coordinated {
		current, ok := coordination.FromContext(ctx)
		if !ok || current != ownedScope || ownedScope.CoreID != invoke.SpaceID.String() || ownedScope.ResourceOwnerID != ownedScope.CoreID || ownedScope.TargetDeviceID != invoke.DeviceID.String() {
			return nil, ErrConflict
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	insertion, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_execution_journal(device_id,action_id,payload_hash,core_id,status,updated_at) VALUES(?,?,?,?,'started',?) ON CONFLICT DO NOTHING`, invoke.DeviceID.String(), action, digest, invoke.SpaceID.String(), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	var storedHash, status string
	var storedResult []byte
	if err := tx.QueryRowContext(ctx, `SELECT payload_hash,status,result FROM kernel_device_execution_journal WHERE device_id=? AND action_id=?`, invoke.DeviceID.String(), action).Scan(&storedHash, &status, &storedResult); err != nil {
		return nil, err
	}
	if storedHash != digest {
		return nil, ErrConflict
	}
	if status == "completed" {
		if invoke.WorkflowRunID != "" && invoke.WorkflowNodeID != "" {
			var token int64
			if err := tx.QueryRowContext(ctx, `SELECT token FROM kernel_device_execution_fences WHERE device_id=? AND workflow_id=? AND node_id=?`, invoke.DeviceID.String(), invoke.WorkflowRunID, invoke.WorkflowNodeID).Scan(&token); err != nil {
				return nil, err
			}
			if token != invoke.FencingToken || token < 1 {
				return nil, ErrFence
			}
		}
		var result protocol.RuntimeResultPayload
		if json.Unmarshal(storedResult, &result) != nil {
			return nil, ErrUncertain
		}
		return &result, nil
	}
	inserted, err := insertion.RowsAffected()
	if err != nil {
		return nil, err
	}
	if inserted != 1 {
		return nil, ErrUncertain
	}
	var count, total int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(length(result)),0) FROM kernel_device_execution_journal`).Scan(&count, &total); err != nil {
		return nil, err
	}
	if count > 100000 || total > 127<<20 {
		return nil, errors.New("设备执行记录已达到保存上限，请先归档已完成的记录")
	}
	if invoke.WorkflowRunID != "" && invoke.WorkflowNodeID != "" {
		if invoke.FencingToken < 1 {
			return nil, ErrFence
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_execution_fences(device_id,workflow_id,node_id,token) VALUES(?,?,?,?) ON CONFLICT(device_id,workflow_id,node_id) DO UPDATE SET token=excluded.token WHERE excluded.token>token`, invoke.DeviceID.String(), invoke.WorkflowRunID, invoke.WorkflowNodeID, invoke.FencingToken); err != nil {
			return nil, err
		}
		var token int64
		if err := tx.QueryRowContext(ctx, `SELECT token FROM kernel_device_execution_fences WHERE device_id=? AND workflow_id=? AND node_id=?`, invoke.DeviceID.String(), invoke.WorkflowRunID, invoke.WorkflowNodeID).Scan(&token); err != nil {
			return nil, err
		}
		if token != invoke.FencingToken {
			return nil, ErrFence
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(ErrUncertain, err)
		}
	}()
	result, runErr := run()
	if runErr != nil || result == nil {
		if runErr != nil {
			return nil, runErr
		}
		return nil, ErrUncertain
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > 1<<20 {
		return nil, ErrUncertain
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	completion := "completed"
	if ownedScope.Coordinated {
		completion = "performed"
		encoded = nil
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if invoke.WorkflowRunID != "" && invoke.WorkflowNodeID != "" {
		var token int64
		if err := tx.QueryRowContext(ctx, `SELECT token FROM kernel_device_execution_fences WHERE device_id=? AND workflow_id=? AND node_id=?`, invoke.DeviceID.String(), invoke.WorkflowRunID, invoke.WorkflowNodeID).Scan(&token); err != nil {
			return nil, err
		}
		if token != invoke.FencingToken {
			return nil, ErrFence
		}
	}
	updated, err := tx.ExecContext(ctx, `UPDATE kernel_device_execution_journal SET status=?,result=?,updated_at=? WHERE device_id=? AND action_id=? AND status='started' AND payload_hash=?`, completion, encoded, time.Now().UTC().Format(time.RFC3339Nano), invoke.DeviceID.String(), action, digest)
	if err != nil {
		return nil, err
	}
	changed, err := updated.RowsAffected()
	if err != nil || changed != 1 {
		return nil, ErrUncertain
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
