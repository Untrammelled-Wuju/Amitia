package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type TaskHostNativeCall struct {
	TaskRunID string          `json:"task_run_id"`
	EffectID  string          `json:"effect_id,omitempty"`
	ToolID    string          `json:"tool_id,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	TimeoutMS int64           `json:"timeout_ms,omitempty"`
	Type      string          `json:"type,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type TaskHostNativeExecutor interface {
	Execute(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error)
}

type OwnedTaskHostPort interface {
	Call(context.Context, *TaskRun, *TaskDefinition, string, string, json.RawMessage) (json.RawMessage, error)
}

type AcknowledgedTaskHostPort struct {
	Data     coordination.DataPort
	Executor TaskHostNativeExecutor
}

type TaskHostNativeConfirmation struct {
	Scope          coordination.ExecutionScope  `json:"executionScope"`
	TaskRunID      string                       `json:"taskRunId"`
	Generation     int64                        `json:"generation"`
	AttemptID      string                       `json:"attemptId"`
	RequestID      string                       `json:"requestId"`
	EffectID       string                       `json:"effectId,omitempty"`
	Method         string                       `json:"method"`
	InputHash      string                       `json:"inputHash"`
	ResultHash     string                       `json:"resultHash"`
	ResultBytes    []byte                       `json:"resultBytes"`
	Acknowledgment coordination.Acknowledgement `json:"acknowledgement"`
}

type TaskHostOperationInspection struct {
	State        string          `json:"state"`
	Confirmation json.RawMessage `json:"confirmation,omitempty"`
}

type ownerNativeCommitUncertain struct{ cause error }

func (e *ownerNativeCommitUncertain) Error() string { return e.cause.Error() }
func (e *ownerNativeCommitUncertain) Unwrap() error { return e.cause }

type ownedTaskHostOperation struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	TaskRunID             string                      `json:"taskRunId"`
	Generation            int64                       `json:"generation"`
	AttemptID             string                      `json:"attemptId"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	RequestID             string                      `json:"requestId"`
	EffectID              string                      `json:"effectId,omitempty"`
	Method                string                      `json:"method"`
	InputHash             string                      `json:"inputHash"`
	State                 string                      `json:"state"`
	ResultHash            string                      `json:"resultHash,omitempty"`
	ResultBytes           []byte                      `json:"resultBytes,omitempty"`
}

func taskHostOperationID(run *TaskRun, requestID string) string {
	return "task/host/" + run.TaskRunID + "/" + hashBytes([]byte(run.ExecutionAttemptID.String()+"\x00"+requestID))
}

func validTaskHostEffectID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return true
}

func taskHostEffectDigest(run *TaskRun, effectID string) string {
	return hashBytes([]byte(run.TaskRunID + "\x00effect\x00" + effectID))
}

func TaskHostNativeOperationID(run *TaskRun, requestID string, call TaskHostNativeCall) string {
	if call.EffectID == "" {
		return taskHostOperationID(run, requestID)
	}
	return "task/host/" + run.TaskRunID + "/effect/" + taskHostEffectDigest(run, call.EffectID)
}

func TaskHostNativeInvocationID(run *TaskRun, requestID string, call TaskHostNativeCall) string {
	if call.EffectID == "" {
		return "task-native-" + hashBytes([]byte(run.TaskRunID + "\x00" + run.ExecutionAttemptID.String() + "\x00" + requestID))[:32]
	}
	return "task-native-" + taskHostEffectDigest(run, call.EffectID)[:32]
}

func parseTaskHostCall(run *TaskRun, definition *TaskDefinition, method string, params json.RawMessage) (TaskHostNativeCall, error) {
	var call TaskHostNativeCall
	if run == nil || definition == nil || run.Generation < 1 || run.ExecutionAttemptID == "" || len(params) > 64<<10 || json.Unmarshal(params, &call) != nil || call.TaskRunID != run.TaskRunID {
		return call, NewTaskError(ErrTaskScopeDenied, "任务Native调用缺少一致的原任务身份")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(params, &fields); err != nil || fields == nil {
		return call, NewTaskError(ErrTaskInputInvalid, "任务Native参数必须为JSON对象")
	}
	if raw, exists := fields["effect_id"]; exists {
		if len(raw) == 0 || raw[0] != '"' || !validTaskHostEffectID(call.EffectID) {
			return call, NewTaskError(ErrTaskInputInvalid, "任务稳定效果编号必须为非空1-128位ASCII字母、数字或_.:-")
		}
	}
	permissionID := ""
	switch method {
	case "task.host.executeTool":
		permissionID = "service.tool.execute"
		if call.ToolID == "" || len(call.ToolID) > 256 || strings.TrimSpace(call.ToolID) != call.ToolID || !json.Valid(call.Input) || call.TimeoutMS < 0 || call.TimeoutMS > 5000 || call.Type != "" || len(call.Payload) != 0 {
			return call, NewTaskError(ErrTaskInputInvalid, "任务工具调用参数无效，超时上限为5秒")
		}
	case "task.host.emitEvent":
		permissionID = "event.emit"
		if call.Type == "" || len(call.Type) > 256 || !json.Valid(call.Payload) || call.ToolID != "" || len(call.Input) != 0 || call.TimeoutMS != 0 {
			return call, NewTaskError(ErrTaskInputInvalid, "任务事件发布参数无效")
		}
	default:
		return call, NewTaskError(ErrTaskScopeDenied, "任务Native端口未授予此方法")
	}
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil {
		return call, err
	}
	for _, requirement := range requirements {
		if requirement.PermissionID == permissionID {
			return call, nil
		}
	}
	return call, NewTaskError(ErrTaskPermissionDenied, "原任务未声明此Native能力的最小权限")
}

func validateTaskHostNativeResult(method string, result json.RawMessage) error {
	if !json.Valid(result) || len(result) > 64<<10 {
		return NewTaskError(ErrTaskScopeDenied, "任务Native结果无效或超过64KiB")
	}
	if method == "task.host.emitEvent" {
		var ack struct {
			Confirmed bool   `json:"confirmed"`
			EventID   string `json:"eventId"`
			OutboxID  string `json:"outboxId"`
		}
		if json.Unmarshal(result, &ack) != nil || !ack.Confirmed || ack.EventID == "" || ack.OutboxID == "" || len(ack.EventID) > 256 || len(ack.OutboxID) > 256 {
			return NewTaskError(ErrTaskScopeDenied, "任务事件尚未获得实际持久事件与发件箱确认")
		}
	} else {
		var ack struct {
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(result, &ack) != nil || !json.Valid(ack.Result) {
			return NewTaskError(ErrTaskScopeDenied, "任务Native工具缺少实际执行结果")
		}
	}
	return nil
}

func (p AcknowledgedTaskHostPort) Inspect(ctx context.Context, run *TaskRun, definition *TaskDefinition, requestID, method string, params json.RawMessage) (TaskHostOperationInspection, error) {
	var empty TaskHostOperationInspection
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return empty, err
	}
	call, err := parseTaskHostCall(run, definition, method, params)
	if err != nil {
		return empty, err
	}
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok || requestID == "" || len(requestID) > 256 {
		return empty, NewTaskError(ErrTaskDependencyUnavailable, "任务Native所有者读取端口未就绪")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return empty, err
	}
	id := TaskHostNativeOperationID(run, requestID, call)
	if call.EffectID != "" {
		legacy, err := resources.Resource(ctx, scope, "tool-result", taskHostOperationID(run, requestID))
		if err != nil {
			return empty, err
		}
		if legacy != nil {
			return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "同一次Native调用已有旧式执行记录，必须先对账再切换稳定效果编号")
		}
	}
	stored, err := resources.Resource(ctx, scope, "tool-result", id)
	if err != nil {
		return empty, err
	}
	if stored == nil {

		cursor, _, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
		if err != nil {
			return empty, err
		}
		if cursor != nil {
			if _, marked := cursor.InFlightEffects[id]; marked {
				return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner游标存在未完成工具记录，但原tool-result已丢失")
			}
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return empty, err
		}
		return TaskHostOperationInspection{State: "absent"}, nil
	}
	var previous ownedTaskHostOperation
	if stored.Deleted || stored.OwnerID != scope.ResourceOwnerID || stored.RoleID != scope.RoleID || stored.Kind != "tool-result" || stored.ID != id || len(stored.Body) > 128<<10 || json.Unmarshal(stored.Body, &previous) != nil || previous.Scope != scope || previous.TaskRunID != run.TaskRunID || previous.DefinitionFingerprint != run.DefinitionFingerprint || previous.EffectID != call.EffectID || (call.EffectID == "" && (previous.Generation != run.Generation || previous.AttemptID != run.ExecutionAttemptID.String() || previous.RequestID != requestID)) || previous.Method != method || previous.InputHash != hashBytes(params) {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "Native调用的所有者记录与原执行身份不一致")
	}

	if previous.Generation < 1 || previous.Generation > run.Generation || previous.AttemptID == "" ||
		(previous.Generation == run.Generation && previous.AttemptID != run.ExecutionAttemptID.String()) {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner历史Native回执属于其他或尚未发生的执行代次")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return empty, err
	}

	cursor, _, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return empty, err
	}
	if stored.Revision == 1 && previous.State == "started" && len(previous.ResultBytes) == 0 && previous.ResultHash == "" {
		if cursor != nil && cursor.InFlightEffects[id] != previous.InputHash {
			return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner工具started记录与提交游标不一致，必须人工对账")
		}
		return TaskHostOperationInspection{State: "started"}, nil
	}
	if stored.Revision != 2 || previous.State != "confirmed" || len(previous.ResultBytes) > 64<<10 || !json.Valid(previous.ResultBytes) || hashBytes(previous.ResultBytes) != previous.ResultHash {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "Native调用结果尚未确认或已有损坏，禁止自动重放")
	}
	if cursor != nil {
		if _, stillPending := cursor.InFlightEffects[id]; stillPending {
			return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner工具已确认但仍被标记为未完成，禁止复用冲突记录")
		}
	}
	if err := validateTaskHostNativeResult(method, previous.ResultBytes); err != nil {
		return empty, err
	}
	confirmation, err := taskHostConfirmation(scope, run, requestID, method, hashBytes(params), previous.ResultBytes, id, call.EffectID)
	if err != nil {
		return empty, err
	}
	return TaskHostOperationInspection{State: "confirmed", Confirmation: confirmation}, nil
}

func (p AcknowledgedTaskHostPort) Call(ctx context.Context, run *TaskRun, definition *TaskDefinition, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	call, err := parseTaskHostCall(run, definition, method, params)
	if err != nil {
		return nil, err
	}
	inspected, err := p.Inspect(ctx, run, definition, requestID, method, params)
	if err != nil {
		return nil, err
	}
	switch inspected.State {
	case "confirmed":
		return inspected.Confirmation, nil
	case "started":
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Native调用已经开始但结果尚未确认，禁止自动重复执行")
	case "absent":

		if run.Attempt > 1 || run.Generation > 1 {
			allowed := false
			if call.EffectID != "" {
				allowed, err = (AcknowledgedTaskCheckpointPort{Data: p.Data}).AuthorizesPostCheckpointEffect(ctx, run, definition, method, call)
				if err != nil {
					return nil, err
				}
			}
			if !allowed {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "恢复后只能首次执行Owner检查点预先声明的稳定效果，未确认或无计划的操作禁止重派")
			}
		}
	default:
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Native调用记录状态无效")
	}
	committed, err := (AcknowledgedTaskOutcomePort{Data: p.Data}).InspectOutcome(ctx, run)
	if err != nil {
		return nil, err
	}
	if committed != nil {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者已确认任务终态，不能再开始新的Native效果")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	if p.Executor == nil {
		return nil, NewTaskError(ErrTaskDependencyUnavailable, "任务Native执行端口未就绪")
	}
	id := TaskHostNativeOperationID(run, requestID, call)
	inputHash := hashBytes(params)
	document := ownedTaskHostOperation{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), DefinitionFingerprint: run.DefinitionFingerprint, RequestID: requestID, EffectID: call.EffectID, Method: method, InputHash: inputHash, State: "started"}
	commit := func(revision int64) error {
		body, err := json.Marshal(document)
		if err != nil {
			return err
		}
		previousCursor, cursorRevision, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
		if err != nil {
			return err
		}
		cursorMutation, err := ownerCursorNativeEffectMutation(scope, run, previousCursor, cursorRevision, id, inputHash, revision == 2)
		if err != nil {
			return err
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		commitScope := taskHostCommitScope(scope, id, revision)
		ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "tool-result", ID: id, RoleID: scope.RoleID, ExpectedRevision: revision - 1, Body: body}, cursorMutation}})
		if err != nil {
			return &ownerNativeCommitUncertain{cause: err}
		}
		if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["tool-result/"+id] != revision || ack.Versions["checkpoint/"+taskOwnerCommitCursorID(run)] != cursorRevision+1 {
			return errors.New("Native调用尚未获得原数据所有者的精确保存确认")
		}
		return coordination.ValidateCurrent(ctx)
	}
	if err := commit(1); err != nil {
		return nil, err
	}
	result, err := p.Executor.Execute(ctx, CloneTaskRun(run), definition, requestID, method, call)
	if err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	if err := validateTaskHostNativeResult(method, result); err != nil {
		return nil, err
	}
	document.State, document.ResultBytes, document.ResultHash = "confirmed", append([]byte(nil), result...), hashBytes(result)
	if err := commit(2); err != nil {

		var uncertain *ownerNativeCommitUncertain
		if !errors.As(err, &uncertain) {
			return nil, err
		}
		if checkErr := coordination.ValidateCurrent(ctx); checkErr != nil {
			return nil, checkErr
		}
		inspected, checkErr := p.Inspect(ctx, run, definition, requestID, method, params)
		if checkErr != nil || inspected.State != "confirmed" {
			return nil, err
		}
		var receipt TaskHostNativeConfirmation
		if json.Unmarshal(inspected.Confirmation, &receipt) != nil || receipt.ResultHash != hashBytes(result) || string(receipt.ResultBytes) != string(result) {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner已确认不同的Native结果，禁止将本次执行视为成功")
		}
		return inspected.Confirmation, nil
	}
	return taskHostConfirmation(scope, run, requestID, method, inputHash, result, id, call.EffectID)
}

func taskHostCommitScope(scope coordination.ExecutionScope, id string, revision int64) coordination.ExecutionScope {
	if revision == 1 {
		scope.RequestID += "|task-host-start|" + id
	} else {
		scope.RequestID += "|task-host-confirm|" + id
	}
	return scope
}

func taskHostConfirmation(scope coordination.ExecutionScope, run *TaskRun, requestID, method, inputHash string, result json.RawMessage, id, effectID string) (json.RawMessage, error) {
	commitScope := taskHostCommitScope(scope, id, 2)
	return json.Marshal(TaskHostNativeConfirmation{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), RequestID: requestID, EffectID: effectID, Method: method, InputHash: inputHash, ResultBytes: result, ResultHash: hashBytes(result), Acknowledgment: coordination.Acknowledgement{OwnerID: scope.ResourceOwnerID, RequestID: commitScope.RequestID, Versions: map[string]int64{"tool-result/" + id: 2}}})
}
