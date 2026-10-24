package task_runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type taskHostExecutorFunc func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error)

func (f taskHostExecutorFunc) Execute(ctx context.Context, run *TaskRun, definition *TaskDefinition, id, method string, call TaskHostNativeCall) (json.RawMessage, error) {
	return f(ctx, run, definition, id, method, call)
}

func TestActualSourceTaskNativeHostUsesAcknowledgedOwnerBridge(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{const result=await ctx.host.executeTool('native.echo',{text:'<>&中文',number:1.2e3});await ctx.host.emitEvent('extension.fixture.updated',{text:'<>&中文'});return {success:true,output:{result,capabilities:ctx.host.capabilities}};};`)
	host.config.HostCapabilities = SourceTaskCapabilities{ExecuteTool: true, EmitEvent: true}
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute", "event.emit"}
	run.TaskRunID, run.TaskDefinitionID = host.config.TaskRunID, definition.TaskID
	run.Input = json.RawMessage(`{}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash, run.Generation, run.ExecutionAttemptID = hashBytes(run.Input), host.config.Generation, "attempt"
	ctx, cancel := context.WithTimeout(coordination.WithScope(t.Context(), authority), 5*time.Second)
	defer cancel()
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	executions := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _ string, method string, call TaskHostNativeCall) (json.RawMessage, error) {
		executions++
		if method == "task.host.executeTool" {
			return json.Marshal(map[string]any{"result": json.RawMessage(call.Input)})
		}
		return json.RawMessage(`{"confirmed":true,"eventId":"event","outboxId":"outbox"}`), nil
	})}
	var output json.RawMessage
	var finishStatus string
	if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{
		OnRequest: func(current context.Context, id, method string, params json.RawMessage) (json.RawMessage, error) {
			confirmation, err := port.Call(current, run, definition, id, method, params)
			if err != nil {
				return nil, err
			}
			var ack TaskHostNativeConfirmation
			if err = json.Unmarshal(confirmation, &ack); err != nil || ack.InputHash != hashBytes(params) || ack.ResultHash != hashBytes(ack.ResultBytes) || ack.Scope != authority {
				return nil, errors.New("invalid owner acknowledgement")
			}
			return json.RawMessage(ack.ResultBytes), nil
		},
		OnFinished: func(status string, result json.RawMessage, _, _, _ string) {
			finishStatus = status
			output = append(json.RawMessage(nil), result...)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if code, err := host.Wait(); err != nil || code != 0 || finishStatus != "succeeded" || executions != 2 {
		t.Fatalf("actual source native bridge failed: %d %v %s %s calls=%d", code, err, finishStatus, output, executions)
	}
	var decoded struct {
		Result struct {
			Text string `json:"text"`
		}
		Capabilities SourceTaskCapabilities
	}
	if json.Unmarshal(output, &decoded) != nil || decoded.Result.Text != "<>&中文" || !decoded.Capabilities.ExecuteTool || !decoded.Capabilities.EmitEvent {
		t.Fatalf("actual SDK changed native result or capabilities: %s", output)
	}
}

func TestOwnedTaskHostPreservesNativeBytesAndRequiresOwnerConfirmation(t *testing.T) {
	for _, scenario := range []string{"confirmed", "start-ack", "final-ack", "unknown", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			_, _, authority, run, definition := taskAuthorityFixture(t)
			definition.PermissionRequirementStrings = []string{"service.tool.execute"}
			run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":true}`)
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			run.InputHash, run.Generation, run.ExecutionAttemptID = hashBytes(run.Input), 1, "attempt"
			ctx, cancel := context.WithCancel(coordination.WithScope(t.Context(), authority))
			defer cancel()
			data := &taskInputData{}
			if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
				t.Fatal(err)
			}
			run.Input = nil
			params := json.RawMessage("{\n \"task_run_id\":\"" + run.TaskRunID + "\", \"tool_id\":\"native.echo\", \"input\":{\"text\":\"<>&中文\",\"number\":1.20e+3} }")
			wire, err := json.Marshal(protocol.TaskOwnerRPCRequest{Method: "task.host.executeTool", Params: params, NativeParamsBytes: params})
			if err != nil {
				t.Fatal(err)
			}
			var transported protocol.TaskOwnerRPCRequest
			if err := json.Unmarshal(wire, &transported); err != nil || !bytes.Equal(transported.NativeParamsBytes, params) {
				t.Fatalf("original native bytes lost: %s %v", wire, err)
			}
			result := json.RawMessage("{\n \"result\":{\"text\":\"<>&中文\",\"number\":1.20e+3} }")
			calls := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				calls++
				if scenario == "unknown" {
					return nil, errors.New("unknown native result")
				}
				if scenario == "final-ack" {
					data.wrongAck = true
				}
				if scenario == "cancel" {
					cancel()
				}
				return result, nil
			})}
			data.wrongAck = scenario == "start-ack"
			confirmed, err := port.Call(ctx, run, definition, "native-request", "task.host.executeTool", transported.NativeParamsBytes)
			if scenario != "confirmed" {
				if err == nil || len(confirmed) != 0 {
					t.Fatalf("unconfirmed native call reported success: %s %v", confirmed, err)
				}
				if scenario == "start-ack" && calls != 0 {
					t.Fatal("native side effect ran before owner start ACK")
				}
				if scenario == "unknown" {
					if _, retryErr := port.Call(ctx, run, definition, "native-request", "task.host.executeTool", params); retryErr == nil || calls != 1 {
						t.Fatalf("unknown operation replayed: %v %d", retryErr, calls)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var ack TaskHostNativeConfirmation
			if json.Unmarshal(confirmed, &ack) != nil || !bytes.Equal(ack.ResultBytes, result) || ack.InputHash != hashBytes(params) || ack.ResultHash != hashBytes(result) || ack.Scope != authority {
				t.Fatalf("native confirmation bytes/authority changed: %s", confirmed)
			}
			replayed, err := port.Call(ctx, run, definition, "native-request", "task.host.executeTool", params)
			if err != nil || !bytes.Equal(replayed, confirmed) || calls != 1 {
				t.Fatalf("confirmed replay re-executed native operation: %s %v %d", replayed, err, calls)
			}
			resource := data.resources["tool-result/"+taskHostOperationID(run, "native-request")]
			var operation ownedTaskHostOperation
			if json.Unmarshal(resource.Body, &operation) != nil || !bytes.Equal(operation.ResultBytes, result) || resource.OwnerID != authority.ResourceOwnerID {
				t.Fatal("owner storage changed private result bytes")
			}
			operation.ResultBytes = json.RawMessage(`{"result":"tampered"}`)
			resource.Body, _ = json.Marshal(operation)
			if _, err := port.Call(ctx, run, definition, "native-request", "task.host.executeTool", params); err == nil || calls != 1 {
				t.Fatal("polluted native replay accepted")
			}
		})
	}
}

func TestOwnedTaskHostInspectionUsesExistingOwnerLedger(t *testing.T) {
	for _, scenario := range []string{"confirmed", "uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			_, _, authority, run, definition := taskAuthorityFixture(t)
			definition.PermissionRequirementStrings = []string{"service.tool.execute"}
			run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"payload":true}`)
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			run.InputHash, run.Generation, run.ExecutionAttemptID = hashBytes(run.Input), 1, "attempt"
			ctx := coordination.WithScope(t.Context(), authority)
			data := &taskInputData{}
			if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
				t.Fatal(err)
			}
			run.Input = nil
			params := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","tool_id":"native.echo","input":{"hello":"world"}}`)
			count := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				count++
				if scenario == "uncertain" {
					return nil, errors.New("tool result unknown")
				}
				return json.RawMessage(`{"result":{"hello":"world"}}`), nil
			})}
			before, err := port.Inspect(ctx, run, definition, "step-1", "task.host.executeTool", params)
			if err != nil || before.State != "absent" || len(before.Confirmation) != 0 {
				t.Fatalf("unexpected missing-state inspection: %+v %v", before, err)
			}
			result, callErr := port.Call(ctx, run, definition, "step-1", "task.host.executeTool", params)
			if scenario == "uncertain" && callErr == nil || scenario == "confirmed" && callErr != nil {
				t.Fatalf("call: %s %v", result, callErr)
			}
			after, err := port.Inspect(ctx, run, definition, "step-1", "task.host.executeTool", params)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "uncertain" {
				if after.State != "started" || len(after.Confirmation) != 0 {
					t.Fatalf("uncertain operation was treated as confirmed: %+v", after)
				}
				if _, err := port.Call(ctx, run, definition, "step-1", "task.host.executeTool", params); err == nil || count != 1 {
					t.Fatalf("uncertain side effect executed twice: %v %d", err, count)
				}
			} else {
				if after.State != "confirmed" || !bytes.Equal(after.Confirmation, result) {
					t.Fatalf("confirmed operation lost acknowledgement: %+v", after)
				}
				if _, err := port.Call(ctx, run, definition, "step-1", "task.host.executeTool", params); err != nil || count != 1 {
					t.Fatalf("confirmed side effect executed twice: %v %d", err, count)
				}
			}
			modified := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","tool_id":"native.echo","input":{"hello":"other"}}`)
			if _, err := port.Inspect(ctx, run, definition, "step-1", "task.host.executeTool", modified); err == nil {
				t.Fatal("a different input hash was accepted for an existing effect")
			}
		})
	}
}

func TestOwnedTaskHostStableEffectReconcilesAcrossAttemptsWithoutReexecution(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute"}
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"payload":true}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash, run.Generation, run.ExecutionAttemptID, run.Attempt = hashBytes(run.Input), 1, "first-attempt", 1
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	params := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"plan.step-01","tool_id":"native.echo","input":{"path":"document.txt"}}`)
	call := TaskHostNativeCall{TaskRunID: run.TaskRunID, EffectID: "plan.step-01"}
	firstID := TaskHostNativeOperationID(run, "process-a/1", call)
	firstNativeID := TaskHostNativeInvocationID(run, "process-a/1", call)
	count := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		count++
		return json.RawMessage(`{"result":{"saved":true}}`), nil
	})}
	first, err := port.Call(ctx, run, definition, "process-a/1", "task.host.executeTool", params)
	if err != nil || count != 1 {
		t.Fatalf("first effect not confirmed: %s %v count=%d", first, err, count)
	}
	run.Generation, run.ExecutionAttemptID, run.Attempt = 2, "second-attempt", 2
	secondID := TaskHostNativeOperationID(run, "process-b/97", call)
	secondNativeID := TaskHostNativeInvocationID(run, "process-b/97", call)
	if secondID != firstID || secondNativeID != firstNativeID {
		t.Fatalf("logical effect changed between attempts: %q / %q; %q / %q", firstID, secondID, firstNativeID, secondNativeID)
	}
	second, err := port.Call(ctx, run, definition, "process-b/97", "task.host.executeTool", params)
	if err != nil || count != 1 {
		t.Fatalf("confirmed prior effect executed again: %s %v count=%d", second, err, count)
	}
	var ack TaskHostNativeConfirmation
	if err := json.Unmarshal(second, &ack); err != nil || ack.AttemptID != "second-attempt" || ack.Generation != 2 || ack.RequestID != "process-b/97" || ack.EffectID != "plan.step-01" || ack.Acknowledgment.Versions["tool-result/"+firstID] != 2 {
		t.Fatalf("invalid current-attempt stable receipt: %s %v", second, err)
	}
	other := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"plan.step-01","tool_id":"native.echo","input":{"path":"other.txt"}}`)
	if _, err := port.Call(ctx, run, definition, "process-b/98", "task.host.executeTool", other); err == nil || count != 1 {
		t.Fatalf("same logical effect accepted different input: %v count=%d", err, count)
	}
	if got := len(data.resources); got != 3 || data.resources["checkpoint/"+taskOwnerCommitCursorID(run)] == nil {
		t.Fatalf("stable effect created duplicate owner record: %d", got)
	}
}

func TestOwnedTaskHostStableEffectKeepsUnknownAndLegacyCallsBlocked(t *testing.T) {
	for _, scenario := range []string{"unknown", "new-on-retry", "legacy-switch"} {
		t.Run(scenario, func(t *testing.T) {
			_, _, authority, run, definition := taskAuthorityFixture(t)
			definition.PermissionRequirementStrings = []string{"service.tool.execute"}
			run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"value":1}`)
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			run.InputHash, run.Generation, run.ExecutionAttemptID, run.Attempt = hashBytes(run.Input), 1, "original", 1
			ctx := coordination.WithScope(t.Context(), authority)
			data := &taskInputData{}
			if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
				t.Fatal(err)
			}
			run.Input = nil
			stable := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"change-001","tool_id":"native.echo","input":{"value":1}}`)
			legacy := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","tool_id":"native.echo","input":{"value":1}}`)
			count := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				count++
				return nil, errors.New("external result unknown")
			})}
			if scenario == "new-on-retry" {
				run.Attempt, run.Generation, run.ExecutionAttemptID = 2, 2, "retried"
			} else {
				params := stable
				if scenario == "legacy-switch" {
					params = legacy
				}
				if _, err := port.Call(ctx, run, definition, "process-a/1", "task.host.executeTool", params); err == nil || count != 1 {
					t.Fatalf("unconfirmed effect returned success: %v count=%d", err, count)
				}
				run.Attempt, run.Generation, run.ExecutionAttemptID = 2, 2, "retried"
			}
			if _, err := port.Call(ctx, run, definition, "process-b/2", "task.host.executeTool", stable); err == nil || count != 0 && count != 1 {
				t.Fatalf("unknown effect or new retry was executed: %v count=%d", err, count)
			}
			if scenario == "new-on-retry" && count != 0 || scenario != "new-on-retry" && count != 1 {
				t.Fatalf("invalid repeat count: %d", count)
			}
		})
	}
}

func TestOwnedTaskHostRejectsUntrackedLegacyNativeEffectOnRetry(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute"}
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"value":1}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash, run.Generation, run.ExecutionAttemptID, run.Attempt = hashBytes(run.Input), 2, "retry", 2
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	calls := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"result":{}}`), nil
	})}
	params := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","tool_id":"native.echo","input":{}}`)
	if _, err := port.Call(ctx, run, definition, "new-rpc-after-crash", "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 0 {
		t.Fatalf("legacy effect unexpectedly dispatched after crash: %v executions=%d", err, calls)
	}
}

func TestOwnedTaskHostRejectsMalformedStableEffectID(t *testing.T) {
	_, _, _, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute"}
	run.Generation, run.ExecutionAttemptID = 1, "attempt"
	for _, literal := range []string{`""`, `null`, `42`, `false`} {
		params := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":` + literal + `,"tool_id":"native.echo","input":{}}`)
		if _, err := parseTaskHostCall(run, definition, "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskInputInvalid) && !IsTaskErrorCode(err, ErrTaskScopeDenied) {
			t.Fatalf("explicit invalid stable effect ID accepted: %s %v", params, err)
		}
	}
	for _, id := range []string{" bad", "a/b", "含中文", strings.Repeat("x", 129), "\n"} {
		params, err := json.Marshal(TaskHostNativeCall{TaskRunID: run.TaskRunID, EffectID: id, ToolID: "native.echo", Input: json.RawMessage(`{}`)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseTaskHostCall(run, definition, "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskInputInvalid) {
			t.Fatalf("invalid effect id accepted: %q %v", id, err)
		}
	}
}

func TestOwnedTaskHostConfirmedReceiptReplaysWithoutExecutor(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute", "event.emit"}
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"value":1}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash, run.Generation, run.ExecutionAttemptID, run.Attempt = hashBytes(run.Input), 1, "first", 1
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	for _, method := range []string{"task.host.executeTool", "task.host.emitEvent"} {
		t.Run(method, func(t *testing.T) {
			call := TaskHostNativeCall{TaskRunID: run.TaskRunID, EffectID: "stable." + method}
			if method == "task.host.executeTool" {
				call.ToolID = "native.echo"
				call.Input = json.RawMessage(`{"value":1}`)
			} else {
				call.Type = "extension.test.updated"
				call.Payload = json.RawMessage(`{"value":1}`)
			}
			params, err := json.Marshal(call)
			if err != nil {
				t.Fatal(err)
			}
			result := json.RawMessage(`{"result":{"value":1}}`)
			if method == "task.host.emitEvent" {
				result = json.RawMessage(`{"confirmed":true,"eventId":"e1","outboxId":"o1"}`)
			}
			count := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				count++
				return result, nil
			})}
			if _, err := port.Call(ctx, run, definition, "original-rpc", method, params); err != nil {
				t.Fatal(err)
			}
			retry := CloneTaskRun(run)
			retry.Generation, retry.ExecutionAttemptID, retry.Attempt = 2, "second", 2
			port.Executor = nil
			replayed, err := port.Call(ctx, retry, definition, "new-process-rpc", method, params)
			if err != nil || count != 1 {
				t.Fatalf("confirmed effect needs executor or was repeated: %v count=%d", err, count)
			}
			var ack TaskHostNativeConfirmation
			if json.Unmarshal(replayed, &ack) != nil || ack.AttemptID != "second" || ack.EffectID != call.EffectID || ack.ResultHash != hashBytes(result) {
				t.Fatalf("bad replay confirmation: %s", replayed)
			}
		})
	}
}

func TestOwnedTaskHostUnseenEffectRequiresExecutorAndCannotReplayOnRetry(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute"}
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"value":1}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash, run.Generation, run.ExecutionAttemptID, run.Attempt = hashBytes(run.Input), 1, "original", 1
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	params := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"new.one","tool_id":"native.echo","input":{}}`)
	port := AcknowledgedTaskHostPort{Data: data}
	if _, err := port.Call(ctx, run, definition, "first", "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) {
		t.Fatalf("missing executor accepted: %v", err)
	}
	if _, err := port.Inspect(ctx, run, definition, "first", "task.host.executeTool", params); err != nil {
		t.Fatal(err)
	}
	run.Attempt, run.Generation, run.ExecutionAttemptID = 2, 2, "retry"
	if _, err := port.Call(ctx, run, definition, "later", "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("unseen retry effect accepted: %v", err)
	}
}
