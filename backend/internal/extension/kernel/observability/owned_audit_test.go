package observability

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestDeviceOwnedAuditPersistsHashesWithoutDeviceBody(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{ResourceOwnerID: "device"})
	invocation := capability.ToolInvocationContext{InvocationID: "invoke", SpaceID: "core", Metadata: map[string]any{"userText": "private-device-input"}}
	if err := hook.BeginInvocation(ctx, invocation, "tool", []byte(`{"text":"private-device-input"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	inputRecord, err := store.GetInvocation(ctx, "invoke")
	if err != nil || len(inputRecord.InputHash) != 64 {
		t.Fatalf("input audit hash missing: %+v %v", inputRecord, err)
	}
	inputEncoded, _ := json.Marshal(inputRecord)
	if strings.Contains(string(inputEncoded), "private-device") {
		t.Fatalf("device input persisted in audit: %s", inputEncoded)
	}
	result := capability.NewToolFailureResult("invoke", "tool", &capability.ToolError{Code: "failed", Message: "private-device-error"})
	result.Structured = json.RawMessage(`{"text":"private-device-output"}`)
	if err := hook.FinishInvocation(ctx, invocation, result, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	outputRecord, err := store.GetInvocation(ctx, "invoke")
	if err != nil || len(outputRecord.OutputHash) != 64 {
		t.Fatalf("output audit hash missing: %+v %v", outputRecord, err)
	}
	outputEncoded, _ := json.Marshal(outputRecord)
	if strings.Contains(string(outputEncoded), "private-device") {
		t.Fatalf("device output persisted in audit: %s", outputEncoded)
	}
	if err := hook.OnSideEffectRecorded(ctx, "invoke", []capability.RecordedSideEffect{{Type: "write", Target: "private-device-path", Description: "private-device-action"}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	record, err := store.GetInvocation(ctx, "invoke")
	if err != nil {
		t.Fatalf("audit hash missing: %+v %v", record, err)
	}
	encoded, _ := json.Marshal(record)
	if strings.Contains(string(encoded), "private-device") {
		t.Fatalf("device body persisted in invocation: %s", encoded)
	}
	events, _, err := store.ListRuntimeEvents(ctx, EventFilter{InvocationID: "invoke", ListOptions: ListOptions{Limit: 20}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(events)
	if strings.Contains(string(encoded), "private-device") {
		t.Fatalf("device body persisted in events: %s", encoded)
	}
}
