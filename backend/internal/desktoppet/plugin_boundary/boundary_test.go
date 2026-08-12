package plugin_boundary

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func installContrib(bound *DesktopPetPluginBoundary, ctx context.Context, ext domain.ExtensionID, version, opID string, contribs ...domain.ContributionDefinition) {
	for _, c := range contribs {
		bound.reconciler.HandleEvent(ctx, LifecycleEvent{
			Phase:        PhaseInstalled,
			ExtensionID:  ext,
			Version:      version,
			OperationID:  opID,
			Contribution: c,
		})
	}
}

func enableContrib(bound *DesktopPetPluginBoundary, ctx context.Context, contribs ...domain.ContributionDefinition) {
	for _, c := range contribs {
		bound.reconciler.HandleEvent(ctx, LifecycleEvent{
			Phase:        PhaseEnabled,
			ExtensionID:  c.ExtensionID,
			Contribution: c,
		})
	}
}

func TestBoundaryLifecycle_FullCycle(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")

	a1 := makeContrib(string(ext), "a1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "a1",
		"displayName": "A1",
	})
	r1 := makeContrib(string(ext), "r1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"contributionKind": "pet_resource",
		"displayName":      "R1",
		"assetKind":        "sprite",
	})

	installContrib(bound, ctx, ext, "1.0", "op_install", a1, r1)
	enableContrib(bound, ctx, a1, r1)

	ref1 := ContributionRefFromDefinition(a1)
	if !bound.IsExecutable(ref1) {
		t.Error("a1 should be executable after enable")
	}

	if err := bound.HandleExtensionDisabled(ctx, ext, "1.0", "op_disable", ""); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if bound.IsExecutable(ref1) {
		t.Error("a1 should not be executable after disable")
	}

	if err := bound.HandleExtensionUninstalled(ctx, ext, "1.0", "op_uninstall", ""); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	for _, r := range bound.FindByExt(string(ext)) {
		if r.Status != ContributionStatusDetached {
			t.Errorf("status=%s want detached", r.Status)
		}
	}
}

func TestBoundaryValidateActionInvocation(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	okAction := makeContrib(string(ext), "ok", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"actionKey":   "ok",
		"displayName": "OK",
	})

	installContrib(bound, ctx, ext, "1.0", "op_install", okAction)
	enableContrib(bound, ctx, okAction)

	ref := ContributionRefFromDefinition(okAction)

	if err := bound.ValidateActionInvocation(ctx, ref); err != nil {
		t.Errorf("expected valid invocation: %v", err)
	}

	bound.HandleExtensionDisabled(ctx, ext, "1.0", "op_disable", "")
	if err := bound.ValidateActionInvocation(ctx, ref); err == nil {
		t.Error("expected disabled contribution invocation to fail")
	}
}

func TestBoundaryValidateResourceAccess_MissingProvider(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	missingRef := ContributionRef{ExtensionID: "com.gone/ext", PluginID: "p", ContributionID: "r"}
	err := bound.ValidateResourceAccess(ctx, missingRef)
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestBoundaryValidateResourceAccess_StaticAvailable(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	res := makeContrib(string(ext), "static_image", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"contributionKind": "pet_resource",
		"displayName":      "StaticImage",
		"assetKind":        "sprite",
	})

	installContrib(bound, ctx, ext, "1.0", "op_install", res)

	ref := ContributionRefFromDefinition(res)
	reg, ok := bound.Find(ref)
	if !ok {
		t.Fatal("expected resource registered")
	}
	_ = reg

	if err := bound.ValidateResourceAccess(ctx, ref); err != nil {
		t.Errorf("static resource registered before enable should be accessible: %v", err)
	}
}

func TestBoundaryValidateRuntimeCapability_Rules(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	cap := makeContrib(string(ext), "cap1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"contributionKind": "pet_runtime_capability",
		"capabilityId":     "cap_trigger",
		"capabilityKind":   "trigger_action",
	})
	installContrib(bound, ctx, ext, "1.0", "op_install", cap)

	ref := ContributionRefFromDefinition(cap)
	if err := bound.ValidateRuntimeCapability(ctx, ref); err == nil {
		t.Error("registered (not enabled) runtime cap should not be executable")
	}

	enableContrib(bound, ctx, cap)
	if err := bound.ValidateRuntimeCapability(ctx, ref); err != nil {
		t.Errorf("enabled runtime cap should be executable: %v", err)
	}
}

func TestBoundaryValidateFloatingWindowScope(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	ext := domain.ExtensionID("com.example/pet")
	win := makeContrib(string(ext), "win1", domain.ContributionKindDesktopPetPlugin, map[string]any{
		"contributionKind": "pet_floating_window_capability",
		"supportsShow":     true,
		"supportsPosition": true,
		"supportsOpacity":  false,
	})
	installContrib(bound, ctx, ext, "1.0", "op_install", win)
	enableContrib(bound, ctx, win)

	ref := ContributionRefFromDefinition(win)

	if err := bound.ValidateFloatingWindowScope(ctx, ref, "show"); err != nil {
		t.Errorf("show should be supported: %v", err)
	}
	if err := bound.ValidateFloatingWindowScope(ctx, ref, "position"); err != nil {
		t.Errorf("position should be supported: %v", err)
	}
	if err := bound.ValidateFloatingWindowScope(ctx, ref, "opacity"); err == nil {
		t.Error("opacity should be denied")
	}
	if err := bound.ValidateFloatingWindowScope(ctx, ref, "unknown_op"); err == nil {
		t.Error("unknown op should be denied")
	}

	bound.HandleExtensionDisabled(ctx, ext, "1.0", "op_disable", "")
	if err := bound.ValidateFloatingWindowScope(ctx, ref, "show"); err == nil {
		t.Error("disabled extension's window op should be denied")
	}
}

func TestLifecycleHookRegistry(t *testing.T) {
	reg := NewLifecycleHookRegistry()
	calls := 0
	reg.Register(func(ctx context.Context, evt LifecycleEvent) error {
		calls++
		return nil
	})
	reg.Register(func(ctx context.Context, evt LifecycleEvent) error {
		calls++
		return nil
	})

	if err := reg.Dispatch(context.Background(), LifecycleEvent{Phase: PhaseInstalled}); err != nil {
		t.Errorf("dispatch error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls=%d want 2", calls)
	}
}

func TestBoundaryMissingProvider_NotCrash(t *testing.T) {
	src := newFakeSource()
	bound := NewBoundary(src)
	ctx := context.Background()

	for _, fn := range []func() error{
		func() error { return bound.ValidateActionInvocation(ctx, ContributionRef{ExtensionID: "com/x", PluginID: "p", ContributionID: "c"}) },
		func() error { return bound.ValidateResourceAccess(ctx, ContributionRef{ExtensionID: "com/x", PluginID: "p", ContributionID: "c"}) },
		func() error { return bound.ValidateRuntimeCapability(ctx, ContributionRef{ExtensionID: "com/x", PluginID: "p", ContributionID: "c"}) },
		func() error { return bound.ValidateFloatingWindowScope(ctx, ContributionRef{ExtensionID: "com/x", PluginID: "p", ContributionID: "c"}, "show") },
	} {
		if err := fn(); err == nil {
			t.Error("expected validation error")
		}
	}
}
