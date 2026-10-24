package executionjournal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type Inspection struct {
	State  string                         `json:"state"`
	Result *protocol.RuntimeResultPayload `json:"result,omitempty"`
}

func (s *Store) Inspect(ctx context.Context, invoke protocol.RuntimeInvokePayload) (Inspection, error) {
	var empty Inspection
	if s == nil || s.db == nil {
		return empty, errors.New("设备持久执行账本未就绪")
	}
	action, digest, err := identity(invoke)
	if err != nil {
		return empty, err
	}
	var owned coordination.ExecutionScope
	if len(invoke.OwnedExecutionScope) > 0 && (len(invoke.OwnedExecutionScope) > 64<<10 || json.Unmarshal(invoke.OwnedExecutionScope, &owned) != nil) {
		return empty, ErrConflict
	}
	if owned.Coordinated {
		actual, ok := coordination.FromContext(ctx)
		if !ok || actual != owned || owned.CoreID != invoke.SpaceID.String() || owned.ResourceOwnerID != owned.CoreID || owned.TargetDeviceID != invoke.DeviceID.String() {
			return empty, ErrConflict
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return empty, err
	}
	var storedHash, coreID, status string
	var raw []byte
	err = s.db.QueryRowContext(ctx, `SELECT payload_hash,core_id,status,result FROM kernel_device_execution_journal WHERE device_id=? AND action_id=?`, invoke.DeviceID.String(), action).Scan(&storedHash, &coreID, &status, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Inspection{State: "absent"}, nil
	}
	if err != nil {
		return empty, err
	}
	if storedHash != digest || coreID != invoke.SpaceID.String() {
		return empty, ErrConflict
	}
	if invoke.WorkflowRunID != "" && invoke.WorkflowNodeID != "" {
		var token int64
		err = s.db.QueryRowContext(ctx, `SELECT token FROM kernel_device_execution_fences WHERE device_id=? AND workflow_id=? AND node_id=?`, invoke.DeviceID.String(), invoke.WorkflowRunID, invoke.WorkflowNodeID).Scan(&token)
		if err != nil || token < 1 || token != invoke.FencingToken {
			return empty, ErrFence
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return empty, err
	}
	switch status {
	case "started", "performed":
		return Inspection{State: status}, nil
	case "completed":
		if owned.Coordinated {
			return empty, ErrUncertain
		}
		var result protocol.RuntimeResultPayload
		if len(raw) == 0 || json.Unmarshal(raw, &result) != nil {
			return empty, ErrUncertain
		}
		return Inspection{State: "completed", Result: &result}, nil
	default:
		return empty, ErrUncertain
	}
}
