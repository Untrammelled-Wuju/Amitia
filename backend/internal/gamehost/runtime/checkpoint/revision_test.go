package checkpoint

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestComputeDescriptorRevision_Stable(t *testing.T) {
	descriptor := domain.PluginDescriptor{
		ID:          "com.example.test",
		ExtensionID: "ext-1",
		Name:        "Test",
		Version:     "1.0.0",
		Services: []domain.ServiceDescriptor{
			{ID: "svc-a", Name: "Service A", Kind: domain.ServiceKindProcess, Required: true, DependsOn: []domain.ServiceID{"svc-b"}},
			{ID: "svc-b", Name: "Service B", Kind: domain.ServiceKindExternal, Required: false},
		},
		Capabilities: []domain.Capability{"cap-2", "cap-1", "cap-3"},
	}

	first := ComputeDescriptorRevision(descriptor)
	for i := 0; i < 100; i++ {
		next := ComputeDescriptorRevision(descriptor)
		if first != next {
			t.Fatalf("revision not stable: %s vs %s", first, next)
		}
	}
}

func TestComputeDescriptorRevision_OrderIndependent(t *testing.T) {
	descriptor := domain.PluginDescriptor{
		ID:          "com.order.test",
		ExtensionID: "ext-1",
		Name:        "Order",
		Version:     "1.0.0",
		Services: []domain.ServiceDescriptor{
			{ID: "z-service", Name: "Z", Kind: domain.ServiceKindProcess},
			{ID: "a-service", Name: "A", Kind: domain.ServiceKindProcess},
			{ID: "m-service", Name: "M", Kind: domain.ServiceKindProcess},
		},
		Capabilities: []domain.Capability{"z-cap", "a-cap", "m-cap"},
	}

	revision := ComputeDescriptorRevision(descriptor)

	for i := 0; i < 10; i++ {
		current := ComputeDescriptorRevision(descriptor)
		if current != revision {
			t.Fatalf("order-dependent revision: %s vs %s", current, revision)
		}
	}
}

func TestComputeDescriptorRevision_DifferentDescriptors(t *testing.T) {
	descriptors := []domain.PluginDescriptor{
		{ID: "com.a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"},
		{ID: "com.b", ExtensionID: "ext-1", Name: "B", Version: "1.0.0"},
		{ID: "com.a", ExtensionID: "ext-1", Name: "A", Version: "2.0.0"},
		{ID: "com.a", ExtensionID: "ext-2", Name: "A", Version: "1.0.0"},
	}

	revisions := make(map[string]struct{})
	for _, d := range descriptors {
		rev := ComputeDescriptorRevision(d)
		if _, exists := revisions[rev]; exists {
			t.Fatalf("duplicate revision: %s", rev)
		}
		revisions[rev] = struct{}{}
	}
}

func TestComputeDescriptorRevision_WithDependencies(t *testing.T) {
	d1 := domain.PluginDescriptor{
		ID:       "com.dep",
		Services: []domain.ServiceDescriptor{
			{ID: "a", DependsOn: []domain.ServiceID{"b", "c"}},
			{ID: "b", DependsOn: []domain.ServiceID{}},
			{ID: "c", DependsOn: []domain.ServiceID{"b"}},
		},
	}
	d2 := domain.PluginDescriptor{
		ID:       "com.dep",
		Services: []domain.ServiceDescriptor{
			{ID: "a", DependsOn: []domain.ServiceID{"c", "b"}},
			{ID: "b", DependsOn: []domain.ServiceID{}},
			{ID: "c", DependsOn: []domain.ServiceID{"b"}},
		},
	}

	if ComputeDescriptorRevision(d1) != ComputeDescriptorRevision(d2) {
		t.Fatal("dependency order should not affect revision")
	}
}

func TestComputeDescriptorRevision_NotEmpty(t *testing.T) {
	d := domain.PluginDescriptor{
		ID:          "com.empty.test",
		ExtensionID: "ext-1",
		Name:        "Empty",
		Version:     "1.0.0",
	}
	rev := ComputeDescriptorRevision(d)
	if rev == "" {
		t.Fatal("revision should not be empty")
	}
}
