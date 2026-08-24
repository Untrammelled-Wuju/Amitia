package resource

import (
	"context"
	"sync"
	"testing"
)

func TestRace_AcquireStartupAndStop(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		adapter, reader, _, _, _ := newTestAdapter()
		reader.AddRuntime("rt-1", "p-1", "ext-1")
		reader.AddService("rt-1", "s-1", "p-1", "ext-1")

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
				RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1,
			}, nil)
		}()
		go func() {
			defer wg.Done()
			adapter.MarkStopping("rt-1")
		}()
		wg.Wait()
	}
}

func TestRace_AcquireParallelDifferentRuntimes(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	for i := 0; i < 20; i++ {
		id := "rt-" + string(rune('a'+i))
		reader.AddRuntime(id, "p-1", "ext-1")
		reader.AddService(id, "s-1", "p-1", "ext-1")
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		id := "rt-" + string(rune('a'+i))
		wg.Add(1)
		go func(rid string) {
			defer wg.Done()
			adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
				RuntimeID: rid, PluginID: "p-1", ServiceID: "s-1", Generation: 1,
			}, nil)
		}(id)
	}
	wg.Wait()

	if len(adapter.ActiveSubjects()) != 20 {
		t.Fatalf("expected 20 active, got %d", len(adapter.ActiveSubjects()))
	}
}

func TestRace_AcquireVsRevokeHostState(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-x", "p-1", "ext-1")
	reader.AddService("rt-x", "s-1", "p-1", "ext-1")

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
			RuntimeID: "rt-x", PluginID: "p-1", ServiceID: "s-1", Generation: 1,
		}, nil)
	}()
	go func() {
		defer wg.Done()
		adapter.MarkStopping("rt-x")
	}()
	go func() {
		defer wg.Done()
		adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
			RuntimeID: "rt-x", PluginID: "p-1", ServiceID: "s-1", Generation: 1,
		})
	}()
	wg.Wait()
}

func TestRace_HostShutdownParallelAdmission(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	for i := 0; i < 16; i++ {
		id := "rt-" + string(rune('a'+i))
		reader.AddRuntime(id, "p-1", "ext-1")
		reader.AddService(id, "s-1", "p-1", "ext-1")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		adapter.Shutdown()
	}()
	for i := 0; i < 16; i++ {
		id := "rt-" + string(rune('a'+i))
		wg.Add(1)
		go func(rid string) {
			defer wg.Done()
			_, _ = adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
				RuntimeID: rid, PluginID: "p-1", ServiceID: "s-1", Generation: 1,
			}, nil)
		}(id)
	}
	wg.Wait()
}
