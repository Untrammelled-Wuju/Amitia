package agent

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestJournalInspectionReservedHandlerRejectsUnsafeShapes(t *testing.T) {
	client := NewMeshClient(MeshClientConfig{ExecutionJournal: executionjournal.NewStore(nil)})
	valid, _ := json.Marshal(executionjournal.TaskInspectionRequest{TaskRunID: "task-1", TaskGeneration: 1})
	base := protocol.RuntimeInvokePayload{RuntimeType: "task_recovery", Handler: executionjournal.TaskInspectionHandler, Input: valid}
	for _, change := range []struct {
		name  string
		apply func(*protocol.RuntimeInvokePayload)
	}{
		{"wrong-runtime", func(p *protocol.RuntimeInvokePayload) { p.RuntimeType = "plugin" }},
		{"authority-spoof", func(p *protocol.RuntimeInvokePayload) { p.AuthorityCallID = "other-authority" }},
		{"owned-scope", func(p *protocol.RuntimeInvokePayload) {
			p.OwnedExecutionScope = json.RawMessage(`{"coordinated":true}`)
		}},
		{"idempotency-mutation", func(p *protocol.RuntimeInvokePayload) { p.IdempotencyKey = "some-effect" }},
		{"workflow-fence", func(p *protocol.RuntimeInvokePayload) { p.FencingToken = 10 }},
		{"unbounded-input", func(p *protocol.RuntimeInvokePayload) { p.Input = make([]byte, 4097) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			req := base
			change.apply(&req)
			if result, err := client.inspectTaskJournal(req); err == nil || result != nil {
				t.Fatalf("unsafe request accepted: %+v %v", result, err)
			}
		})
	}
}
