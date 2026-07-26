package enablement

import (
	"context"
	"testing"
)

func TestResolverFullyEnabled(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateValid,
		Enablement:        EnablementEnabled,
		Scope:             ScopeStateActive,
		Permission:        PermissionAllowed,
		DesiredRuntime:    DesiredRuntimeStarted,
		ActualRuntime:     ActualRuntimeReady,
		Health:            HealthHealthy,
		Circuit:           CircuitClosed,
		DependencyReady:   true,
		PlatformSupported: true,
	}
	if err := store.SetState(context.Background(), state); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if !eff.Executable {
		t.Fatalf("expected executable, got reasons: %+v", eff.Reasons)
	}
	if !eff.Visible {
		t.Errorf("expected visible")
	}
}

func TestResolverNotInstalled(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext-missing"}
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "not_installed" {
		t.Errorf("expected not_installed reason, got %+v", r)
	}
}

func TestResolverDisabled(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateValid,
		Enablement:        EnablementDisabled,
		PlatformSupported: true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable when disabled")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "extension_disabled" {
		t.Errorf("expected extension_disabled reason, got %+v", r)
	}
}

func TestResolverPermissionDenied(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectTool, ID: "tool1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateValid,
		Enablement:        EnablementEnabled,
		Scope:             ScopeStateActive,
		Permission:        PermissionDenied,
		PlatformSupported: true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable with denied permission")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "permission_denied" {
		t.Errorf("expected permission_denied, got %+v", r)
	}
}

func TestResolverScopeExpired(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectTool, ID: "tool1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateValid,
		Enablement:        EnablementEnabled,
		Scope:             ScopeStateExpired,
		Permission:        PermissionAllowed,
		PlatformSupported: true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "scope_denied" {
		t.Errorf("expected scope_denied, got %+v", r)
	}
}

func TestResolverRuntimeCrashed(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateValid,
		Enablement:        EnablementEnabled,
		Scope:             ScopeStateActive,
		Permission:        PermissionAllowed,
		DesiredRuntime:    DesiredRuntimeStarted,
		ActualRuntime:     ActualRuntimeCrashed,
		PlatformSupported: true,
		DependencyReady:   true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "runtime_crashed" {
		t.Errorf("expected runtime_crashed, got %+v", r)
	}
}

func TestResolverCircuitOpen(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateValid,
		Enablement:        EnablementEnabled,
		Scope:             ScopeStateActive,
		Permission:        PermissionAllowed,
		DesiredRuntime:    DesiredRuntimeStarted,
		ActualRuntime:     ActualRuntimeReady,
		Health:            HealthHealthy,
		Circuit:           CircuitOpen,
		PlatformSupported: true,
		DependencyReady:   true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "circuit_open" {
		t.Errorf("expected circuit_open, got %+v", r)
	}
}

func TestEnablementService(t *testing.T) {
	store := NewInMemoryStateStore()
	resolver := NewDefaultResolver(store)
	svc := NewEnablementService(store, resolver)
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	if err := svc.Enable(context.Background(), subject); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	st, err := svc.Get(context.Background(), subject)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if st.Enablement != EnablementEnabled {
		t.Errorf("expected enabled, got %s", st.Enablement)
	}
	if err := svc.Disable(context.Background(), subject); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	st, _ = svc.Get(context.Background(), subject)
	if st.Enablement != EnablementDisabled {
		t.Errorf("expected disabled, got %s", st.Enablement)
	}
}

func TestResolverDefinitionInvalid(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalled,
		Definition:        DefinitionStateInvalid,
		Enablement:        EnablementEnabled,
		PlatformSupported: true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "definition_invalid" {
		t.Errorf("expected definition_invalid, got %+v", r)
	}
}

func TestResolverInstallationInProgress(t *testing.T) {
	store := NewInMemoryStateStore()
	subject := StateSubject{Kind: SubjectExtension, ID: "ext1"}
	state := SubjectState{
		Subject:           subject,
		Installation:      InstallationStateInstalling,
		PlatformSupported: true,
	}
	_ = store.SetState(context.Background(), state)
	resolver := NewDefaultResolver(store)
	eff := resolver.Resolve(context.Background(), subject, StateRuntimeContext{})
	if eff.Executable {
		t.Errorf("expected not executable during install")
	}
	if r := eff.PrimaryReason(); r == nil || r.Code != "installation_in_progress" {
		t.Errorf("expected installation_in_progress, got %+v", r)
	}
}
