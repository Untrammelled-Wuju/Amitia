package extension_slots

import "testing"

func dynamicTestSlot(id, parent, owner string) *SlotDefinition {
	return &SlotDefinition{
		SlotID:          SlotID(id),
		ContractVersion: 1,
		SupportedKinds:  []string{"panel"},
		Multiplicity:    MultiplicityOrderedMultiple,
		Layout:          LayoutStack,
		FallbackPolicy:  FallbackEmpty,
		ParentSlotID:    SlotID(parent),
		OwnerExtension:  owner,
		Dynamic:         true,
	}
}

func TestDynamicChildSuspendsAndRestoresWithParent(t *testing.T) {
	r := DefaultSlotRegistry()
	parent := dynamicTestSlot("test.parent", "chat.sidebar.panel", "parent.ext")
	if err := r.RegisterOwned("parent.ext", parent); err != nil {
		t.Fatalf("register parent: %v", err)
	}
	child := dynamicTestSlot("test.child", "test.parent", "child.ext")
	if err := r.RegisterOwned("child.ext", child); err != nil {
		t.Fatalf("register child: %v", err)
	}
	removed, err := r.Unregister("test.parent")
	if err != nil {
		t.Fatalf("unregister parent: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected parent and child removed from active graph, got %v", removed)
	}
	if _, err := r.Get("test.child"); err == nil {
		t.Fatalf("child should be inactive while parent is absent")
	}
	if len(r.Suspended()) != 1 || r.Suspended()[0].SlotID != "test.child" {
		t.Fatalf("expected child to be suspended")
	}
	if err := r.RegisterOwned("parent.ext", parent); err != nil {
		t.Fatalf("restore parent: %v", err)
	}
	if _, err := r.Get("test.child"); err != nil {
		t.Fatalf("child should restore automatically: %v", err)
	}
	if len(r.Suspended()) != 0 {
		t.Fatalf("expected suspended graph to drain")
	}
}

func TestUnregisterOwnedDoesNotRestoreRemovedOwners(t *testing.T) {
	r := DefaultSlotRegistry()
	parent := dynamicTestSlot("owned.parent", "chat.sidebar.panel", "parent.ext")
	child := dynamicTestSlot("owned.child", "owned.parent", "child.ext")
	if err := r.RegisterOwned("parent.ext", parent); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterOwned("child.ext", child); err != nil {
		t.Fatal(err)
	}
	r.UnregisterOwned("child.ext")
	if _, err := r.Get("owned.child"); err == nil {
		t.Fatalf("child owned by disabled extension should be gone")
	}
	if len(r.Suspended()) != 0 {
		t.Fatalf("disabled extension must not leave suspended declarations")
	}
}
