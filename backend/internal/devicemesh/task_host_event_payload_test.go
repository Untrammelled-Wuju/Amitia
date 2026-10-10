package devicemesh

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestSourceEventWirePreservesOriginalBytesAtDeclaredLimit(t *testing.T) {
	run := &task_runtime.TaskRun{TaskRunID: "run", Generation: 1, ExecutionAttemptID: "attempt"}
	for _, payload := range []json.RawMessage{json.RawMessage(" { \"text\":\"<>&中文\", \"number\":1.20e+3 } "), json.RawMessage(append(append([]byte{'"'}, bytes.Repeat([]byte{'a'}, (64<<10)-2)...), '"'))} {
		call := task_runtime.TaskHostNativeCall{TaskRunID: run.TaskRunID, Type: "extension.fixture.updated", Payload: payload}
		encoded, err := encodeSourceTaskEvent(coordination.ExecutionScope{CoreID: "core"}, run, "native", call, true)
		if err != nil || len(encoded) > 128<<10 {
			t.Fatalf("valid payload does not fit envelope: %d %v", len(encoded), err)
		}
		var decoded task_runtime.TaskHostSourceEventRequest
		if json.Unmarshal(encoded, &decoded) != nil || !bytes.Equal(decoded.PayloadBytes, payload) || len(decoded.Call.Payload) > 0 && string(decoded.Call.Payload) != "null" {
			t.Fatal("Source wire rewrote or duplicated original payload bytes")
		}
		if !bytes.Equal(call.Payload, payload) {
			t.Fatal("Source wire changed caller-owned payload")
		}
	}
	if _, err := encodeSourceTaskEvent(coordination.ExecutionScope{}, run, "native", task_runtime.TaskHostNativeCall{Payload: json.RawMessage(`{}`)}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := encodeSourceTaskEvent(coordination.ExecutionScope{}, run, "native", task_runtime.TaskHostNativeCall{Payload: json.RawMessage(`invalid`)}, false); err == nil {
		t.Fatal("invalid payload accepted")
	}
}
