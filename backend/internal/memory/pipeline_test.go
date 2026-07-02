package memory

import (
	"context"
	"testing"
)

type pipelineLayerFunc struct {
	name    string
	process func(context.Context, string, []map[string]string, string) error
}

func (f pipelineLayerFunc) Name() string {
	return f.name
}

func (f pipelineLayerFunc) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return f.process(ctx, convID, messages, newReply)
}

func TestPipelineExecuteStopsWhenContextCancelled(t *testing.T) {
	called := false
	p := NewPipeline(pipelineLayerFunc{
		name: "cancelled",
		process: func(context.Context, string, []map[string]string, string) error {
			called = true
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Execute(ctx, "conv-1", nil, "")
	if called {
		t.Fatal("expected cancelled pipeline not to execute layer")
	}
	run := p.LastRun()
	if run == nil || len(run.Layers) != 1 {
		t.Fatalf("unexpected run: %#v", run)
	}
	if run.Layers[0].Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", run.Layers[0].Status)
	}
}
