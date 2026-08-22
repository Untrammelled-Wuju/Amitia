package extension_slots

import (
	"context"
	"errors"
	"testing"
)

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

func TestDeclarationEpochIncrementsAcrossRedeclare(t *testing.T) {
	r := DefaultSlotRegistry()
	def := dynamicTestSlot("epoch.slot", "chat.sidebar.panel", "epoch.ext")
	if err := r.RegisterOwned("epoch.ext", def); err != nil {
		t.Fatal(err)
	}
	first, err := r.Get("epoch.slot")
	if err != nil {
		t.Fatal(err)
	}
	if first.DeclarationEpoch == 0 {
		t.Fatalf("first declaration must have a non-zero epoch")
	}
	if _, err := r.Unregister("epoch.slot"); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterOwned("epoch.ext", def); err != nil {
		t.Fatal(err)
	}
	second, err := r.Get("epoch.slot")
	if err != nil {
		t.Fatal(err)
	}
	if second.DeclarationEpoch <= first.DeclarationEpoch {
		t.Fatalf("redeclared slot epoch = %d, want > %d", second.DeclarationEpoch, first.DeclarationEpoch)
	}
}

func TestRestoredChildGetsNewDeclarationEpoch(t *testing.T) {
	r := DefaultSlotRegistry()
	parent := dynamicTestSlot("epoch.parent", "chat.sidebar.panel", "parent.ext")
	child := dynamicTestSlot("epoch.child", "epoch.parent", "child.ext")
	if err := r.RegisterOwned("parent.ext", parent); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterOwned("child.ext", child); err != nil {
		t.Fatal(err)
	}
	before, err := r.Get("epoch.child")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Unregister("epoch.parent"); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterOwned("parent.ext", parent); err != nil {
		t.Fatal(err)
	}
	after, err := r.Get("epoch.child")
	if err != nil {
		t.Fatal(err)
	}
	if after.DeclarationEpoch <= before.DeclarationEpoch {
		t.Fatalf("restored child epoch = %d, want > %d", after.DeclarationEpoch, before.DeclarationEpoch)
	}
}

func TestSlotScopeDefaultsAndInheritance(t *testing.T) {
	r := DefaultSlotRegistry()
	chat, err := r.Get("chat.sidebar.panel")
	if err != nil {
		t.Fatal(err)
	}
	if chat.Scope != ScopeSession {
		t.Fatalf("chat scope = %q, want %q", chat.Scope, ScopeSession)
	}
	root, err := r.Get("extension.detail.tab")
	if err != nil {
		t.Fatal(err)
	}
	if root.Scope != ScopeRoot {
		t.Fatalf("root scope = %q, want %q", root.Scope, ScopeRoot)
	}
	child := dynamicTestSlot("scope.child", "chat.sidebar.panel", "scope.ext")
	if err := r.RegisterOwned("scope.ext", child); err != nil {
		t.Fatal(err)
	}
	registered, err := r.Get("scope.child")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Scope != ScopeSession {
		t.Fatalf("child scope = %q, want inherited %q", registered.Scope, ScopeSession)
	}
}

func TestSessionSlotRejectsRootChild(t *testing.T) {
	r := DefaultSlotRegistry()
	child := dynamicTestSlot("scope.escape", "chat.sidebar.panel", "scope.ext")
	child.Scope = ScopeRoot
	if err := r.RegisterOwned("scope.ext", child); !errors.Is(err, ErrSlotScopeEscape) {
		t.Fatalf("register root child under session slot error = %v, want %v", err, ErrSlotScopeEscape)
	}
}

func TestSessionMaybeScopeCanNarrowButNotEscape(t *testing.T) {
	r := DefaultSlotRegistry()
	parent := dynamicTestSlot("scope.maybe", "extension.detail.tab", "scope.ext")
	parent.Scope = ScopeSessionMaybe
	if err := r.RegisterOwned("scope.ext", parent); err != nil {
		t.Fatal(err)
	}
	strict := dynamicTestSlot("scope.maybe.strict", "scope.maybe", "strict.ext")
	strict.Scope = ScopeSession
	if err := r.RegisterOwned("strict.ext", strict); err != nil {
		t.Fatalf("session child under session-maybe failed: %v", err)
	}
	escape := dynamicTestSlot("scope.maybe.escape", "scope.maybe", "escape.ext")
	escape.Scope = ScopeRoot
	if err := r.RegisterOwned("escape.ext", escape); !errors.Is(err, ErrSlotScopeEscape) {
		t.Fatalf("root child under session-maybe error = %v, want %v", err, ErrSlotScopeEscape)
	}
}

type emptySlotResolver struct{}

func (emptySlotResolver) Resolve(context.Context, SlotID) ([]*ContributionSummary, error) {
	return nil, nil
}

func TestDefaultProviderSlotsFormRootedTree(t *testing.T) {
	r := DefaultSlotRegistry()

	root, err := r.Get("root")
	if err != nil {
		t.Fatalf("get root slot: %v", err)
	}
	if root.ParentSlotID != "" || root.Scope != ScopeRoot {
		t.Fatalf("root contract = parent %q scope %q, want no parent and root scope", root.ParentSlotID, root.Scope)
	}

	composer, err := r.Get("provider.conversation.composer")
	if err != nil {
		t.Fatalf("get provider conversation composer: %v", err)
	}
	if composer.ParentSlotID != "provider.conversation.shell" {
		t.Fatalf("composer parent = %q, want provider.conversation.shell", composer.ParentSlotID)
	}
	if composer.Scope != ScopeSessionMaybe {
		t.Fatalf("composer scope = %q, want %q", composer.Scope, ScopeSessionMaybe)
	}
	if composer.Multiplicity != MultiplicityReplaceableSingle {
		t.Fatalf("composer multiplicity = %q, want %q", composer.Multiplicity, MultiplicityReplaceableSingle)
	}

	action, err := r.Get("chat.composer.action")
	if err != nil {
		t.Fatalf("get chat composer action: %v", err)
	}
	if action.ParentSlotID != "provider.conversation.composer" {
		t.Fatalf("chat composer action parent = %q, want provider.conversation.composer", action.ParentSlotID)
	}
	if action.Scope != ScopeSession {
		t.Fatalf("chat composer action scope = %q, want %q", action.Scope, ScopeSession)
	}
}

func TestSnapshotIncludesSlotContractMetadata(t *testing.T) {
	r := DefaultSlotRegistry()
	svc := NewSnapshotService(r, emptySlotResolver{})
	snap, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}

	var composer *SlotSnapshot
	for _, slot := range snap.Slots {
		if slot.SlotID == "provider.conversation.composer" {
			composer = slot
			break
		}
	}
	if composer == nil {
		t.Fatal("provider.conversation.composer missing from snapshot")
	}
	if composer.ParentSlotID != "provider.conversation.shell" || composer.Scope != ScopeSessionMaybe {
		t.Fatalf("snapshot contract lost parent/scope metadata: %#v", composer)
	}
	if len(composer.SupportedKinds) == 0 {
		t.Fatal("snapshot contract lost supported kinds")
	}
}
