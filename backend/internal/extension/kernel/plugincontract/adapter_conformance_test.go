package plugincontract

import (
	"context"
	"encoding/json"
	"testing"
)

type conformanceRegistration struct {
	id string
}

func (r conformanceRegistration) ID() string {
	return r.id
}

type conformanceHandler struct {
	steps []string
}

func (h *conformanceHandler) Build(context.Context, ModuleRef, ContributionDescriptor) (ContributionRegistration, error) {
	h.steps = append(h.steps, "build")
	return conformanceRegistration{id: "registration"}, nil
}

func (h *conformanceHandler) Activate(context.Context, ContributionRegistration) error {
	h.steps = append(h.steps, "activate")
	return nil
}

func (h *conformanceHandler) Deactivate(context.Context, ContributionRegistration) error {
	h.steps = append(h.steps, "deactivate")
	return nil
}

func (h *conformanceHandler) Discard(context.Context, ContributionRegistration) error {
	h.steps = append(h.steps, "discard")
	return nil
}

func TestAdapterConformanceRunsCompleteLifecycle(t *testing.T) {
	descriptor := AdapterDescriptor{
		ID:                 "conformance.adapter",
		ContractVersion:    1,
		SupportedKinds:     []string{"conformance_kind"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              TrustUserTrusted,
	}
	registry := NewAdapterRegistry(AdapterRegistrationPolicy{MinTrust: TrustUserTrusted})
	if err := registry.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	router := NewContributionRouter(registry)
	handler := &conformanceHandler{}
	if err := router.Register(descriptor.ID, descriptor.ContractVersion, handler); err != nil {
		t.Fatal(err)
	}
	report := RunAdapterConformance(context.Background(), registry, router, AdapterConformanceCase{
		Descriptor: descriptor,
		Module:     ModuleRef{PluginID: "com.example/conformance", ModuleID: "main"},
		Contribution: ContributionDescriptor{
			ID:              "conformance",
			Kind:            "conformance_kind",
			Adapter:         descriptor.ID,
			ContractVersion: descriptor.ContractVersion,
			Name:            LocalizedText{Default: "Conformance"},
		},
	})
	if !report.Valid() {
		t.Fatal(report.Err())
	}
	expected := []string{"build", "activate", "deactivate", "discard"}
	if len(handler.steps) != len(expected) {
		t.Fatalf("steps = %#v", handler.steps)
	}
	for index := range expected {
		if handler.steps[index] != expected[index] {
			t.Fatalf("steps = %#v", handler.steps)
		}
	}
}
