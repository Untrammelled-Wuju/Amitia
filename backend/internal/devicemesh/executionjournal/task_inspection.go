package executionjournal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const TaskInspectionHandler = "task.journal.inspect"

type TaskInspectionRequest struct {
	TaskRunID      string `json:"taskRunId"`
	TaskGeneration int64  `json:"taskGeneration"`
	TaskExecutionBinding
}

func (s *Store) InspectTask(ctx context.Context, coreID, deviceID string, request TaskInspectionRequest) (Inspection, error) {
	if s == nil || s.db == nil {
		return Inspection{}, errors.New("设备持久执行账本未就绪")
	}
	if !validTaskInspectionIdentity(coreID, deviceID, request) {
		return Inspection{}, ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return Inspection{}, err
	}
	actionID := fmt.Sprintf("task/%s/%d", request.TaskRunID, request.TaskGeneration)
	var storedCore, status string
	var storedBinding []byte
	err := s.db.QueryRowContext(ctx, `SELECT core_id,status,result FROM kernel_device_execution_journal WHERE device_id=? AND action_id=?`, deviceID, actionID).Scan(&storedCore, &status, &storedBinding)
	if errors.Is(err, sql.ErrNoRows) {
		return Inspection{State: "absent"}, nil
	}
	if err != nil {
		return Inspection{}, err
	}
	if storedCore != coreID {
		return Inspection{}, ErrConflict
	}

	var binding TaskExecutionBinding
	if len(storedBinding) == 0 || json.Unmarshal(storedBinding, &binding) != nil || binding != request.TaskExecutionBinding {
		return Inspection{}, ErrUncertain
	}
	if err := ctx.Err(); err != nil {
		return Inspection{}, err
	}
	switch status {
	case "started", "performed", "stopped_unknown", "stopped_performed":
		return Inspection{State: status}, nil
	case "completed":

		return Inspection{State: "completed"}, nil
	default:
		return Inspection{}, ErrUncertain
	}
}

func (b TaskExecutionBinding) validForInspection() bool {
	return b.AttemptID != "" && b.LeaseID != "" && b.RuntimeID != "" && b.SessionID != "" && b.ConnectionGeneration > 0 && b.TaskDefinitionID != "" &&
		len(b.AttemptID) <= 256 && len(b.LeaseID) <= 256 && len(b.RuntimeID) <= 256 && len(b.SessionID) <= 256 && len(b.TaskDefinitionID) <= 256
}

func validTaskInspectionIdentity(coreID, deviceID string, request TaskInspectionRequest) bool {
	return coreID != "" && deviceID != "" && request.TaskExecutionBinding.validForInspection() &&
		request.TaskGeneration >= 1 && request.TaskGeneration <= 9007199254740991 &&
		request.TaskRunID != "" && len(request.TaskRunID) <= 256 &&
		strings.TrimSpace(request.TaskRunID) == request.TaskRunID &&
		!strings.ContainsAny(request.TaskRunID, `/\\`) && strings.IndexFunc(request.TaskRunID, unicode.IsControl) < 0
}

func (s *Store) RecordWorkerStop(ctx context.Context, coreID, deviceID string, request TaskInspectionRequest) error {
	if s == nil || s.db == nil {
		return errors.New("设备持久执行账本未就绪")
	}
	if !validTaskInspectionIdentity(coreID, deviceID, request) {
		return ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actionID := fmt.Sprintf("task/%s/%d", request.TaskRunID, request.TaskGeneration)
	bindingRaw, err := json.Marshal(request.TaskExecutionBinding)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var storedCore, status string
	var storedBinding []byte
	if err := tx.QueryRowContext(ctx, `SELECT core_id,status,result FROM kernel_device_execution_journal WHERE device_id=? AND action_id=?`, deviceID, actionID).Scan(&storedCore, &status, &storedBinding); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUncertain
		}
		return err
	}
	var actual TaskExecutionBinding
	if storedCore != coreID || json.Unmarshal(storedBinding, &actual) != nil || actual != request.TaskExecutionBinding {
		return ErrConflict
	}
	var newStatus string
	switch status {
	case "started":
		newStatus = "stopped_unknown"
	case "performed":
		newStatus = "stopped_performed"
	case "stopped_unknown", "stopped_performed":

		return nil
	default:
		return ErrUncertain
	}
	updated, err := tx.ExecContext(ctx, `UPDATE kernel_device_execution_journal SET status=?,updated_at=? WHERE device_id=? AND action_id=? AND core_id=? AND status=? AND result=?`, newStatus, time.Now().UTC().Format(time.RFC3339Nano), deviceID, actionID, coreID, status, bindingRaw)
	if err != nil {
		return err
	}
	count, err := updated.RowsAffected()
	if err != nil || count != 1 {
		return ErrUncertain
	}
	return tx.Commit()
}
