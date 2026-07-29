package registry

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func makeContribution(id string, kind domain.ContributionKind, ext string, mod string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:          domain.ContributionID(id),
		ModuleID:    domain.ModuleID(mod),
		ExtensionID: domain.ExtensionID(ext),
		Kind:        kind,
		Name:        domain.LocalizedText{Default: id},
		Definition:  map[string]any{},
	}
}

func TestRegisterBatch(t *testing.T) {
	r := NewDefaultRegistry()
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
			makeContribution("skill1", domain.ContributionKindAgentSkill, "com.example/test", "main"),
		},
		Source: "test",
	}
	result := r.RegisterBatch(context.Background(), batch)
	if len(result.Registered) != 2 {
		t.Errorf("expected 2 registered, got %d", len(result.Registered))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}
}

func TestRegisterBatchDuplicate(t *testing.T) {
	r := NewDefaultRegistry()
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
		},
	}
	r.RegisterBatch(context.Background(), batch)
	result := r.RegisterBatch(context.Background(), batch)
	if len(result.Errors) == 0 {
		t.Errorf("expected duplicate error")
	}
}

func TestRegisterBatchReplaceExisting(t *testing.T) {
	r := NewDefaultRegistry()
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
		},
		ReplaceExisting: true,
	}
	r.RegisterBatch(context.Background(), batch)
	result := r.RegisterBatch(context.Background(), batch)
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors with replace, got %v", result.Errors)
	}
}

func TestGetAndList(t *testing.T) {
	r := NewDefaultRegistry()
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
			makeContribution("skill1", domain.ContributionKindAgentSkill, "com.example/test", "main"),
			makeContribution("tool2", domain.ContributionKindTool, "com.example/test", "main"),
		},
	}
	r.RegisterBatch(context.Background(), batch)
	got, err := r.Get(context.Background(), "tool1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Definition.ID != "tool1" {
		t.Errorf("expected tool1, got %s", got.Definition.ID)
	}
	all, _ := r.List(context.Background(), ContributionFilter{Kind: domain.ContributionKindTool})
	if len(all) != 2 {
		t.Errorf("expected 2 tools, got %d", len(all))
	}
	byExt, _ := r.List(context.Background(), ContributionFilter{ExtensionID: "com.example/test"})
	if len(byExt) != 3 {
		t.Errorf("expected 3 by extension, got %d", len(byExt))
	}
}

func TestActivateDeactivate(t *testing.T) {
	r := NewDefaultRegistry()
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
		},
	}
	r.RegisterBatch(context.Background(), batch)
	binding := domain.RuntimeBinding{
		RuntimeID:   "rt1",
		RuntimeType: domain.RuntimeTypeBuiltin,
		Generation:  1,
		InstanceID:  "inst1",
	}
	if err := r.Activate(context.Background(), "tool1", binding); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	got, _ := r.Get(context.Background(), "tool1")
	if got.ActivationState != "active" {
		t.Errorf("expected active, got %s", got.ActivationState)
	}
	if got.RuntimeBinding == nil {
		t.Errorf("expected runtime binding")
	}
	active, _ := r.List(context.Background(), ContributionFilter{ActiveOnly: true})
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
	if err := r.Deactivate(context.Background(), "tool1"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	got, _ = r.Get(context.Background(), "tool1")
	if got.ActivationState != "inactive" {
		t.Errorf("expected inactive, got %s", got.ActivationState)
	}
}

func TestUnregisterBatch(t *testing.T) {
	r := NewDefaultRegistry()
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
			makeContribution("tool2", domain.ContributionKindTool, "com.example/test", "main"),
		},
	}
	r.RegisterBatch(context.Background(), batch)
	result := r.UnregisterBatch(context.Background(), ContributionUnregisterRequest{
		ExtensionID:   "com.example/test",
		Contributions: []domain.ContributionID{"tool1"},
	})
	if len(result.Unregistered) != 1 {
		t.Errorf("expected 1 unregistered, got %d", len(result.Unregistered))
	}
	if _, err := r.Get(context.Background(), "tool1"); err == nil {
		t.Errorf("expected error after unregister")
	}
}

func TestReplaceGeneration(t *testing.T) {
	r := NewDefaultRegistry()
	r.RegisterBatch(context.Background(), ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
			makeContribution("tool2", domain.ContributionKindTool, "com.example/test", "main"),
		},
	})
	result := r.ReplaceGeneration(context.Background(), ContributionReplacementRequest{
		ExtensionID:   "com.example/test",
		OldGeneration: 1,
		NewGeneration: 2,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
			makeContribution("tool3", domain.ContributionKindTool, "com.example/test", "main"),
		},
	})
	if len(result.Unregistered) != 2 {
		t.Errorf("expected 2 unregistered, got %d", len(result.Unregistered))
	}
	if len(result.Registered) != 2 {
		t.Errorf("expected 2 registered, got %d", len(result.Registered))
	}
	got, _ := r.Get(context.Background(), "tool1")
	if got.Generation != 2 {
		t.Errorf("expected gen 2, got %d", got.Generation)
	}
	if _, err := r.Get(context.Background(), "tool2"); err == nil {
		t.Errorf("expected tool2 unregistered")
	}
	if _, err := r.Get(context.Background(), "tool3"); err != nil {
		t.Errorf("expected tool3 registered")
	}
}

func TestDiff(t *testing.T) {
	r := NewDefaultRegistry()
	r.RegisterBatch(context.Background(), ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
		},
	})
	r.RegisterBatch(context.Background(), ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  2,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
			makeContribution("tool2", domain.ContributionKindTool, "com.example/test", "main"),
		},
		ReplaceExisting: true,
	})
	diff := r.Diff(context.Background(), 1, 2)
	if len(diff.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(diff.Added))
	}
}

func TestRebuild(t *testing.T) {
	r := NewDefaultRegistry()
	r.RegisterBatch(context.Background(), ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
		},
	})
	if err := r.Rebuild(context.Background(), nil); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	all, _ := r.List(context.Background(), ContributionFilter{})
	if len(all) != 0 {
		t.Errorf("expected 0 after rebuild")
	}
}

func TestRegisterAdapter(t *testing.T) {
	r := NewDefaultRegistry()
	adapter := &fakeAdapter{kind: domain.ContributionKindTool}
	r.RegisterAdapter(adapter)
	batch := ContributionRegistrationBatch{
		ExtensionID: "com.example/test",
		ModuleID:    "main",
		Generation:  1,
		Contributions: []domain.ContributionDefinition{
			makeContribution("tool1", domain.ContributionKindTool, "com.example/test", "main"),
		},
	}
	r.RegisterBatch(context.Background(), batch)
	if adapter.registerCount != 1 {
		t.Errorf("expected 1 register call, got %d", adapter.registerCount)
	}
}

type fakeAdapter struct {
	kind            domain.ContributionKind
	registerCount   int
	activateCount   int
	deactivateCount int
	unregisterCount int
}

func (a *fakeAdapter) Kind() domain.ContributionKind { return a.kind }
func (a *fakeAdapter) OnRegister(_ context.Context, _ RegisteredContribution) error {
	a.registerCount++
	return nil
}
func (a *fakeAdapter) OnActivate(_ context.Context, _ RegisteredContribution) error {
	a.activateCount++
	return nil
}
func (a *fakeAdapter) OnDeactivate(_ context.Context, _ RegisteredContribution) error {
	a.deactivateCount++
	return nil
}
func (a *fakeAdapter) OnUnregister(_ context.Context, _ RegisteredContribution) error {
	a.unregisterCount++
	return nil
}
