package plugincontract

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testDescriptor(providerID, serviceID string, priority int) ServiceDescriptor {
	return ServiceDescriptor{
		ProviderID: providerID,
		Module:     ModuleRef{PluginID: "plugin." + providerID, ModuleID: "module.main"},
		ServiceID:  serviceID,
		Version:    "1.0.0",
		Scopes:     []ServiceScope{ServiceScopeGlobal},
		Placement:  ServicePlacementCore,
		Priority:   priority,
	}
}

func testGraph(t *testing.T, providers []ServiceDescriptor, selection ServiceSelection) ServiceGraphResolution {
	t.Helper()
	resolution := ServiceGraphResolver{}.Resolve(ServiceGraphRequest{
		Providers: providers,
		Consumers: []ServiceConsumer{{
			Module: ModuleRef{PluginID: "consumer.plugin", ModuleID: "module.main"},
			Requirements: []ServiceRequirement{{
				ServiceID:    "memory.search",
				VersionRange: ">=1.0.0",
				Selection:    selection,
			}},
		}},
	})
	if !resolution.Valid() {
		t.Fatalf("test graph should be valid, issues: %v", resolution.Issues)
	}
	return resolution
}

func testSnapshot(t *testing.T, bindings ...Binding) *BindingSnapshot {
	t.Helper()
	snapshot, err := NewBindingSnapshot("boot-1", 4, bindings)
	if err != nil {
		t.Fatalf("NewBindingSnapshot: %v", err)
	}
	return snapshot
}

func TestSelectProviderCandidatesPrefersBoundProvider(t *testing.T) {
	resolution := testGraph(t, []ServiceDescriptor{
		testDescriptor("provider.b", "memory.search", 5),
		testDescriptor("provider.a", "memory.search", 10),
	}, ServiceSelectionAll)
	snapshot := testSnapshot(t, globalBinding("memory.search", "provider.b"))
	selected, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if err != nil {
		t.Fatalf("SelectProviderCandidates: %v", err)
	}
	if selected.Selected.ProviderID != "provider.b" {
		t.Fatalf("expected bound provider.b, got %s", selected.Selected.ProviderID)
	}
	if len(selected.Candidates) != 2 {
		t.Fatalf("expected two candidates, got %d", len(selected.Candidates))
	}
	if selected.Candidates[0].ProviderID != "provider.a" || selected.Candidates[1].ProviderID != "provider.b" {
		t.Fatalf("candidates not ordered by priority: %s, %s", selected.Candidates[0].ProviderID, selected.Candidates[1].ProviderID)
	}
	if selected.Revision != 4 || selected.Epoch != "boot-1" || selected.Scope != BindingScopeGlobal {
		t.Fatalf("selection lost snapshot identity: revision=%d epoch=%s scope=%s", selected.Revision, selected.Epoch, selected.Scope)
	}
	if !selected.Bound {
		t.Fatalf("selection should be marked as bound")
	}
	if len(selected.Trail) != 3 || selected.Trail[2] != "global=hit" {
		t.Fatalf("unexpected trail %v", selected.Trail)
	}
}

func TestSelectProviderCandidatesIsDeterministic(t *testing.T) {
	providers := []ServiceDescriptor{
		testDescriptor("provider.c", "memory.search", 5),
		testDescriptor("provider.b", "memory.search", 5),
		testDescriptor("provider.a", "memory.search", 10),
	}
	snapshot := testSnapshot(t, globalBinding("memory.search", "provider.a"))
	var first []string
	for round := 0; round < 25; round++ {
		resolution := testGraph(t, providers, ServiceSelectionAll)
		selected, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		order := make([]string, len(selected.Candidates))
		for index, candidate := range selected.Candidates {
			order[index] = candidate.ProviderID
		}
		if round == 0 {
			first = order
			continue
		}
		if len(order) != len(first) {
			t.Fatalf("round %d candidate count drifted", round)
		}
		for index := range order {
			if order[index] != first[index] {
				t.Fatalf("round %d order drifted: %v vs %v", round, order, first)
			}
		}
	}
	if first[0] != "provider.a" || first[1] != "provider.b" || first[2] != "provider.c" {
		t.Fatalf("tie break should be provider id ascending, got %v", first)
	}
}

func TestSelectProviderCandidatesRejectsInvalidGraph(t *testing.T) {
	duplicated := testDescriptor("provider.a", "memory.search", 10)
	resolution := ServiceGraphResolver{}.Resolve(ServiceGraphRequest{Providers: []ServiceDescriptor{duplicated, duplicated}})
	if resolution.Valid() {
		t.Fatalf("graph should be invalid")
	}
	snapshot := testSnapshot(t, globalBinding("memory.search", "provider.a"))
	_, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if !errors.Is(err, ErrServiceGraphInvalid) {
		t.Fatalf("expected ErrServiceGraphInvalid, got %v", err)
	}
	var invalid *ServiceGraphInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected typed ServiceGraphInvalidError, got %T", err)
	}
	if len(invalid.Issues) == 0 || invalid.Issues[0].Code != "duplicate_provider" {
		t.Fatalf("issues not carried: %v", invalid.Issues)
	}
	if !strings.Contains(err.Error(), "duplicate_provider") {
		t.Fatalf("error text should name the issue code, got %q", err.Error())
	}
}

func TestSelectProviderCandidatesRejectsUnselectedBinding(t *testing.T) {
	resolution := testGraph(t, []ServiceDescriptor{
		testDescriptor("provider.a", "memory.search", 10),
	}, ServiceSelectionAll)
	snapshot := testSnapshot(t, globalBinding("memory.search", "provider.ghost"))
	_, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if !errors.Is(err, ErrProviderNotSelected) {
		t.Fatalf("expected ErrProviderNotSelected, got %v", err)
	}
	var notSelected *ProviderNotSelectedError
	if !errors.As(err, &notSelected) {
		t.Fatalf("expected typed ProviderNotSelectedError, got %T", err)
	}
	if notSelected.ProviderID != "provider.ghost" || len(notSelected.Candidates) != 1 || notSelected.Candidates[0] != "provider.a" {
		t.Fatalf("unexpected error payload: %+v", notSelected)
	}
	if notSelected.Revision != 4 {
		t.Fatalf("error should carry revision, got %d", notSelected.Revision)
	}
}

func TestSelectProviderCandidatesPropagatesBindingMiss(t *testing.T) {
	resolution := testGraph(t, []ServiceDescriptor{testDescriptor("provider.a", "memory.search", 10)}, ServiceSelectionAll)
	snapshot := testSnapshot(t, deviceBinding("memory.search", "space-1", "device-1", "provider.a"))
	_, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search", SpaceID: "space-2", DeviceID: "device-1"}, "memory.search")
	if !errors.Is(err, ErrBindingNotFound) {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
	_, err = SelectProviderCandidates(resolution, nil, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if !errors.Is(err, ErrSnapshotEmpty) {
		t.Fatalf("expected ErrSnapshotEmpty for nil snapshot, got %v", err)
	}
	_, err = SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "  ")
	if !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("expected ErrBindingInvalid for empty service id, got %v", err)
	}
}

func TestSelectProviderCandidatesReportsMissingService(t *testing.T) {
	resolution := testGraph(t, []ServiceDescriptor{testDescriptor("provider.a", "memory.search", 10)}, ServiceSelectionAll)
	snapshot := testSnapshot(t, globalBinding("memory.delete", "provider.a"))
	_, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.delete"}, "memory.delete")
	if !errors.Is(err, ErrProviderNotSelected) {
		t.Fatalf("expected ErrProviderNotSelected, got %v", err)
	}
	var notSelected *ProviderNotSelectedError
	if !errors.As(err, &notSelected) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if len(notSelected.Candidates) != 0 || notSelected.ProviderID != "provider.a" {
		t.Fatalf("unexpected payload %+v", notSelected)
	}
	if !strings.Contains(err.Error(), "candidates: ") {
		t.Fatalf("error should expose empty candidate list, got %q", err.Error())
	}
}

func TestSelectProviderCandidatesReturnsCopies(t *testing.T) {
	resolution := testGraph(t, []ServiceDescriptor{testDescriptor("provider.a", "memory.search", 10)}, ServiceSelectionAll)
	snapshot := testSnapshot(t, globalBinding("memory.search", "provider.a"))
	selected, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if err != nil {
		t.Fatalf("SelectProviderCandidates: %v", err)
	}
	selected.Candidates[0].Scopes[0] = ServiceScopeDevice
	selected.Selected.ProviderID = "mutated"
	again, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if err != nil {
		t.Fatalf("SelectProviderCandidates again: %v", err)
	}
	if again.Candidates[0].Scopes[0] != ServiceScopeGlobal || again.Selected.ProviderID != "provider.a" {
		t.Fatalf("graph state was mutated through returned candidates")
	}
}

type providerSelectionGoldenCandidate struct {
	ProviderID string `json:"providerId"`
	Priority   int    `json:"priority"`
}

type providerSelectionGolden struct {
	SchemaVersion int                                `json:"schemaVersion"`
	ServiceID     string                             `json:"serviceId"`
	Revision      uint64                             `json:"revision"`
	Scope         BindingScope                       `json:"scope"`
	Candidates    []providerSelectionGoldenCandidate `json:"candidates"`
	Selected      string                             `json:"selected"`
	Trail         []string                           `json:"trail"`
}

func providerSelectionFixture(t *testing.T) providerSelectionGolden {
	t.Helper()
	resolution := testGraph(t, []ServiceDescriptor{
		testDescriptor("provider.b", "memory.search", 5),
		testDescriptor("provider.a", "memory.search", 10),
	}, ServiceSelectionAll)
	snapshot := testSnapshot(t, globalBinding("memory.search", "provider.b"))
	selected, err := SelectProviderCandidates(resolution, snapshot, BindingRequest{CapabilityID: "memory.search"}, "memory.search")
	if err != nil {
		t.Fatalf("SelectProviderCandidates: %v", err)
	}
	candidates := make([]providerSelectionGoldenCandidate, len(selected.Candidates))
	for index, candidate := range selected.Candidates {
		candidates[index] = providerSelectionGoldenCandidate{ProviderID: candidate.ProviderID, Priority: candidate.Priority}
	}
	return providerSelectionGolden{
		SchemaVersion: 1,
		ServiceID:     selected.ServiceID,
		Revision:      selected.Revision,
		Scope:         selected.Scope,
		Candidates:    candidates,
		Selected:      selected.Selected.ProviderID,
		Trail:         selected.Trail,
	}
}

func TestProviderSelectionMatchesGolden(t *testing.T) {
	path := filepath.Join("testdata", "provider-selection.json")
	actual := providerSelectionFixture(t)
	raw, readErr := os.ReadFile(path)
	if readErr == nil {
		var expected providerSelectionGolden
		if err := json.Unmarshal(raw, &expected); err != nil {
			t.Fatalf("decode golden %s: %v", path, err)
		}
		if mustJSON(t, expected) != mustJSON(t, actual) {
			t.Fatalf("provider selection drifted from golden %s\n--- expected ---\n%s\n--- actual ---\n%s", path, mustJSON(t, expected), mustJSON(t, actual))
		}
		return
	}
	if !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("read golden %s: %v", path, readErr)
	}
	writeGolden(t, path, actual)
}
