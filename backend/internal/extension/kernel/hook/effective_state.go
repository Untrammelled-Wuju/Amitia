package hook

type EffectiveHookState string

const (
	StateActive             EffectiveHookState = "active"
	StateDisabled           EffectiveHookState = "disabled"
	StateExtensionDisabled  EffectiveHookState = "extension_disabled"
	StateRuntimeUnavailable EffectiveHookState = "runtime_unavailable"
	StatePointUnavailable   EffectiveHookState = "point_unavailable"
	StateContractIncompat   EffectiveHookState = "contract_incompatible"
	StatePermissionDenied   EffectiveHookState = "permission_denied"
	StateScopeDenied        EffectiveHookState = "scope_denied"
	StateCircuitOpen        EffectiveHookState = "circuit_open"
	StateCircuitHalfOpen    EffectiveHookState = "half_open"
	StateOrderingConflict   EffectiveHookState = "ordering_conflict"
	StateHandlerInvalid     EffectiveHookState = "handler_invalid"
)

type EffectiveStateResult struct {
	State      EffectiveHookState `json:"state"`
	Reason     string             `json:"reason"`
	Diagnostic string             `json:"diagnostic,omitempty"`
}

type EffectiveStateInput struct {
	Contribution    HookContributionDefinition
	Point           *HookPointDefinition
	CircuitState    CircuitState
	ExtensionActive bool
	RuntimeReady    bool
	PermissionOK    bool
	ScopeOK         bool
	PlanExists      bool
	InPlan          bool
	PlanCycle       bool
}

func ComputeEffectiveState(input EffectiveStateInput) EffectiveStateResult {
	c := input.Contribution

	if !input.ExtensionActive {
		return EffectiveStateResult{
			State:  StateExtensionDisabled,
			Reason: "extension is not active",
		}
	}

	if !c.Enabled {
		return EffectiveStateResult{
			State:  StateDisabled,
			Reason: "contribution is disabled",
		}
	}

	if input.Point == nil {
		return EffectiveStateResult{
			State:  StatePointUnavailable,
			Reason: "hook point not found: " + c.HookPointID,
		}
	}

	if c.ContractVersion != input.Point.ContractVersion {
		return EffectiveStateResult{
			State:      StateContractIncompat,
			Reason:     "contract version mismatch",
			Diagnostic: formatContractDiag(c.ContractVersion, input.Point.ContractVersion),
		}
	}

	switch input.CircuitState {
	case CircuitOpen:
		return EffectiveStateResult{
			State:  StateCircuitOpen,
			Reason: "circuit is open due to consecutive failures",
		}
	case CircuitHalfOpen:
		return EffectiveStateResult{
			State:  StateCircuitHalfOpen,
			Reason: "circuit is half-open, awaiting probe",
		}
	}

	if !input.RuntimeReady {
		return EffectiveStateResult{
			State:  StateRuntimeUnavailable,
			Reason: "handler runtime is not ready",
		}
	}

	if !input.PermissionOK {
		return EffectiveStateResult{
			State:  StatePermissionDenied,
			Reason: "contribution permission requirements not satisfied",
		}
	}

	if !input.ScopeOK {
		return EffectiveStateResult{
			State:  StateScopeDenied,
			Reason: "contribution scope rule does not match current context",
		}
	}

	if !input.PlanExists {
		return EffectiveStateResult{
			State:  StatePointUnavailable,
			Reason: "no compiled plan exists for this hook point",
		}
	}

	if !input.InPlan {
		return EffectiveStateResult{
			State:  StateHandlerInvalid,
			Reason: "contribution not included in current plan (possibly filtered)",
		}
	}

	if input.PlanCycle {
		return EffectiveStateResult{
			State:  StateOrderingConflict,
			Reason: "ordering cycle detected among contributions",
		}
	}

	return EffectiveStateResult{
		State:  StateActive,
		Reason: "active",
	}
}

func formatContractDiag(contribVersion, pointVersion int) string {
	return "contribution contract " + itoa(contribVersion) + " != point contract " + itoa(pointVersion)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
