package plugincontract

import (
	"errors"
	"fmt"
	"testing"
)

func scopeContract(t *testing.T, providerID, serviceID string, claims ScopeClaims) ProviderScopeContract {
	t.Helper()
	contract := ProviderScopeContract{ProviderID: providerID, ServiceID: serviceID, Claims: claims}
	if err := contract.validate(); err != nil {
		t.Fatalf("fixture contract %s: %v", providerID, err)
	}
	return contract
}

func TestScopeDeviceBinding(t *testing.T) {
	contract := scopeContract(t, "local.device", "memory.search", ScopeClaims{ScopeDimensionDevice: "device-a"})
	match, err := contract.Matches(ScopeRequirement{ScopeDimensionDevice: "device-a"})
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if !match.Matched {
		t.Fatalf("match = %+v want matched", match)
	}
	fmt.Printf("route scope device provider=%s device=%s matched=%v\n", match.ProviderID, contract.Claims[ScopeDimensionDevice], match.Matched)
}

func TestScopeSpaceBinding(t *testing.T) {
	contract := scopeContract(t, "space.service", "memory.search", ScopeClaims{ScopeDimensionSpace: "space-1"})
	match, err := contract.Matches(ScopeRequirement{ScopeDimensionSpace: "space-1"})
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if !match.Matched {
		t.Fatalf("match = %+v want matched", match)
	}
	fmt.Printf("route scope space provider=%s space=%s matched=%v\n", match.ProviderID, contract.Claims[ScopeDimensionSpace], match.Matched)
}

func TestScopeRoleAndSurfaceBinding(t *testing.T) {
	contract := scopeContract(t, "role.butler", "memory.search", ScopeClaims{
		ScopeDimensionRole:    "butler",
		ScopeDimensionSurface: "desktop",
	})
	match, err := contract.Matches(ScopeRequirement{ScopeDimensionRole: "butler", ScopeDimensionSurface: "desktop"})
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if !match.Matched {
		t.Fatalf("match = %+v want matched", match)
	}
	fmt.Printf(
		"route scope role surface provider=%s role=%s surface=%s matched=%v\n",
		match.ProviderID, contract.Claims[ScopeDimensionRole], contract.Claims[ScopeDimensionSurface], match.Matched,
	)
}

func TestScopeRemoteOfflineNeverRebindsLocal(t *testing.T) {
	remote := scopeContract(t, "remote.7", "memory.search", ScopeClaims{ScopeDimensionDevice: "device-7"})
	local := scopeContract(t, "local.device", "memory.search", ScopeClaims{ScopeDimensionDevice: "local"})
	core := scopeContract(t, "builtin.core/core/memory.search", "memory.search", ScopeClaims{ScopeDimensionCore: "local-core"})
	selected, _, err := SelectScopedProvider(
		[]ProviderScopeContract{remote, local, core},
		ScopeRequirement{ScopeDimensionDevice: "device-7"},
		func(providerID string) bool { return false },
	)
	if err == nil {
		t.Fatalf("offline remote must fail, got provider %s", selected.ProviderID)
	}
	var unavailable *ScopeProviderUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("error = %v want *ScopeProviderUnavailableError", err)
	}
	if !errors.Is(err, ErrScopeProviderUnavailable) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	if len(unavailable.Candidates) != 1 || unavailable.Candidates[0] != "remote.7" {
		t.Fatalf("candidates = %+v want only remote.7", unavailable.Candidates)
	}
	localRejected := false
	for _, reason := range unavailable.Reasons {
		if reason == "local.device=claim_mismatch" || reason == "builtin.core/core/memory.search=core_cannot_serve_device" {
			localRejected = true
		}
	}
	if !localRejected {
		t.Fatalf("reasons = %+v want the local providers rejected", unavailable.Reasons)
	}
	fmt.Printf(
		"route scope remote offline requirement=device:device-7 selected= error=%v candidates=%d localRejected=%v\n",
		"ErrScopeProviderUnavailable", len(unavailable.Candidates), localRejected,
	)
}

func TestScopeRemoteOnlineSelectsRemote(t *testing.T) {
	remote := scopeContract(t, "remote.7", "memory.search", ScopeClaims{ScopeDimensionDevice: "device-7"})
	local := scopeContract(t, "local.device", "memory.search", ScopeClaims{ScopeDimensionDevice: "local"})
	selected, match, err := SelectScopedProvider(
		[]ProviderScopeContract{local, remote},
		ScopeRequirement{ScopeDimensionDevice: "device-7"},
		func(providerID string) bool { return providerID == "remote.7" },
	)
	if err != nil {
		t.Fatalf("SelectScopedProvider: %v", err)
	}
	if selected.ProviderID != "remote.7" || !match.Matched {
		t.Fatalf("selected = %s match = %+v", selected.ProviderID, match)
	}
	fmt.Printf("route scope remote online requirement=device:device-7 selected=%s matched=%v\n", selected.ProviderID, match.Matched)
}

func TestScopeCoreCannotServeDevice(t *testing.T) {
	core := scopeContract(t, "builtin.core/core/memory.search", "memory.search", ScopeClaims{ScopeDimensionCore: "local-core"})
	match, err := core.Matches(ScopeRequirement{ScopeDimensionDevice: "device-7"})
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if match.Matched || match.Reason != "core_cannot_serve_device" || match.Dimension != ScopeDimensionDevice {
		t.Fatalf("match = %+v want core_cannot_serve_device", match)
	}
	fmt.Printf(
		"route scope core cannot serve device provider=%s device=device-7 matched=%v reason=%s\n",
		match.ProviderID, match.Matched, match.Reason,
	)
}

func TestScopeMissingClaimDoesNotMatch(t *testing.T) {
	contract := scopeContract(t, "device.only", "memory.search", ScopeClaims{ScopeDimensionDevice: "device-7"})
	match, err := contract.Matches(ScopeRequirement{ScopeDimensionDevice: "device-7", ScopeDimensionRole: "butler"})
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if match.Matched || match.Reason != "missing_claim" || match.Dimension != ScopeDimensionRole {
		t.Fatalf("match = %+v want missing_claim on role", match)
	}
	_, _, selectErr := SelectScopedProvider(
		[]ProviderScopeContract{contract},
		ScopeRequirement{ScopeDimensionDevice: "device-7", ScopeDimensionRole: "butler"},
		nil,
	)
	if !errors.Is(selectErr, ErrScopeNoMatch) {
		t.Fatalf("select error = %v want ErrScopeNoMatch", selectErr)
	}
	fmt.Printf(
		"route scope missing claim provider=%s dimension=%s matched=%v reason=%s error=%v\n",
		match.ProviderID, match.Dimension, match.Matched, match.Reason, "ErrScopeNoMatch",
	)
}

func TestScopeContractInvalidDimensions(t *testing.T) {
	contract := ProviderScopeContract{
		ProviderID: "weird.provider",
		ServiceID:  "memory.search",
		Claims:     ScopeClaims{ScopeDimension("platform"): "linux"},
	}
	_, err := contract.Matches(ScopeRequirement{ScopeDimensionDevice: "device-7"})
	if err == nil {
		t.Fatalf("unknown dimension must fail")
	}
	var invalid *ScopeContractInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v want *ScopeContractInvalidError", err)
	}
	if !errors.Is(err, ErrScopeContractInvalid) {
		t.Fatalf("error lost the sentinel: %v", err)
	}
	conflicted := ProviderScopeContract{
		ProviderID: "conflicted.provider",
		ServiceID:  "memory.search",
		Claims:     ScopeClaims{ScopeDimensionCore: "local-core", ScopeDimensionDevice: "device-7"},
	}
	_, conflictErr := conflicted.Matches(ScopeRequirement{ScopeDimensionDevice: "device-7"})
	if !errors.Is(conflictErr, ErrScopeContractInvalid) {
		t.Fatalf("core+device error = %v want ErrScopeContractInvalid", conflictErr)
	}
	fmt.Printf(
		"route scope invalid field=%s value=%s error=%v coreDeviceConflict=%v\n",
		invalid.Field, invalid.Value, "ErrScopeContractInvalid", errors.Is(conflictErr, ErrScopeContractInvalid),
	)
}
