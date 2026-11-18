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
		DispatchKind:    DispatchList,
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
	if chat.Scope != ScopeSessionMaybe {
		t.Fatalf("chat scope = %q, want %q", chat.Scope, ScopeSessionMaybe)
	}
	root, err := r.Get("extension.detail.tab")
	if err != nil {
		t.Fatal(err)
	}
	if root.Scope != ScopeRoot {
		t.Fatalf("root scope = %q, want %q", root.Scope, ScopeRoot)
	}
	rightPane, err := r.Get("ui.sidebar.right.pane.tab")
	if err != nil {
		t.Fatal(err)
	}
	if rightPane.Scope != ScopeSessionMaybe || rightPane.ParentSlotID != "root" || rightPane.DispatchKind != DispatchList {
		t.Fatalf("right pane slot contract = %+v", rightPane)
	}
	child := dynamicTestSlot("scope.child", "chat.sidebar.panel", "scope.ext")
	if err := r.RegisterOwned("scope.ext", child); err != nil {
		t.Fatal(err)
	}
	registered, err := r.Get("scope.child")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Scope != ScopeSessionMaybe {
		t.Fatalf("child scope = %q, want inherited %q", registered.Scope, ScopeSessionMaybe)
	}
}

func TestSessionSlotRejectsRootChild(t *testing.T) {
	r := DefaultSlotRegistry()
	child := dynamicTestSlot("scope.escape", "chat.message.action", "scope.ext")
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
	if composer.DispatchKind != DispatchChain {
		t.Fatalf("composer dispatch kind = %q, want %q", composer.DispatchKind, DispatchChain)
	}

	action, err := r.Get("chat.composer.action")
	if err != nil {
		t.Fatalf("get chat composer action: %v", err)
	}
	if action.ParentSlotID != "provider.conversation.composer" {
		t.Fatalf("chat composer action parent = %q, want provider.conversation.composer", action.ParentSlotID)
	}
	if action.Scope != ScopeSessionMaybe {
		t.Fatalf("chat composer action scope = %q, want %q", action.Scope, ScopeSessionMaybe)
	}

	dockTop, err := r.Get("chat.composer.dock.top")
	if err != nil {
		t.Fatalf("get chat composer dock top: %v", err)
	}
	if dockTop.ParentSlotID != "provider.conversation.composer" || dockTop.Scope != ScopeSession || dockTop.DispatchKind != DispatchList {
		t.Fatalf("chat composer dock top contract = parent %q scope %q kind %q, want provider.conversation.composer/session/list", dockTop.ParentSlotID, dockTop.Scope, dockTop.DispatchKind)
	}

	model, err := r.Get("chat.composer.model")
	if err != nil {
		t.Fatalf("get chat composer model: %v", err)
	}
	if model.ParentSlotID != "provider.conversation.composer" || model.Scope != ScopeSession || model.DispatchKind != DispatchSingle {
		t.Fatalf("chat composer model contract = parent %q scope %q kind %q, want provider.conversation.composer/session/single", model.ParentSlotID, model.Scope, model.DispatchKind)
	}

	modelLabel, err := r.Get("chat.composer.model.label")
	if err != nil {
		t.Fatalf("get chat composer model label: %v", err)
	}
	if modelLabel.ParentSlotID != "chat.composer.model" || modelLabel.Scope != ScopeSession || modelLabel.DispatchKind != DispatchSingle {
		t.Fatalf("chat composer model label contract = parent %q scope %q kind %q, want chat.composer.model/session/single", modelLabel.ParentSlotID, modelLabel.Scope, modelLabel.DispatchKind)
	}

	modelOptions, err := r.Get("chat.composer.model.options")
	if err != nil {
		t.Fatalf("get chat composer model options: %v", err)
	}
	if modelOptions.ParentSlotID != "chat.composer.model" || modelOptions.Scope != ScopeSession || modelOptions.DispatchKind != DispatchList {
		t.Fatalf("chat composer model options contract = parent %q scope %q kind %q, want chat.composer.model/session/list", modelOptions.ParentSlotID, modelOptions.Scope, modelOptions.DispatchKind)
	}

	permission, err := r.Get("chat.composer.permission")
	if err != nil {
		t.Fatalf("get chat composer permission: %v", err)
	}
	if permission.ParentSlotID != "provider.conversation.composer" || permission.Scope != ScopeSession || permission.DispatchKind != DispatchSingle {
		t.Fatalf("chat composer permission contract = parent %q scope %q kind %q, want provider.conversation.composer/session/single", permission.ParentSlotID, permission.Scope, permission.DispatchKind)
	}

	messageRenderer, err := r.Get("provider.conversation.message_renderer")
	if err != nil {
		t.Fatalf("get provider message renderer: %v", err)
	}
	if messageRenderer.Scope != ScopeSession || messageRenderer.DispatchKind != DispatchChain {
		t.Fatalf("message renderer contract = scope %q kind %q, want session/chain", messageRenderer.Scope, messageRenderer.DispatchKind)
	}

	messageImages, err := r.Get("chat.message.images")
	if err != nil {
		t.Fatalf("get assistant message images: %v", err)
	}
	if messageImages.ParentSlotID != "provider.conversation.messages" || messageImages.Scope != ScopeSession || messageImages.DispatchKind != DispatchSingle {
		t.Fatalf("assistant message images contract = parent %q scope %q kind %q, want provider.conversation.messages/session/single", messageImages.ParentSlotID, messageImages.Scope, messageImages.DispatchKind)
	}

	userMessageImages, err := r.Get("chat.message.user.images")
	if err != nil {
		t.Fatalf("get user message images: %v", err)
	}
	if userMessageImages.ParentSlotID != "provider.conversation.messages" || userMessageImages.Scope != ScopeSession || userMessageImages.DispatchKind != DispatchSingle {
		t.Fatalf("user message images contract = parent %q scope %q kind %q, want provider.conversation.messages/session/single", userMessageImages.ParentSlotID, userMessageImages.Scope, userMessageImages.DispatchKind)
	}

	quotaNotice, err := r.Get("app.shell.quota_notice")
	if err != nil {
		t.Fatalf("get shell quota notice: %v", err)
	}
	if quotaNotice.ParentSlotID != "root" || quotaNotice.Scope != ScopeRoot || quotaNotice.DispatchKind != DispatchChain {
		t.Fatalf("shell quota notice contract = parent %q scope %q kind %q, want root/root/chain", quotaNotice.ParentSlotID, quotaNotice.Scope, quotaNotice.DispatchKind)
	}

	shellBottom, err := r.Get("app.shell.bottom")
	if err != nil {
		t.Fatalf("get shell bottom: %v", err)
	}
	if shellBottom.ParentSlotID != "root" || shellBottom.Scope != ScopeRoot || shellBottom.DispatchKind != DispatchSingle {
		t.Fatalf("shell bottom contract = parent %q scope %q kind %q, want root/root/single", shellBottom.ParentSlotID, shellBottom.Scope, shellBottom.DispatchKind)
	}

	for _, slotID := range []SlotID{"desktop.context_menu.content", "desktop.context_menu.icon"} {
		slot, err := r.Get(slotID)
		if err != nil {
			t.Fatalf("get %s: %v", slotID, err)
		}
		if slot.ParentSlotID != "root" || slot.Scope != ScopeRoot || slot.DispatchKind != DispatchKeyed {
			t.Fatalf("%s contract = parent %q scope %q kind %q, want root/root/keyed", slotID, slot.ParentSlotID, slot.Scope, slot.DispatchKind)
		}
		if len(slot.Platform) != 4 {
			t.Fatalf("%s platforms = %#v, want four desktop/web platforms", slotID, slot.Platform)
		}
	}

	for _, slotID := range []SlotID{"workbench.file.actions", "workbench.review.file.actions"} {
		slot, err := r.Get(slotID)
		if err != nil {
			t.Fatalf("get %s: %v", slotID, err)
		}
		if slot.ParentSlotID != "provider.conversation.shell" || slot.Scope != ScopeSession || slot.DispatchKind != DispatchList {
			t.Fatalf("%s contract = parent %q scope %q kind %q, want provider.conversation.shell/session/list", slotID, slot.ParentSlotID, slot.Scope, slot.DispatchKind)
		}
		if len(slot.Platform) != 4 {
			t.Fatalf("%s platforms = %#v, want four desktop/web platforms", slotID, slot.Platform)
		}
	}

	modelProvider, err := r.Get("settings.model.provider")
	if err != nil {
		t.Fatalf("get model provider settings slot: %v", err)
	}
	if modelProvider.ParentSlotID != "root" || modelProvider.Scope != ScopeRoot || modelProvider.DispatchKind != DispatchKeyed {
		t.Fatalf("model provider settings contract = parent %q scope %q kind %q, want root/root/keyed", modelProvider.ParentSlotID, modelProvider.Scope, modelProvider.DispatchKind)
	}

	modelFooter, err := r.Get("settings.model.footer")
	if err != nil {
		t.Fatalf("get model footer settings slot: %v", err)
	}
	if modelFooter.ParentSlotID != "root" || modelFooter.Scope != ScopeRoot || modelFooter.DispatchKind != DispatchList {
		t.Fatalf("model footer settings contract = parent %q scope %q kind %q, want root/root/list", modelFooter.ParentSlotID, modelFooter.Scope, modelFooter.DispatchKind)
	}

	documentBody, err := r.Get("ui.sidebar.right.pane.document")
	if err != nil {
		t.Fatalf("get right pane document body: %v", err)
	}
	if documentBody.ParentSlotID != "ui.sidebar.right.pane.tab" || documentBody.Scope != ScopeSession || documentBody.DispatchKind != DispatchKeyed {
		t.Fatalf("right pane document body contract = parent %q scope %q kind %q, want ui.sidebar.right.pane.tab/session/keyed", documentBody.ParentSlotID, documentBody.Scope, documentBody.DispatchKind)
	}

	for _, slotID := range []SlotID{
		"ui.sidebar.right.pane.document.action",
		"ui.sidebar.right.pane.document.actions",
		"ui.sidebar.right.pane.document.pdf",
		"ui.sidebar.right.pane.document.unpreviewable",
	} {
		slot, err := r.Get(slotID)
		if err != nil {
			t.Fatalf("get %s: %v", slotID, err)
		}
		if slot.ParentSlotID != "ui.sidebar.right.pane.document" || slot.Scope != ScopeSession {
			t.Fatalf("%s contract = parent %q scope %q, want ui.sidebar.right.pane.document/session", slotID, slot.ParentSlotID, slot.Scope)
		}
		if len(slot.Platform) != 4 {
			t.Fatalf("%s platforms = %#v, want four desktop/web platforms", slotID, slot.Platform)
		}
	}
	for _, expected := range []struct {
		slotID     SlotID
		parentSlot SlotID
		scope      SlotScope
		dispatch   SlotDispatchKind
	}{
		{"ui.sidebar.right.pane.files.actions", "ui.sidebar.right.pane.tab", ScopeSession, DispatchList},
		{"ui.sidebar.right.pane.guide", "ui.sidebar.right.pane.tab", ScopeSession, DispatchChain},
		{"ui.sidebar.right.pane.guide.actions", "ui.sidebar.right.pane.guide", ScopeRoot, DispatchList},
		{"ui.sidebar.right.pane.guide.entry", "ui.sidebar.right.pane.guide", ScopeSession, DispatchKeyed},
		{"ui.sidebar.right.pane.tab.menu.item", "ui.sidebar.right.pane.tab", ScopeSession, DispatchList},
		{"app.rightbar", "root", ScopeRoot, DispatchSingle},
		{"app.rightbar.session", "app.rightbar", ScopeSession, DispatchSingle},
		{"settings.shell", "root", ScopeRoot, DispatchSingle},
		{"settings.shell.header", "settings.shell", ScopeRoot, DispatchSingle},
		{"settings.shell.action", "settings.shell", ScopeRoot, DispatchList},
		{"settings.shell.close", "settings.shell", ScopeRoot, DispatchSingle},
		{"settings.shell.launcher", "settings.shell", ScopeRoot, DispatchSingle},
		{"settings.shell.launcher.notifications", "settings.shell.launcher", ScopeRoot, DispatchSingle},
		{"settings.onboarding", "settings.shell", ScopeRoot, DispatchList},
		{"settings.section.icon", "settings.shell", ScopeRoot, DispatchList},
		{"settings.shell.trigger", "settings.shell", ScopeRoot, DispatchSingle},
		{"settings.model.sign-in", "settings.onboarding", ScopeRoot, DispatchSingle},
		{"extension.plugin.tab", "root", ScopeRoot, DispatchList},
		{"extension.plugin.item", "root", ScopeRoot, DispatchList},
		{"extension.plugin.bundle.activation", "root", ScopeRoot, DispatchKeyed},
		{"extension.plugin.bundle.config", "root", ScopeRoot, DispatchKeyed},
		{"extension.settings.tab", "root", ScopeRoot, DispatchList},
	} {
		slot, err := r.Get(expected.slotID)
		if err != nil {
			t.Fatalf("get %s: %v", expected.slotID, err)
		}
		if slot.ParentSlotID != expected.parentSlot || slot.Scope != expected.scope || slot.DispatchKind != expected.dispatch {
			t.Fatalf("%s contract = parent %q scope %q kind %q, want %q/%q/%q", expected.slotID, slot.ParentSlotID, slot.Scope, slot.DispatchKind, expected.parentSlot, expected.scope, expected.dispatch)
		}
		if len(slot.Platform) != 4 {
			t.Fatalf("%s platforms = %#v, want four desktop/web platforms", expected.slotID, slot.Platform)
		}
	}
	guideActions, err := r.Get("ui.sidebar.right.pane.guide.actions")
	if err != nil {
		t.Fatalf("get guide actions: %v", err)
	}
	if !guideActions.AllowRootChild || guideActions.Scope != ScopeRoot || guideActions.ParentSlotID != "ui.sidebar.right.pane.guide" {
		t.Fatalf("guide actions scope escape contract = %+v", guideActions)
	}

	desktopCommand, err := r.Get("desktop.command")
	if err != nil {
		t.Fatalf("get desktop command: %v", err)
	}
	if desktopCommand.DispatchKind != DispatchKeyed {
		t.Fatalf("desktop command dispatch kind = %q, want %q", desktopCommand.DispatchKind, DispatchKeyed)
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
	if composer.DispatchKind != DispatchChain {
		t.Fatalf("snapshot contract lost dispatch kind: got %q", composer.DispatchKind)
	}
}

func TestDefaultDispatchContracts(t *testing.T) {
	r := DefaultSlotRegistry()
	cases := []struct {
		slot string
		kind SlotDispatchKind
	}{
		{"provider.conversation.overlay", DispatchList},
		{"extension.settings.page", DispatchList},
		{"chat.message.custom_renderer", DispatchChain},
		{"chat.message.attachment_renderer", DispatchChain},
		{"desktop.window.page", DispatchKeyed},
		{"ui.portal.overlay", DispatchList},
		{"ui.portal.modal", DispatchList},
		{"ui.portal.drawer", DispatchList},
		{"ui.portal.toast", DispatchList},
	}
	for _, tc := range cases {
		def, err := r.Get(SlotID(tc.slot))
		if err != nil {
			t.Fatalf("get %s: %v", tc.slot, err)
		}
		if def.DispatchKind != tc.kind {
			t.Fatalf("%s contract kind = %q, want %q", tc.slot, def.DispatchKind, tc.kind)
		}
	}
}

func TestRegisterValidatesAndDerivesDispatchKind(t *testing.T) {
	r := DefaultSlotRegistry()
	def := dynamicTestSlot("dispatch.chain", "chat.sidebar.panel", "dispatch.ext")
	def.DispatchKind = DispatchChain
	if err := r.RegisterOwned("dispatch.ext", def); err != nil {
		t.Fatalf("register chain: %v", err)
	}
	registered, err := r.Get("dispatch.chain")
	if err != nil {
		t.Fatal(err)
	}
	if registered.DispatchKind != DispatchChain {
		t.Fatalf("dispatch contract kind = %q", registered.DispatchKind)
	}

	hoc := dynamicTestSlot("dispatch.hoc", "chat.sidebar.panel", "dispatch.ext")
	hoc.DispatchKind = DispatchHoc
	if err := r.RegisterOwned("dispatch.ext", hoc); err != nil {
		t.Fatalf("register hoc: %v", err)
	}
	registeredHoc, err := r.Get("dispatch.hoc")
	if err != nil {
		t.Fatal(err)
	}
	if registeredHoc.DispatchKind != DispatchHoc {
		t.Fatalf("dispatch hoc contract kind = %q", registeredHoc.DispatchKind)
	}

	invalid := dynamicTestSlot("dispatch.invalid", "chat.sidebar.panel", "dispatch.ext")
	invalid.DispatchKind = SlotDispatchKind("unknown")
	if err := r.RegisterOwned("dispatch.ext", invalid); !errors.Is(err, ErrInvalidSlotDispatchKind) {
		t.Fatalf("invalid dispatch error = %v, want %v", err, ErrInvalidSlotDispatchKind)
	}
}
