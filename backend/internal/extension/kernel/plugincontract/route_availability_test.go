package plugincontract

import (
	"errors"
	"fmt"
	"testing"
)

func TestRouteAvailabilityCoreMissingImplementation(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "builtin.core/core/memory.search")
	decision, err := SelectRouteChecked(candidates, RoutePolicy{Mode: RouteModeAuto}, NewCoreImplementationIndex(), nil)
	if err == nil {
		t.Fatalf("core without an implementation must fail")
	}
	var failure *RouteFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v want *RouteFailureError", err)
	}
	if !errors.Is(err, ErrRouteNoBuiltin) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	if failure.Class != RouteFailureNoBuiltin || failure.ProviderID != "builtin.core/core/memory.search" || !failure.FallbackBlocked {
		t.Fatalf("failure = %+v", failure)
	}
	if decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("selected = %s want core", decision.Selected.ProviderID)
	}
	fmt.Printf(
		"route availability core missing service=%s provider=%s class=%s fallbackBlocked=%v candidates=%d\n",
		decision.ServiceID, failure.ProviderID, failure.Class, failure.FallbackBlocked, len(decision.Candidates),
	)
}

func TestRouteAvailabilityCoreImplemented(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "builtin.core/core/memory.search")
	index := NewCoreImplementationIndex("memory.search")
	decision, err := SelectRouteChecked(candidates, RoutePolicy{Mode: RouteModeAuto}, index, nil)
	if err != nil {
		t.Fatalf("SelectRouteChecked: %v", err)
	}
	if !index.Has("builtin.core/core/memory.search") || index.Len() != 1 {
		t.Fatalf("index = %d want 1", index.Len())
	}
	if decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("selected = %s want core", decision.Selected.ProviderID)
	}
	fmt.Printf(
		"route availability core implemented service=%s provider=%s class=%s implementations=%d\n",
		decision.ServiceID, decision.Selected.ProviderID, RouteFailureNone, index.Len(),
	)
}

func TestRouteAvailabilityPluginHealthy(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	probes := 0
	decision, err := SelectRouteChecked(candidates, RoutePolicy{Mode: RouteModeAuto}, NewCoreImplementationIndex("memory.search"), func(providerID string) bool {
		probes++
		return providerID == "plugin.host"
	})
	if err != nil {
		t.Fatalf("healthy plugin must route: %v", err)
	}
	if probes != 1 {
		t.Fatalf("probes = %d want 1", probes)
	}
	fmt.Printf(
		"route availability plugin healthy service=%s provider=%s class=%s probes=%d\n",
		decision.ServiceID, decision.Selected.ProviderID, RouteFailureNone, probes,
	)
}

func TestRouteAvailabilityPluginCrashed(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	decision, err := SelectRouteChecked(candidates, RoutePolicy{Mode: RouteModeAuto}, NewCoreImplementationIndex("memory.search"), func(string) bool {
		return false
	})
	if err == nil {
		t.Fatalf("crashed plugin must be unavailable")
	}
	var failure *RouteFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v want *RouteFailureError", err)
	}
	if !errors.Is(err, ErrRouteUnavailable) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	if failure.Class != RouteFailureUnavailable || failure.ProviderID != "plugin.host" || failure.FallbackBlocked {
		t.Fatalf("failure = %+v", failure)
	}
	if decision.Selected.ProviderID != "plugin.host" {
		t.Fatalf("selected = %s want plugin", decision.Selected.ProviderID)
	}
	fmt.Printf(
		"route availability plugin crashed service=%s provider=%s class=%s error=%s\n",
		decision.ServiceID, failure.ProviderID, failure.Class, "ErrRouteUnavailable",
	)
}

func TestRouteAvailabilityRefusesFakeFallback(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "builtin.core/core/memory.search")
	if candidates.Selected.Priority != 10 {
		t.Fatalf("fixture should bind core with priority 10, got %d", candidates.Selected.Priority)
	}
	pluginCandidates := 0
	for _, candidate := range candidates.Candidates {
		if !isCoreProvider(candidate.ProviderID) {
			pluginCandidates++
		}
	}
	decision, err := SelectRouteChecked(candidates, RoutePolicy{Mode: RouteModeAuto}, NewCoreImplementationIndex(), nil)
	if err == nil {
		t.Fatalf("missing core implementation must not route to the plugin")
	}
	if !errors.Is(err, ErrRouteNoBuiltin) {
		t.Fatalf("error = %v want ErrRouteNoBuiltin", err)
	}
	if decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("selected = %s the router switched to the plugin", decision.Selected.ProviderID)
	}
	fmt.Printf(
		"route availability no fake fallback mode=%s bound=%s selected=%s pluginCandidates=%d fallbackBlocked=true\n",
		decision.Mode, candidates.Selected.ProviderID, decision.Selected.ProviderID, pluginCandidates,
	)
}

func TestRouteAvailabilityPluginOnlyCrashStaysUnavailable(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	decision, err := SelectRouteChecked(
		candidates,
		RoutePolicy{Mode: RouteModePluginOnly},
		NewCoreImplementationIndex("memory.search"),
		func(string) bool { return false },
	)
	if err == nil {
		t.Fatalf("plugin_only with a crashed plugin must fail")
	}
	var failure *RouteFailureError
	if !errors.As(err, &failure) || failure.Class != RouteFailureUnavailable {
		t.Fatalf("failure = %+v", failure)
	}
	if failure.ServiceID != "memory.search" {
		t.Fatalf("service = %s", failure.ServiceID)
	}
	fmt.Printf(
		"route availability plugin only crashed mode=%s provider=%s class=%s skippedCore=%d\n",
		decision.Mode, failure.ProviderID, failure.Class, len(decision.Skipped),
	)
}

func TestRouteAvailabilityIgnoresHealthForCore(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "builtin.core/core/memory.search")
	probes := 0
	decision, err := SelectRouteChecked(
		candidates,
		RoutePolicy{Mode: RouteModeCoreOnly},
		NewCoreImplementationIndex("memory.search"),
		func(string) bool {
			probes++
			return false
		},
	)
	if err != nil {
		t.Fatalf("core implementation present must not be reported unavailable: %v", err)
	}
	if probes != 0 {
		t.Fatalf("probes = %d want 0 for a core provider", probes)
	}
	if decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("selected = %s want core", decision.Selected.ProviderID)
	}
	fmt.Printf(
		"route availability core health ignored mode=%s provider=%s healthProbe=skipped class=%s probeCalls=%d\n",
		decision.Mode, decision.Selected.ProviderID, RouteFailureNone, probes,
	)
}

func TestRouteAvailabilityPassesPolicyErrorsThrough(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	decision, err := SelectRouteChecked(candidates, RoutePolicy{Mode: RouteMode("hybrid")}, NewCoreImplementationIndex(), nil)
	if err == nil {
		t.Fatalf("invalid policy must fail")
	}
	var invalid *RoutePolicyInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v want *RoutePolicyInvalidError", err)
	}
	var failure *RouteFailureError
	if errors.As(err, &failure) {
		t.Fatalf("policy error must not be reclassified: %+v", failure)
	}
	if decision.Selected.ProviderID != "" {
		t.Fatalf("invalid policy must not select a provider, got %s", decision.Selected.ProviderID)
	}
	fmt.Printf(
		"route availability invalid policy field=%s value=%s error=%s class=none\n",
		invalid.Field, invalid.Value, "ErrRoutePolicyInvalid",
	)
}
