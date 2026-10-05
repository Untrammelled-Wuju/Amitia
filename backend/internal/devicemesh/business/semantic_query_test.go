package business

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type semanticTestModel struct {
	*testModel
	queries atomic.Int32
}

func (m *semanticTestModel) OwnedQueryVector(context.Context, string) ([]float32, string, error) {
	m.queries.Add(1)
	return []float32{1, 0}, "model", nil
}

type semanticQueryPort struct {
	testDataPort
	seen atomic.Int32
}

func (p *semanticQueryPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	if len(query.Vector) == 2 && query.VectorModel == "model" {
		p.seen.Add(1)
	}
	return p.testDataPort.Snapshot(ctx, scope, query)
}

func TestCoreSuppliesOwnerQueryVectorAndReplaySkipsEmbedding(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	semantic := &semanticTestModel{testModel: model}
	port := &semanticQueryPort{testDataPort: engine.data.(testDataPort)}
	engine.model, engine.data = semantic, port
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "chat", RequestID: "semantic-request", Message: "检索历史记忆"}
	for i := 0; i < 2; i++ {
		response, err := engine.Run(t.Context(), request)
		if err != nil || !response.Saved {
			t.Fatalf("reply=%+v err=%v", response, err)
		}
	}
	if semantic.queries.Load() != 1 || port.seen.Load() != 1 || model.calls.Load() != 1 {
		t.Fatalf("query calls=%d owner queries=%d model calls=%d", semantic.queries.Load(), port.seen.Load(), model.calls.Load())
	}
}
