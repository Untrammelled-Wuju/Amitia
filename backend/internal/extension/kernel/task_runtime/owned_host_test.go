package task_runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
