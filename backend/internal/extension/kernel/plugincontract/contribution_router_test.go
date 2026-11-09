package plugincontract

import (
	"context"
	"errors"
	"testing"
)

type testContributionRegistration struct {
	id string
}

func (r *testContributionRegistration) ID() string {
	return r.id
}

type testContributionHandler struct {
	buildCount      int
	activateCount   int
	deactivateCount int
	discardCount    int
	buildErr        error
}

func (h *testContributionHandler) Build(_ context.Context, _ ModuleRef, contribution ContributionDescriptor) (ContributionRegistration, error) {
	h.buildCount++
	if h.buildErr != nil {
		return nil, h.buildErr
	}
	return &testContributionRegistration{id: contribution.ID}, nil
}

func (h *testContributionHandler) Activate(context.Context, ContributionRegistration) error {
	h.activateCount++
	return nil
}

func (h *testContributionHandler) Deactivate(context.Context, ContributionRegistration) error {
	h.deactivateCount++
	return nil
}

func (h *testContributionHandler) Discard(context.Context, ContributionRegistration) error {
	h.discardCount++
	return nil
}

func TestContributionRouterValidatesAdapterAndKind(t *testing.T) {
	adapters, err := NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatal(err)
	}
	router := NewContributionRouter(adapters)
	handler := &testContributionHandler{}
	if err := router.Register("tool", 1, handler); err != nil {
		t.Fatal(err)
	}
	contribution := ContributionDescriptor{
		ID:              "command",
		Kind:            "tool",
		Adapter:         "tool",
		ContractVersion: 1,
		Name:            LocalizedText{Default: "Command"},
	}
	module := ModuleRef{PluginID: "com.example/plugin", ModuleID: "main"}
	registration, err := router.BuildAndActivate(context.Background(), module, contribution)
	if err != nil {
		t.Fatal(err)
	}
	if handler.buildCount != 1 || handler.activateCount != 1 {
		t.Fatalf("unexpected handler counts: %#v", handler)
	}
	if err := router.DeactivateAndDiscard(context.Background(), module, contribution, registration); err != nil {
		t.Fatal(err)
	}
	if handler.deactivateCount != 1 || handler.discardCount != 1 {
		t.Fatalf("unexpected cleanup counts: %#v", handler)
	}
	contribution.Kind = "ui_page"
	if _, err := router.Resolve(contribution); err == nil {
		t.Fatal("adapter accepted unsupported kind")
	}
}

func TestContributionRouterDoesNotMutateRouteOnBuildFailure(t *testing.T) {
	adapters, err := NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatal(err)
	}
	router := NewContributionRouter(adapters)
	handler := &testContributionHandler{buildErr: errors.New("build failed")}
	if err := router.Register("tool", 1, handler); err != nil {
		t.Fatal(err)
	}
	contribution := ContributionDescriptor{
		ID:              "command",
		Kind:            "tool",
		Adapter:         "tool",
		ContractVersion: 1,
		Name:            LocalizedText{Default: "Command"},
	}
	if _, err := router.BuildAndActivate(context.Background(), ModuleRef{PluginID: "com.example/plugin", ModuleID: "main"}, contribution); err == nil {
		t.Fatal("build failure was ignored")
	}
	if resolved, err := router.Resolve(contribution); err != nil || resolved != handler {
		t.Fatalf("route was mutated after build failure: handler=%v err=%v", resolved, err)
	}
}
