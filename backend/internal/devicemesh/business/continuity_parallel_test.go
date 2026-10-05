package business

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
)

func TestContinuityRunsIndependentTasksWithBoundedConcurrency(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	started := make(chan struct{}, 6)
	release := make(chan struct{})
	model.generate = func(ctx context.Context, _ Inference) (Generation, error) {
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return Generation{}, ctx.Err()
		case <-release:
			return Generation{Text: "完成检查"}, nil
		}
	}
	for i := 0; i < 6; i++ {
		request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: fmt.Sprintf("create/%d", i)}
		document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: fmt.Sprintf("事项 %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		due := time.Now().Add(-time.Second)
		request.RequestID = fmt.Sprintf("wait/%d", i)
		if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}}); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() { done <- engine.TickContinuity(t.Context()) }()
	for i := 0; i < 4; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("independent tasks were serialized")
		}
	}
	select {
	case <-started:
		close(release)
		t.Fatal("worker concurrency limit exceeded")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("continuity workers did not finish")
	}
	if model.calls.Load() != 6 {
		t.Fatalf("tasks omitted or replayed: %d", model.calls.Load())
	}
}
