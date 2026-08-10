package hostapi

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type fakeRuntimeStateReader struct {
	pluginExists   bool
	runtimeExists  bool
	runtimeOwnedBy bool
}

func (f *fakeRuntimeStateReader) RuntimeExists(ctx context.Context, runtimeID string) (bool, error) {
	return f.runtimeExists, nil
}

func (f *fakeRuntimeStateReader) RuntimeOwnedBy(ctx context.Context, runtimeID string, pluginID string) (bool, error) {
	return f.runtimeOwnedBy, nil
}

func (f *fakeRuntimeStateReader) PluginExists(ctx context.Context, pluginID string) (bool, error) {
	return f.pluginExists, nil
}

type fakeTopologyReader struct{ belongs bool }

func (f *fakeTopologyReader) ServiceBelongsToRuntime(ctx context.Context, runtimeID string, serviceID string) error {
	if f.belongs {
		return nil
	}
	return errors.New("service not in topology")
}

type fakeRuntimeResolver struct {
	err error
	ext string
	mod string
	rtt string
	gen int64
}

func (f *fakeRuntimeResolver) RuntimeInfo(ctx context.Context, runtimeID string) (string, string, string, int64, error) {
	if f.err != nil {
		return "", "", "", 0, f.err
	}
	return f.ext, f.mod, f.rtt, f.gen, nil
}

func TestIdentityMapper_NormalMapping(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: true, runtimeOwnedBy: true},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{ext: "test.example/app", mod: "core", rtt: "service", gen: 7},
	)
	identity, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "p1", RuntimeID: "r1", ServiceID: "s1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.InstanceID != "r1" {
		t.Fatalf("expected InstanceID r1, got %s", identity.InstanceID)
	}
	if identity.ExtensionID != domain.ExtensionID("test.example/app") {
		t.Fatalf("expected ExtensionID, got %s", identity.ExtensionID)
	}
	if identity.ModuleID != domain.ModuleID("core") {
		t.Fatalf("expected ModuleID core, got %s", identity.ModuleID)
	}
	if identity.Generation != 7 {
		t.Fatalf("expected Generation 7, got %d", identity.Generation)
	}
}

func TestIdentityMapper_RejectsUnknownPlugin(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: false},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{},
	)
	_, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "unknown", RuntimeID: "r1", ServiceID: "s1",
	})
	if err == nil {
		t.Fatalf("expected rejection for unknown plugin")
	}
}

func TestIdentityMapper_RejectsUnknownRuntime(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: false},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{},
	)
	_, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "p1", RuntimeID: "unknown", ServiceID: "s1",
	})
	if err == nil {
		t.Fatalf("expected rejection for unknown runtime")
	}
}

func TestIdentityMapper_RejectsRuntimePluginMismatch(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: true, runtimeOwnedBy: false},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{},
	)
	_, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "p1", RuntimeID: "other-runtime", ServiceID: "s1",
	})
	if err == nil {
		t.Fatalf("expected rejection for runtime not owned by plugin")
	}
}

func TestIdentityMapper_RejectsServiceNotInRuntime(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: true, runtimeOwnedBy: true},
		&fakeTopologyReader{belongs: false},
		&fakeRuntimeResolver{},
	)
	_, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "p1", RuntimeID: "r1", ServiceID: "foreign-service",
	})
	if err == nil {
		t.Fatalf("expected rejection for service not in runtime topology")
	}
}

func TestIdentityMapper_RejectsResolverError(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: true, runtimeOwnedBy: true},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{err: errors.New("kernel unavailable")},
	)
	_, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "p1", RuntimeID: "r1", ServiceID: "s1",
	})
	if err == nil {
		t.Fatalf("expected rejection when runtime info lookup fails")
	}
}

func TestIdentityMapper_RejectsEmptyPeer(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: true, runtimeOwnedBy: true},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{ext: "a/b", mod: "c"},
	)
	_, err := mapper.MapIdentity(context.Background(), ipc.Peer{})
	if err == nil {
		t.Fatalf("expected rejection for empty peer")
	}
}

func TestIdentityMapper_ConnectionIDNotInIdentity(t *testing.T) {
	mapper := NewIdentityMapper(
		&fakeRuntimeStateReader{pluginExists: true, runtimeExists: true, runtimeOwnedBy: true},
		&fakeTopologyReader{belongs: true},
		&fakeRuntimeResolver{ext: "a/b", mod: "c"},
	)
	identity, err := mapper.MapIdentity(context.Background(), ipc.Peer{
		PluginID: "p1", RuntimeID: "r1", ServiceID: "s1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var _ runtime_supervisor.RuntimeIdentity = identity
}
