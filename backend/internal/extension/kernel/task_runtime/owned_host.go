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
	Method         string                       `json:"method"`
	InputHash      string                       `json:"inputHash"`
	ResultHash     string                       `json:"resultHash"`
	ResultBytes    []byte                       `json:"resultBytes"`
	Acknowledgment coordination.Acknowledgement `json:"acknowledgement"`
}

type ownedTaskHostOperation struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	TaskRunID             string                      `json:"taskRunId"`
	Generation            int64                       `json:"generation"`
	AttemptID             string                      `json:"attemptId"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	RequestID             string                      `json:"requestId"`
	Method                string                      `json:"method"`
	InputHash             string                      `json:"inputHash"`
	State                 string                      `json:"state"`
	ResultHash            string                      `json:"resultHash,omitempty"`
	ResultBytes           []byte                      `json:"resultBytes,omitempty"`
}

func taskHostOperationID(run *TaskRun, requestID string) string {
	return "task/host/" + run.TaskRunID + "/" + hashBytes([]byte(run.ExecutionAttemptID.String()+"\x00"+requestID))
}

func parseTaskHostCall(run *TaskRun, definition *TaskDefinition, method string, params json.RawMessage) (TaskHostNativeCall, error) {
	var call TaskHostNativeCall
	if run == nil || definition == nil || run.Generation < 1 || run.ExecutionAttemptID == "" || len(params) > 64<<10 || json.Unmarshal(params, &call) != nil || call.TaskRunID != run.TaskRunID {
		return call, NewTaskError(ErrTaskScopeDenied, "任务Native调用缺少一致的原任务身份")
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

func (p AcknowledgedTaskHostPort) Call(ctx context.Context, run *TaskRun, definition *TaskDefinition, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	call, err := parseTaskHostCall(run, definition, method, params)
	if err != nil {
		return nil, err
	}
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok || p.Executor == nil || requestID == "" || len(requestID) > 256 {
		return nil, NewTaskError(ErrTaskDependencyUnavailable, "任务Native所有者确认或执行端口未就绪")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	id := taskHostOperationID(run, requestID)
	inputHash := hashBytes(params)
	document := ownedTaskHostOperation{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), DefinitionFingerprint: run.DefinitionFingerprint, RequestID: requestID, Method: method, InputHash: inputHash, State: "started"}
	stored, err := resources.Resource(ctx, scope, "tool-result", id)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		var previous ownedTaskHostOperation
		if stored.Deleted || stored.OwnerID != scope.ResourceOwnerID || stored.RoleID != scope.RoleID || stored.Kind != "tool-result" || stored.ID != id || stored.Revision != 2 || len(stored.Body) > 128<<10 || json.Unmarshal(stored.Body, &previous) != nil || previous.Scope != scope || previous.TaskRunID != run.TaskRunID || previous.Generation != run.Generation || previous.AttemptID != run.ExecutionAttemptID.String() || previous.DefinitionFingerprint != run.DefinitionFingerprint || previous.RequestID != requestID || previous.Method != method || previous.InputHash != inputHash || previous.State != "confirmed" || len(previous.ResultBytes) > 64<<10 || !json.Valid(previous.ResultBytes) || hashBytes(previous.ResultBytes) != previous.ResultHash {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Native调用已有未确认或不一致记录，禁止自动重复执行")
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		if err := validateTaskHostNativeResult(method, previous.ResultBytes); err != nil {
			return nil, err
		}
		return taskHostConfirmation(scope, run, requestID, method, inputHash, previous.ResultBytes, id)
	}
	commit := func(revision int64) error {
		body, err := json.Marshal(document)
		if err != nil {
			return err
		}
		commitScope := taskHostCommitScope(scope, run, requestID, revision)
		ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "tool-result", ID: id, RoleID: scope.RoleID, ExpectedRevision: revision - 1, Body: body}}})
		if err != nil {
			return err
		}
		if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["tool-result/"+id] != revision {
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
		return nil, err
	}
	return taskHostConfirmation(scope, run, requestID, method, inputHash, result, id)
}

func taskHostCommitScope(scope coordination.ExecutionScope, run *TaskRun, requestID string, revision int64) coordination.ExecutionScope {
	if revision == 1 {
		scope.RequestID += "|task-host-start|" + taskHostOperationID(run, requestID)
	} else {
		scope.RequestID += "|task-host-confirm|" + taskHostOperationID(run, requestID)
	}
	return scope
}

func taskHostConfirmation(scope coordination.ExecutionScope, run *TaskRun, requestID, method, inputHash string, result json.RawMessage, id string) (json.RawMessage, error) {
	commitScope := taskHostCommitScope(scope, run, requestID, 2)
	return json.Marshal(TaskHostNativeConfirmation{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), RequestID: requestID, Method: method, InputHash: inputHash, ResultBytes: result, ResultHash: hashBytes(result), Acknowledgment: coordination.Acknowledgement{OwnerID: scope.ResourceOwnerID, RequestID: commitScope.RequestID, Versions: map[string]int64{"tool-result/" + id: 2}}})
}
