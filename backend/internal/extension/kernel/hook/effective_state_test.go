package hook

import (
	"testing"
)

func TestComputeEffectiveState_Active(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState:    CircuitClosed,
		ExtensionActive: true,
		RuntimeReady:    true,
		PermissionOK:    true,
		ScopeOK:         true,
		PlanExists:      true,
		InPlan:          true,
	})

	if result.State != StateActive {
		t.Errorf("expected active, got %s", result.State)
	}
}

func TestComputeEffectiveState_Disabled(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         false,
		},
		ExtensionActive: true,
	})

	if result.State != StateDisabled {
		t.Errorf("expected disabled, got %s", result.State)
	}
}

func TestComputeEffectiveState_ExtensionDisabled(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			Enabled: true,
		},
		ExtensionActive: false,
	})

	if result.State != StateExtensionDisabled {
		t.Errorf("expected extension_disabled, got %s", result.State)
	}
}

func TestComputeEffectiveState_PointUnavailable(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			Enabled: true,
		},
		ExtensionActive: true,
		Point:           nil,
	})

	if result.State != StatePointUnavailable {
		t.Errorf("expected point_unavailable, got %s", result.State)
	}
}

func TestComputeEffectiveState_ContractIncompat(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			Enabled:         true,
			ContractVersion: 2,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			ContractVersion: 1,
		},
	})

	if result.State != StateContractIncompat {
		t.Errorf("expected contract_incompatible, got %s", result.State)
	}
}

func TestComputeEffectiveState_CircuitOpen(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitOpen,
	})

	if result.State != StateCircuitOpen {
		t.Errorf("expected circuit_open, got %s", result.State)
	}
}

func TestComputeEffectiveState_CircuitHalfOpen(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitHalfOpen,
	})

	if result.State != StateCircuitHalfOpen {
		t.Errorf("expected half_open, got %s", result.State)
	}
}

func TestComputeEffectiveState_RuntimeUnavailable(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitClosed,
		RuntimeReady: false,
	})

	if result.State != StateRuntimeUnavailable {
		t.Errorf("expected runtime_unavailable, got %s", result.State)
	}
}

func TestComputeEffectiveState_PermissionDenied(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitClosed,
		RuntimeReady: true,
		PermissionOK: false,
	})

	if result.State != StatePermissionDenied {
		t.Errorf("expected permission_denied, got %s", result.State)
	}
}

func TestComputeEffectiveState_ScopeDenied(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitClosed,
		RuntimeReady: true,
		PermissionOK: true,
		ScopeOK:      false,
	})

	if result.State != StateScopeDenied {
		t.Errorf("expected scope_denied, got %s", result.State)
	}
}

func TestComputeEffectiveState_OrderingConflict(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitClosed,
		RuntimeReady: true,
		PermissionOK: true,
		ScopeOK:      true,
		PlanExists:   true,
		InPlan:       true,
		PlanCycle:    true,
	})

	if result.State != StateOrderingConflict {
		t.Errorf("expected ordering_conflict, got %s", result.State)
	}
}

func TestComputeEffectiveState_HandlerInvalid(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution: HookContributionDefinition{
			ContributionID:  "c1",
			HookPointID:     "pt/1",
			ContractVersion: 1,
			Enabled:         true,
		},
		ExtensionActive: true,
		Point: &HookPointDefinition{
			HookPointID:     "pt/1",
			ContractVersion: 1,
		},
		CircuitState: CircuitClosed,
		RuntimeReady: true,
		PermissionOK: true,
		ScopeOK:      true,
		PlanExists:   true,
		InPlan:       false,
	})

	if result.State != StateHandlerInvalid {
		t.Errorf("expected handler_invalid, got %s", result.State)
	}
}

func TestComputeEffectiveState_ReasonsPresent(t *testing.T) {
	result := ComputeEffectiveState(EffectiveStateInput{
		Contribution:    HookContributionDefinition{Enabled: false},
		ExtensionActive: true,
	})
	if result.Reason == "" {
		t.Error("expected non-empty reason for disabled state")
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{
		0:    "0",
		1:    "1",
		-1:   "-1",
		100:  "100",
		-100: "-100",
		999:  "999",
	}
	for n, expected := range cases {
		got := itoa(n)
		if got != expected {
			t.Errorf("itoa(%d) = %s, want %s", n, got, expected)
		}
	}
}
