package resource

import (
	"context"
	"testing"
)

func TestLifecycleCoordinator_OnRuntimeStop_MarksStopping(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	reader.AddRuntime("rt-2", "p-1", "ext-1")
	reader.AddService("rt-2", "s-1", "p-1", "ext-1")

	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	}, nil)
	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-2", PluginID: "p-1", ServiceID: "s-1",
	}, nil)

	coord := NewLifecycleCoordinator(adapter, nil)
	coord.OnRuntimeStop("rt-1")

	decision, _ := adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	})
	if decision.Allowed {
		t.Fatal("stopped runtime should deny pending")
	}

	decision, _ = adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-2", PluginID: "p-1", ServiceID: "s-1",
	})
	if !decision.Allowed {
		t.Fatal("rt-2 should still allow")
	}
}

func TestLifecycleCoordinator_OnRuntimeRestart_ClearsStopping(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	}, nil)
	adapter.MarkStopping("rt-1")

	coord := NewLifecycleCoordinator(adapter, nil)
	coord.OnRuntimeRestart("rt-1")

	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	}, nil)
	decision, _ := adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	})
	if !decision.Allowed {
		t.Fatal("restart should clear stopping mark")
	}
}

func TestLifecycleCoordinator_OnExtensionDisabled_MarksAll(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	reader.AddRuntime("rt-2", "p-1", "ext-1")
	reader.AddService("rt-2", "s-1", "p-1", "ext-1")

	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	}, nil)
	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-2", PluginID: "p-1", ServiceID: "s-1",
	}, nil)

	coord := NewLifecycleCoordinator(adapter, nil)
	coord.OnExtensionDisabled("ext-1")

	for _, id := range []string{"rt-1", "rt-2"} {
		decision, _ := adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
			RuntimeID: id, PluginID: "p-1", ServiceID: "s-1",
		})
		if decision.Allowed {
			t.Fatalf("disabled extension %s should deny pending", id)
		}
	}
}

func TestLifecycleCoordinator_OnHostShutdown(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	}, nil)

	coord := NewLifecycleCoordinator(adapter, nil)
	coord.OnHostShutdown()

	if _, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1",
	}, nil); err != ErrHostShutdown {
		t.Fatalf("expected ErrHostShutdown, got %v", err)
	}
}

func TestLifecycleCoordinator_NilAdapter(t *testing.T) {
	coord := NewLifecycleCoordinator(nil, nil)
	coord.OnRuntimeStop("x")
	coord.OnRuntimeRestart("x")
	coord.OnExtensionDisabled("x")
	coord.OnExtensionUninstalled("x")
	coord.OnHostShutdown()
}
