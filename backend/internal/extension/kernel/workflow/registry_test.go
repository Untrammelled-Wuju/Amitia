package workflow

import "testing"

func trueWorkflowPtr() *bool { v := true; return &v }

func TestWorkflowRegistryRegister(t *testing.T) {
	reg := NewWorkflowRegistry()

	def := WorkflowDefinition{
		ID:   "wf-001",
		Name: "test-wf",
	}

	if err := reg.Register(def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 workflow, got %d", reg.Count())
	}

	if err := reg.Register(def); err == nil {
		t.Fatal("expected duplicate error")
	}

	retrieved, ok := reg.Get(def.ID)
	if !ok {
		t.Fatal("expected found")
	}
	if retrieved.Name != "test-wf" {
		t.Fatalf("expected name test-wf, got %s", retrieved.Name)
	}
}

func TestWorkflowRegistryCallable(t *testing.T) {
	reg := NewWorkflowRegistry()

	_ = reg.Register(WorkflowDefinition{ID: "wf-call", Name: "callable", CallableByAgent: true, Enabled: true})
	_ = reg.Register(WorkflowDefinition{ID: "wf-no-call", Name: "no-call", CallableByAgent: false, Enabled: true})

	all := reg.List(WorkflowFilter{})
	if len(all) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(all))
	}

	callable := reg.List(WorkflowFilter{CallableByAgent: trueWorkflowPtr()})
	if len(callable) != 1 {
		t.Fatalf("expected 1 callable workflow, got %d", len(callable))
	}
	if callable[0].ID != "wf-call" {
		t.Fatalf("expected wf-call, got %s", callable[0].ID)
	}
}

func TestWorkflowRegistryUnregister(t *testing.T) {
	reg := NewWorkflowRegistry()

	_ = reg.Register(WorkflowDefinition{ID: "wf-del", Name: "delete-me"})

	if err := reg.Unregister("wf-del"); err != nil {
		t.Fatalf("unexpected Unregister error: %v", err)
	}

	if _, ok := reg.Get("wf-del"); ok {
		t.Fatal("expected not found")
	}
}
