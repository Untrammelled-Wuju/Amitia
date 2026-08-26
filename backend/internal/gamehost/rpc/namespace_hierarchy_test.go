package rpc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/rpc"
)

func TestValidateCustomNamespaceHierarchical(t *testing.T) {
	valid := []rpc.Namespace{"mock", "mock.core", "vendor_name.game-v2.rpc"}
	for _, namespace := range valid {
		if err := rpc.ValidateCustomNamespace(namespace); err != nil {
			t.Fatalf("expected namespace %q to be valid: %v", namespace, err)
		}
	}

	invalid := []rpc.Namespace{
		".mock", "mock.", "mock..core",
		"host.custom", "binary.vendor", "permission.plugin",
	}
	for _, namespace := range invalid {
		if err := rpc.ValidateCustomNamespace(namespace); err == nil {
			t.Fatalf("expected namespace %q to be rejected", namespace)
		}
	}
}

func TestNamespaceCandidatesOfMethodMostSpecificFirst(t *testing.T) {
	got := rpc.NamespaceCandidatesOfMethod("mock.core.inventory.read")
	want := []rpc.Namespace{"mock.core.inventory", "mock.core", "mock"}
	if len(got) != len(want) {
		t.Fatalf("candidates length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNamespaceCandidatesOfMethodRejectInvalidMethod(t *testing.T) {
	for _, method := range []rpc.Method{"mock", "mock..read", ".mock.read", "mock.Read"} {
		if got := rpc.NamespaceCandidatesOfMethod(method); len(got) != 0 {
			t.Fatalf("invalid method %q returned candidates: %#v", method, got)
		}
	}
}

func TestNamespaceCandidatesOfMethodBoundedByNamespaceLimit(t *testing.T) {
	method := rpc.Method("root." + strings.Repeat("segment.", 80) + "read")
	candidates := rpc.NamespaceCandidatesOfMethod(method)
	if len(candidates) == 0 {
		t.Fatal("expected at least the root namespace candidate")
	}
	for _, candidate := range candidates {
		if len(candidate) > 256 {
			t.Fatalf("candidate length %d exceeds namespace limit: %q", len(candidate), candidate)
		}
	}
}

func TestNamespaceRegistryResolveUsesLongestRegisteredPrefix(t *testing.T) {
	ctx := context.Background()
	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{})
	for _, route := range []rpc.Route{
		{RuntimeID: "runtime-1", PluginID: "plugin-1", ServiceID: "service-root", Namespace: "mock"},
		{RuntimeID: "runtime-1", PluginID: "plugin-1", ServiceID: "service-core", Namespace: "mock.core"},
		{RuntimeID: "runtime-1", PluginID: "plugin-1", ServiceID: "service-inventory", Namespace: "mock.core.inventory"},
	} {
		if err := reg.Register(ctx, route); err != nil {
			t.Fatalf("register %q: %v", route.Namespace, err)
		}
	}

	resolved, err := reg.Resolve(ctx, "runtime-1", "mock.core.inventory.read")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Namespace != "mock.core.inventory" || resolved.ServiceID != "service-inventory" {
		t.Fatalf("resolved = %#v, want most-specific namespace", resolved)
	}
}

func TestNamespaceRegistryResolveFallsBackToParentPrefix(t *testing.T) {
	ctx := context.Background()
	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{})
	if err := reg.Register(ctx, rpc.Route{
		RuntimeID: "runtime-1", PluginID: "plugin-1", ServiceID: "service-core", Namespace: "mock.core",
	}); err != nil {
		t.Fatal(err)
	}

	resolved, err := reg.Resolve(ctx, "runtime-1", "mock.core.inventory.read")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Namespace != "mock.core" {
		t.Fatalf("resolved namespace = %q, want mock.core", resolved.Namespace)
	}
}

func TestNamespaceRegistryResolveDoesNotTreatWholeMethodAsNamespace(t *testing.T) {
	ctx := context.Background()
	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{})
	if err := reg.Register(ctx, rpc.Route{
		RuntimeID: "runtime-1", PluginID: "plugin-1", ServiceID: "service-whole", Namespace: "mock.core.read",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := reg.Resolve(ctx, "runtime-1", "mock.core.read"); err == nil {
		t.Fatal("full method must not be treated as a namespace; a method suffix is required")
	}
}
