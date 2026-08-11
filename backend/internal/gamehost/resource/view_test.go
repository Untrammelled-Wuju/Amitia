package resource

import (
	"context"
	"sync"
	"testing"
)

type fakeViewResolver struct {
	mu      sync.Mutex
	pending map[string]int
	binary  map[string]int
}

func (r *fakeViewResolver) ResolveCPUMemory(runtimeID string) (int, int64, int64, int, int) {
	return 25, 1024 * 1024 * 256, 1024 * 1024 * 1024 * 4, 128, 4
}

func (r *fakeViewResolver) ResolvePending(runtimeID, serviceID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending[runtimeID+"/"+serviceID]
}

func (r *fakeViewResolver) ResolveBinaryCount(runtimeID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.binary[runtimeID]
}

func TestBuildView_Derived(t *testing.T) {
	resolver := &fakeViewResolver{}
	viewer := NewResourcePolicyViewerWithClock(resolver, func() int64 { return 42 })

	view, err := viewer.BuildView(context.Background(), "rt-1", "svc-1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if view.RuntimeID != "rt-1" || view.ServiceID != "svc-1" {
		t.Fatalf("wrong subject: %+v", view)
	}
	if view.GeneratedAt != 42 {
		t.Fatalf("expected clock 42, got %d", view.GeneratedAt)
	}
	if len(view.Limits) == 0 {
		t.Fatal("expected at least one limit dimension")
	}

	var found bool
	for _, l := range view.Limits {
		if l.Dimension == UsageCPUPercent {
			found = true
		}
	}
	if !found {
		t.Fatal("expected CPU percent dimension")
	}
}

func TestBuildView_NilResolver(t *testing.T) {
	viewer := NewResourcePolicyViewer(nil)
	view, err := viewer.BuildView(context.Background(), "rt", "svc")
	if err != nil {
		t.Fatalf("nil resolver should not error: %v", err)
	}
	if len(view.Limits) != 0 {
		t.Fatal("nil resolver should produce empty view")
	}
}

func TestViewer_RaceConcurrentBuild(t *testing.T) {
	resolver := &fakeViewResolver{pending: make(map[string]int), binary: make(map[string]int)}
	viewer := NewResourcePolicyViewer(resolver)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = viewer.BuildView(context.Background(), "rt-1", "svc-1")
		}()
	}
	wg.Wait()
}
