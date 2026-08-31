package plugin_boundary

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestRace_DisableVsAction(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	for i := 0; i < 50; i++ {
		c := makeContrib(string(ext), "c"+string(rune('a'+i%26))+string(rune('0'+i/26)), domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   "act" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			"displayName": "Action",
		})
		installContrib(bound, ctx, ext, "1.0", "op", c)
		enableContrib(bound, ctx, c)
	}

	var disableWG sync.WaitGroup
	var validateWG sync.WaitGroup
	var invalidDuringDisable int32

	for i := 0; i < 50; i++ {
		c := makeContrib(string(ext), "c"+string(rune('a'+i%26))+string(rune('0'+i/26)), domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   "act" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			"displayName": "Action",
		})
		ref := ContributionRefFromDefinition(c)
		validateWG.Add(1)
		go func() {
			defer validateWG.Done()
			if !bound.IsExecutable(ref) {
				atomic.AddInt32(&invalidDuringDisable, 1)
			}
		}()
	}

	disableWG.Add(1)
	go func() {
		defer disableWG.Done()
		bound.HandleExtensionDisabled(ctx, ext, "1.0", "op_disable", "")
	}()

	disableWG.Wait()
	validateWG.Wait()

	observedInvalid := atomic.LoadInt32(&invalidDuringDisable)
	if observedInvalid < 0 || observedInvalid > 50 {
		t.Fatalf("invalid observation count out of range: %d", observedInvalid)
	}
	for i := 0; i < 50; i++ {
		c := makeContrib(string(ext), "c"+string(rune('a'+i%26))+string(rune('0'+i/26)), domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   "act" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			"displayName": "Action",
		})
		if bound.IsExecutable(ContributionRefFromDefinition(c)) {
			t.Fatalf("contribution %d remained executable after disable completed", i)
		}
	}
}

func TestRace_UninstallVsAction(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	for i := 0; i < 50; i++ {
		id := "c" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		c := makeContrib(string(ext), id, domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   "act" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			"displayName": "Action",
		})
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: c})
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: c})
	}

	done := make(chan struct{})
	go func() {
		rec.DetachExtension(ctx, ext)
		close(done)
	}()

	var executableSeen int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		id := "c" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		ref := ContributionRef{ExtensionID: ext, PluginID: domain.ContributionID(id), ContributionID: id}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec.IsExecutable(ref) {
				atomic.AddInt32(&executableSeen, 1)
			}
		}()
	}

	<-done
	wg.Wait()
	observedExecutable := atomic.LoadInt32(&executableSeen)
	if observedExecutable < 0 || observedExecutable > 50 {
		t.Fatalf("executable observation count out of range: %d", observedExecutable)
	}
	for i := 0; i < 50; i++ {
		id := "c" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		ref := ContributionRef{ExtensionID: ext, PluginID: domain.ContributionID(id), ContributionID: id}
		if rec.IsExecutable(ref) {
			t.Fatalf("contribution %s remained executable after detach completed", id)
		}
	}
}

func TestRace_ConcurrentReconcile_NoDuplicates(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")

	a1 := makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a1",
		"displayName": "A1",
	})

	src.contribs[string(ext)] = []domain.ContributionDefinition{a1}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.ReconcileExtension(ctx, ext)
		}()
	}
	wg.Wait()

	view := rec.View()
	if got := len(view.FindByExt(string(ext))); got != 1 {
		t.Errorf("expected exactly 1 contribution after concurrent reconcile, got %d", got)
	}
}

func TestRace_IsExecutableDuringDisable_RaceSafe(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	c := makeContrib(string(ext), "c1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "c1",
		"displayName": "C1",
	})
	ref := ContributionRefFromDefinition(c)

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: c})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: c})

	if !rec.IsExecutable(ref) {
		t.Fatal("expected executable before disable")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseDisabled, ExtensionID: ext, Contribution: c})
	}()
	wg.Wait()

	if rec.IsExecutable(ref) {
		t.Error("expected not executable after disable")
	}
}

func TestIsolation_PluginAB(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	extA := domain.ExtensionID("com.a/pet")
	extB := domain.ExtensionID("com.b/pet")

	a1 := makeContrib(string(extA), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a1",
		"displayName": "A1",
	})
	b1 := makeContrib(string(extB), "b1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "b1",
		"displayName": "B1",
	})

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: extA, Contribution: a1})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: extB, Contribution: b1})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: extA, Contribution: a1})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: extB, Contribution: b1})

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseDisabled, ExtensionID: extA, Contribution: a1})

	refA := ContributionRefFromDefinition(a1)
	refB := ContributionRefFromDefinition(b1)

	if rec.IsExecutable(refA) {
		t.Error("A disabled → not executable")
	}
	if !rec.IsExecutable(refB) {
		t.Error("B unaffected → still executable")
	}
	view := rec.View()
	if got := len(view.FindByExt(string(extB))); got != 1 {
		t.Errorf("ext B count=%d want 1", got)
	}
}

func TestZeroResidue_EnableDisableLoop(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	c := makeContrib(string(ext), "c1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "c1",
		"displayName": "C1",
	})
	ref := ContributionRefFromDefinition(c)

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: c})

	for i := 0; i < 100; i++ {
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: c})
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseDisabled, ExtensionID: ext, Contribution: c})
	}

	if rec.IsExecutable(ref) {
		t.Error("final state should be disabled")
	}
	view := rec.View()
	if got := len(view.FindByExt(string(ext))); got != 1 {
		t.Errorf("expect 1 registration after loop, got %d", got)
	}
}

func TestZeroResidue_InstallUninstallLoop(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")

	for i := 0; i < 5; i++ {
		c := makeContrib(string(ext), "c1", domain.ContributionKindDesktopPetPlugin, map[string]any{
			"actionKey":   "c1",
			"displayName": "C1",
		})
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: c})
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: c})
		rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseUninstalled, ExtensionID: ext})
	}

	view := rec.View()
	if got := len(view.FindByExt(string(ext))); got != 1 {
		t.Errorf("leaked registrations after install/uninstall cycles: got %d", got)
	}
}

func TestIsolation_SameExtensionTwoPlugins(t *testing.T) {
	src := newFakeSource()
	rec := NewReconciler(src, defaultAdapters())
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/suite")
	p1 := makeContrib(string(ext), "plugin_one", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "run",
		"displayName": "Run",
	})
	p2 := makeContrib(string(ext), "plugin_two", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "walk",
		"displayName": "Walk",
	})

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: p1})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseInstalled, ExtensionID: ext, Contribution: p2})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: p1})
	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseEnabled, ExtensionID: ext, Contribution: p2})

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseDisabled, ExtensionID: ext, Contribution: p1})

	if rec.IsExecutable(ContributionRefFromDefinition(p1)) {
		t.Error("p1 disabled")
	}
	if !rec.IsExecutable(ContributionRefFromDefinition(p2)) {
		t.Error("p2 should remain active")
	}

	view := rec.View()
	if got := len(view.FindByPlugin(string(ext), "plugin_two")); got != 1 {
		t.Errorf("plugin_two registrations=%d want 1", got)
	}

	rec.HandleEvent(ctx, LifecycleEvent{Phase: PhaseUninstalled, ExtensionID: ext})
	view = rec.View()
	for _, reg := range view.FindByExt(string(ext)) {
		if reg.Status != ContributionStatusDetached {
			t.Errorf("status=%s want detached", reg.Status)
		}
	}
}
