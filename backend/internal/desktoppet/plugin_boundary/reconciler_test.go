package plugin_boundary

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type fakeContributionSource struct {
	contribs map[string][]domain.ContributionDefinition
	inst     map[string]domain.ExtensionInstallation
	err      error
}

func newFakeSource() *fakeContributionSource {
	return &fakeContributionSource{
		contribs: make(map[string][]domain.ContributionDefinition),
		inst:     make(map[string]domain.ExtensionInstallation),
	}
}

func (f *fakeContributionSource) ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contribs[string(extID)], nil
}

func (f *fakeContributionSource) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	if f.err != nil {
		return domain.ExtensionInstallation{}, f.err
	}
	return f.inst[string(id)], nil
}

func makeContrib(extID, contribID string, kind domain.ContributionKind, def map[string]any) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:          domain.ContributionID(contribID),
		ModuleID:    "module_pet",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        kind,
		Name:        domain.LocalizedText{Default: contribID},
		Definition:  def,
	}
}

func TestReconcilerHandleEvent_Installed(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	evt := LifecycleEvent{
		Phase:       PhaseInstalled,
		ExtensionID: "com.example/pet",
		OperationID: "op_install",
		Contribution: makeContrib("com.example/pet", "wave_action", domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   "wave",
			"displayName": "Wave",
			"kind":        "builtin",
		}),
	}

	if err := rec.HandleEvent(context.Background(), evt); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ref := ContributionRef{ExtensionID: "com.example/pet", PluginID: "wave_action", ContributionID: "wave_action"}
	reg, ok := rec.Get(ref)
	if !ok {
		t.Fatal("expected contribution registered")
	}
	if reg.Status != ContributionStatusRegistered {
		t.Errorf("status=%s want registered", reg.Status)
	}
	if rec.IsExecutable(ref) {
		t.Error("installed only → not executable")
	}
}

func TestReconcilerHandleEvent_Installed_StaleRevision(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	ref := ContributionRef{ExtensionID: "com.example/pet", PluginID: "a1", ContributionID: "a1"}

	base := makeContrib("com.example/pet", "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"revision":    float64(2),
		"actionKey":   "new_wave",
		"displayName": "New Wave",
	})
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: "com.example/pet", Contribution: base}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	reg, _ := rec.Get(ref)
	if reg.Revision != 2 {
		t.Fatalf("revision=%d want 2", reg.Revision)
	}

	stale := makeContrib("com.example/pet", "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"revision":    float64(1),
		"actionKey":   "old_wave",
		"displayName": "Old Wave",
	})
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: "com.example/pet", Contribution: stale}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	reg, _ = rec.Get(ref)
	if reg.Revision != 2 {
		t.Errorf("stale install overrode revision: got %d want 2", reg.Revision)
	}
}

func TestReconcilerHandleEvent_Enabled(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	contrib := makeContrib("com.example/pet", "wave", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "wave",
		"displayName": "Wave",
	})

	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: "com.example/pet", Contribution: contrib}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseEnabled, ExtensionID: "com.example/pet", Contribution: contrib}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ref := ContributionRef{ExtensionID: "com.example/pet", PluginID: "wave", ContributionID: "wave"}
	reg, _ := rec.Get(ref)
	if reg.Status != ContributionStatusActive {
		t.Errorf("status=%s want active", reg.Status)
	}
	if !rec.IsExecutable(ref) {
		t.Error("enabled → executable")
	}
}

func TestReconcilerHandleEvent_Disabled_DetachesNonCurrentPlugin(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	p1 := makeContrib("com.example/pet", "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a1",
		"displayName": "A1",
	})
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: "com.example/pet", Contribution: p1}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseEnabled, ExtensionID: "com.example/pet", Contribution: p1}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	p2 := makeContrib("com.example/pet", "a2", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a2",
		"displayName": "A2",
	})
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: "com.example/pet", Contribution: p2}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseEnabled, ExtensionID: "com.example/pet", Contribution: p2}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseDisabled, ExtensionID: "com.example/pet", Contribution: p1}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ref1 := ContributionRefFromDefinition(p1)
	if rec.IsExecutable(ref1) {
		t.Error("p1 disabled → not executable")
	}

	ref2 := ContributionRefFromDefinition(p2)
	if !rec.IsExecutable(ref2) {
		t.Error("p2 still enabled → executable")
	}

	if reg, ok := rec.Get(ref1); !ok || reg.Status != ContributionStatusDisabled {
		t.Errorf("p1 disabled status=%v ok=%v", reg.Status, ok)
	}
}

func TestReconcilerHandleEvent_Uninstall_DetachesAll(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	ext := domain.ExtensionID("com.example/pet")
	for _, id := range []string{"a1", "a2", "a3"} {
		contrib := makeContrib(string(ext), id, domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   id,
			"displayName": id,
		})
		if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: contrib}); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: contrib}); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	}

	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseUninstalled, ExtensionID: ext}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	view := rec.View()
	if got := len(view.FindByExt(string(ext))); got != 3 {
		t.Errorf("uninstalled ext view count=%d want 3", got)
	}
	for _, reg := range view.FindByExt(string(ext)) {
		if reg.Status != ContributionStatusDetached {
			t.Errorf("status=%s want detached", reg.Status)
		}
		if rec.IsExecutable(reg.Ref) {
			t.Error("uninstalled → not executable")
		}
	}
}

func TestReconcilerReconcileExtension_Diff_AddRemoveUpdate(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	ext := domain.ExtensionID("com.example/pet")

	a1 := makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"revision":    float64(1),
		"actionKey":   "a1",
		"displayName": "A1 v1",
	})
	src.contribs[string(ext)] = []domain.ContributionDefinition{a1}

	if err := rec.ReconcileExtension(context.Background(), ext); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	view := rec.View()
	if got := len(view.FindByExt(string(ext))); got != 1 {
		t.Errorf("after first reconcile count=%d want 1", got)
	}

	a1v2 := makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"revision":    float64(2),
		"actionKey":   "a1",
		"displayName": "A1 v2",
	})
	a2 := makeContrib(string(ext), "a2", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a2",
		"displayName": "A2",
	})
	src.contribs[string(ext)] = []domain.ContributionDefinition{a1v2, a2}

	if err := rec.ReconcileExtension(context.Background(), ext); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	view = rec.View()
	byName := map[string]ContributionRegistration{}
	for _, reg := range view.FindByExt(string(ext)) {
		byName[reg.Ref.ContributionID] = reg
	}
	if len(byName) != 2 {
		t.Fatalf("after reconcile count=%d want 2", len(byName))
	}
	if got := byName["a1"].Revision; got != 2 {
		t.Errorf("a1 revision=%d want 2", got)
	}
	if _, ok := byName["a2"]; !ok {
		t.Error("expected a2 registered")
	}
}

func TestReconcilerReconcileExtension_RemovesGoneContributions(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	ext := domain.ExtensionID("com.example/pet")

	a1 := makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a1",
		"displayName": "A1",
	})
	a2 := makeContrib(string(ext), "a2", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a2",
		"displayName": "A2",
	})
	src.contribs[string(ext)] = []domain.ContributionDefinition{a1, a2}

	if err := rec.ReconcileExtension(context.Background(), ext); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got := len(rec.View().FindByExt(string(ext))); got != 2 {
		t.Fatalf("setup count=%d want 2", got)
	}

	src.contribs[string(ext)] = []domain.ContributionDefinition{a1}
	if err := rec.ReconcileExtension(context.Background(), ext); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	view := rec.View()
	if got := len(view.FindByExt(string(ext))); got != 1 {
		t.Errorf("after removal reconcile count=%d want 1", got)
	}
	if _, ok := rec.Get(ContributionRefFromDefinition(a2)); ok {
		t.Error("a2 should have been removed")
	}
}

type refreshResourceAdapter struct {
	registers int
	detaches  int
}

func (a *refreshResourceAdapter) Kind() ContributionKind { return KindResource }

func (a *refreshResourceAdapter) Validate(context.Context, ContributionRef, map[string]any) error {
	return nil
}

func (a *refreshResourceAdapter) Register(_ context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error) {
	a.registers++
	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}
	return ContributionRegistration{
		Ref:             ref,
		Kind:            KindResource,
		Revision:        rev,
		Status:          status,
		Definition:      cloneMap(def),
		CanonicalHandle: fmt.Sprintf("resource-%d", a.registers),
	}, nil
}

func (a *refreshResourceAdapter) Detach(context.Context, ContributionRegistration) error {
	a.detaches++
	return nil
}

func TestReconcilerUpdateRefreshesSamePathResourceWhenDefinitionIsUnchanged(t *testing.T) {
	src := newFakeSource()
	adapter := &refreshResourceAdapter{}
	rec := NewReconciler(src, []ContributionAdapter{adapter})
	ext := domain.ExtensionID("com.example/pet")
	contrib := makeContrib(string(ext), "pet-resource", domain.ContributionKindPetPlugin, map[string]any{
		"contributionKind": "pet_resource",
		"displayName":      "Pet",
		"resourceRef":      "assets/pet/manifest.json",
	})
	src.contribs[string(ext)] = []domain.ContributionDefinition{contrib}

	if err := rec.ReconcileExtension(context.Background(), ext); err != nil {
		t.Fatal(err)
	}
	ref := ContributionRefFromDefinition(contrib)
	before, ok := rec.Get(ref)
	if !ok || before.CanonicalHandle != "resource-1" {
		t.Fatalf("initial registration=%+v ok=%v", before, ok)
	}

	if err := rec.HandleEvent(context.Background(), LifecycleEvent{
		Phase:       PhaseUpdated,
		ExtensionID: ext,
		Version:     "1.0.3",
	}); err != nil {
		t.Fatal(err)
	}
	after, ok := rec.Get(ref)
	if !ok || after.CanonicalHandle != "resource-2" {
		t.Fatalf("updated registration=%+v ok=%v", after, ok)
	}
	if adapter.registers != 2 || adapter.detaches != 1 {
		t.Fatalf("refresh registers=%d detaches=%d", adapter.registers, adapter.detaches)
	}
}

func TestReconcilerInvalidContribution_MarksUnregistered(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	badAction := makeContrib("com.example/pet", "bad", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"displayName": "Bad — missing actionKey",
	})
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: "com.example/pet", Contribution: badAction}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ref := ContributionRefFromDefinition(badAction)
	reg, ok := rec.Get(ref)
	if !ok {
		t.Fatal("expected registered (even if invalid)")
	}
	if reg.Status != ContributionStatusInvalid {
		t.Errorf("status=%s want invalid", reg.Status)
	}
	if rec.IsExecutable(ref) {
		t.Error("invalid contribution should not be executable")
	}
}

func TestReconcilerUnknownPhaseNoPanic(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: LifecyclePhase("bogus"), ExtensionID: "com.example/pet"})
	if err == nil {
		t.Error("expected error for unknown phase")
	}
}

func TestReconcilerOwnershipMismatchLocked(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())

	evt := LifecycleEvent{
		Phase:        PhaseInstalled,
		ExtensionID:  "com.example/pet",
		Contribution: makeContrib("com.other/pet_conflict", "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{"actionKey": "a1", "displayName": "A1"}),
	}
	if err := rec.HandleEvent(context.Background(), evt); err == nil {
		t.Error("expected ownership mismatch error")
	}
}

func TestReconcilerAttachSource(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(nil, defaultAdapters())
	rec.AttachSource(src)
	if rec.source == nil {
		t.Error("expected source set")
	}
}

type failingSource struct{}

func (f *failingSource) ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	return nil, errors.New("kernel unavailable")
}
func (f *failingSource) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	return domain.ExtensionInstallation{}, errors.New("kernel unavailable")
}

func TestReconcilerReconcileErrorPropagates(t *testing.T) {
	rec := NewReconciler(&failingSource{}, defaultAdapters())

	err := rec.ReconcileExtension(context.Background(), "com.example/pet")
	if err == nil {
		t.Error("expected reconcile error from failing source")
	}
}

func TestReconcilerUninstallCleansOwnedInstallations(t *testing.T) {
	src := newFakeSource()
	ext := domain.ExtensionID("com.example/pet")
	var cleaned []domain.ExtensionID
	rec := NewReconciler(src, defaultAdapters(), WithUninstallCleanup(func(_ context.Context, extID domain.ExtensionID) error {
		cleaned = append(cleaned, extID)
		return nil
	}))

	contrib := makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a1",
		"displayName": "a1",
	})
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: contrib}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseUninstalled, ExtensionID: ext}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0] != ext {
		t.Fatalf("cleanup calls=%v want single call for %s", cleaned, ext)
	}
}

func TestReconcilerUninstallPropagatesCleanupError(t *testing.T) {
	src := newFakeSource()
	ext := domain.ExtensionID("com.example/pet")
	cleanupErr := errors.New("runtime offline: descriptor rtsess-one generation 4")
	rec := NewReconciler(src, defaultAdapters(), WithUninstallCleanup(func(_ context.Context, extID domain.ExtensionID) error {
		return fmt.Errorf("cleanup %s: %w", extID, cleanupErr)
	}))

	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseUninstalled, ExtensionID: ext}); err == nil {
		t.Fatal("expected uninstall cleanup error")
	} else if !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup error chain lost: %v", err)
	}
}

func TestReconcilerUpdateDoesNotCleanupInstallations(t *testing.T) {
	src := newFakeSource()
	ext := domain.ExtensionID("com.example/pet")
	calls := 0
	rec := NewReconciler(src, defaultAdapters(), WithUninstallCleanup(func(_ context.Context, extID domain.ExtensionID) error {
		calls++
		return nil
	}))
	src.contribs[string(ext)] = []domain.ContributionDefinition{
		makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
			"revision":    float64(2),
			"actionKey":   "a1",
			"displayName": "a1",
		}),
	}
	if err := rec.HandleEvent(context.Background(), LifecycleEvent{Phase: PhaseUpdated, ExtensionID: ext}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if calls != 0 {
		t.Fatalf("update lifecycle must not clean installations, calls=%d", calls)
	}
}
