package plugincontract

import (
	"errors"
	"fmt"
	"testing"
)

func routeCandidates(t *testing.T, providers []ServiceDescriptor, boundProviderID string) ProviderCandidates {
	t.Helper()
	resolution := testGraph(t, providers, ServiceSelectionAll)
	snapshot := testSnapshot(t, globalBinding("memory.search", boundProviderID))
	candidates, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if err != nil {
		t.Fatalf("SelectProviderCandidates: %v", err)
	}
	return candidates
}

func routeProviders() []ServiceDescriptor {
	plugin := testDescriptor("plugin.host", "memory.search", 90)
	core := testDescriptor("builtin.core/core/memory.search", "memory.search", 10)
	return []ServiceDescriptor{plugin, core}
}

func TestRoutePolicyAutoKeepsBoundProvider(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	decision, err := SelectRoute(candidates, RoutePolicy{Mode: RouteModeAuto})
	if err != nil {
		t.Fatalf("SelectRoute: %v", err)
	}
	if decision.Selected.ProviderID != "plugin.host" || decision.Pinned || len(decision.Skipped) != 0 {
		t.Fatalf("decision = %+v", decision)
	}
	if len(decision.Candidates) != 2 {
		t.Fatalf("candidates = %d want 2", len(decision.Candidates))
	}
	fmt.Printf(
		"route policy mode=auto selected=%s bound=%s candidates=%d skipped=%d pinned=%v\n",
		decision.Selected.ProviderID, candidates.Selected.ProviderID, len(decision.Candidates), len(decision.Skipped), decision.Pinned,
	)
}

func TestRoutePolicyCoreOnlyExcludesPlugins(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	decision, err := SelectRoute(candidates, RoutePolicy{Mode: RouteModeCoreOnly})
	if err != nil {
		t.Fatalf("SelectRoute: %v", err)
	}
	if decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("selected = %s want core", decision.Selected.ProviderID)
	}
	if len(decision.Skipped) != 1 || decision.Skipped[0].ProviderID != "plugin.host" || decision.Skipped[0].Reason != "mode_excluded" {
		t.Fatalf("skipped = %+v", decision.Skipped)
	}
	fmt.Printf(
		"route policy mode=core_only selected=%s candidates=%d skipped=%d reason=%s\n",
		decision.Selected.ProviderID, len(decision.Candidates), len(decision.Skipped), decision.Skipped[0].Reason,
	)
}

func TestRoutePolicyPluginOnlyExcludesCore(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	decision, err := SelectRoute(candidates, RoutePolicy{Mode: RouteModePluginOnly})
	if err != nil {
		t.Fatalf("SelectRoute: %v", err)
	}
	if decision.Selected.ProviderID != "plugin.host" {
		t.Fatalf("selected = %s want plugin", decision.Selected.ProviderID)
	}
	if len(decision.Skipped) != 1 || decision.Skipped[0].Reason != "mode_excluded" {
		t.Fatalf("skipped = %+v", decision.Skipped)
	}
	fmt.Printf(
		"route policy mode=plugin_only selected=%s candidates=%d skipped=%d reason=%s\n",
		decision.Selected.ProviderID, len(decision.Candidates), len(decision.Skipped), decision.Skipped[0].Reason,
	)
}

func TestRoutePolicyPinnedCoreBeatsEvolutionCandidate(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	if candidates.Selected.Priority != 90 {
		t.Fatalf("fixture should bind the higher priority plugin, got %d", candidates.Selected.Priority)
	}
	decision, err := SelectRoute(candidates, RoutePolicy{Mode: RouteModeAuto, PinnedProviderID: "builtin.core/core/memory.search"})
	if err != nil {
		t.Fatalf("SelectRoute: %v", err)
	}
	if !decision.Pinned || decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("pinned decision = %+v", decision)
	}
	if decision.Selected.Priority >= 90 {
		t.Fatalf("pinned core kept the plugin priority %d", decision.Selected.Priority)
	}
	fmt.Printf(
		"route policy pin pinned=%s selected=%s pluginPriority=90 corePriority=%d overridden=%v\n",
		"builtin.core/core/memory.search", decision.Selected.ProviderID, decision.Selected.Priority, decision.Selected.ProviderID != candidates.Selected.ProviderID,
	)
}

func TestRoutePolicyPinConflictsWithMode(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	_, err := SelectRoute(candidates, RoutePolicy{Mode: RouteModeCoreOnly, PinnedProviderID: "plugin.host"})
	if err == nil {
		t.Fatalf("core_only with a plugin pin must fail")
	}
	var conflict *RoutePinConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v want *RoutePinConflictError", err)
	}
	if !errors.Is(err, ErrRoutePinConflict) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	if conflict.Pinned != "plugin.host" || conflict.Mode != RouteModeCoreOnly {
		t.Fatalf("conflict = %+v", conflict)
	}
	fmt.Printf(
		"route policy pin conflict pinned=%s mode=%s error=%v\n",
		conflict.Pinned, conflict.Mode, "ErrRoutePinConflict",
	)
}

func TestRoutePolicyCompatibilityGates(t *testing.T) {
	legacy := testDescriptor("plugin.host", "memory.search", 90)
	legacy.ContractHash = "contract.plugin"
	newer := testDescriptor("plugin.new", "memory.search", 80)
	newer.ContractHash = "contract.core"
	newer.Version = "1.0.0"
	core := testDescriptor("builtin.core/core/memory.search", "memory.search", 10)
	core.ContractHash = "contract.core"
	core.Version = "2.1.0"
	candidates := routeCandidates(t, []ServiceDescriptor{legacy, newer, core}, "plugin.host")
	decision, err := SelectRoute(candidates, RoutePolicy{
		Mode:             RouteModeAuto,
		RequiredContract: "contract.core",
		RequiredVersion:  ">=2.0.0",
	})
	if err != nil {
		t.Fatalf("SelectRoute: %v", err)
	}
	if decision.Selected.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("selected = %s want core", decision.Selected.ProviderID)
	}
	if !decision.Fallback {
		t.Fatalf("incompatible bound provider must fall back to a compatible candidate")
	}
	if len(decision.Skipped) != 2 {
		t.Fatalf("skipped = %+v want 2", decision.Skipped)
	}
	if decision.Skipped[0].Reason != "contract_mismatch" || decision.Skipped[1].Reason != "version_mismatch" {
		t.Fatalf("skip reasons = %+v", decision.Skipped)
	}
	fmt.Printf(
		"route policy compat selected=%s candidates=%d skipped=%d reasons=%s,%s fallback=%v\n",
		decision.Selected.ProviderID, len(decision.Candidates), len(decision.Skipped),
		decision.Skipped[0].Reason, decision.Skipped[1].Reason, decision.Fallback,
	)
}

func TestRoutePolicyUnsatisfiedMode(t *testing.T) {
	plugin := testDescriptor("plugin.host", "memory.search", 90)
	candidates := routeCandidates(t, []ServiceDescriptor{plugin}, "plugin.host")
	_, err := SelectRoute(candidates, RoutePolicy{Mode: RouteModeCoreOnly})
	if err == nil {
		t.Fatalf("core_only without a core provider must fail")
	}
	var unsatisfied *RouteModeUnsatisfiedError
	if !errors.As(err, &unsatisfied) {
		t.Fatalf("error = %v want *RouteModeUnsatisfiedError", err)
	}
	if !errors.Is(err, ErrRouteModeUnsatisfied) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	if unsatisfied.Mode != RouteModeCoreOnly || len(unsatisfied.Skipped) != 1 {
		t.Fatalf("unsatisfied = %+v", unsatisfied)
	}
	fmt.Printf(
		"route policy unsatisfied mode=%s candidates=0 skipped=%d error=%v\n",
		unsatisfied.Mode, len(unsatisfied.Skipped), "ErrRouteModeUnsatisfied",
	)
}

func TestRoutePolicyInvalidInput(t *testing.T) {
	candidates := routeCandidates(t, routeProviders(), "plugin.host")
	_, err := SelectRoute(candidates, RoutePolicy{Mode: RouteMode("hybrid")})
	if err == nil {
		t.Fatalf("unknown mode must fail")
	}
	var invalid *RoutePolicyInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v want *RoutePolicyInvalidError", err)
	}
	if !errors.Is(err, ErrRoutePolicyInvalid) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	if invalid.Field != "mode" || invalid.Value != "hybrid" {
		t.Fatalf("invalid = %+v", invalid)
	}
	fmt.Printf(
		"route policy invalid field=%s value=%s error=%v\n",
		invalid.Field, invalid.Value, "ErrRoutePolicyInvalid",
	)
}
