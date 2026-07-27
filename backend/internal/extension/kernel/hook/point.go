package hook

import (
	"encoding/json"
	"time"
)

type HookPhase string

const (
	PhaseBefore    HookPhase = "before"
	PhaseFilter    HookPhase = "filter"
	PhaseTransform HookPhase = "transform"
	PhaseObserve   HookPhase = "observe"
	PhaseAfter     HookPhase = "after"
)

func (p HookPhase) Valid() bool {
	switch p {
	case PhaseBefore, PhaseFilter, PhaseTransform, PhaseObserve, PhaseAfter:
		return true
	}
	return false
}

func (p HookPhase) AllowedDecisions() []HookDecision {
	switch p {
	case PhaseBefore:
		return []HookDecision{DecisionContinue, DecisionReject, DecisionReplace}
	case PhaseFilter:
		return []HookDecision{DecisionAllow, DecisionDeny, DecisionSkip}
	case PhaseTransform:
		return []HookDecision{DecisionContinue, DecisionReplace}
	case PhaseObserve:
		return []HookDecision{DecisionContinue}
	case PhaseAfter:
		return []HookDecision{DecisionContinue, DecisionReplace}
	}
	return nil
}

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type MutationRule struct {
	Path          string         `json:"path"`
	Operations    []string       `json:"operations"`
	ConflictMode  ConflictMode   `json:"conflictMode"`
	ValueSchema   json.RawMessage `json:"valueSchema,omitempty"`
	OwnerOnly     bool           `json:"ownerOnly,omitempty"`
}

type ConflictMode string

const (
	ConflictLastWriterWins ConflictMode = "last_writer_wins"
	ConflictExclusive      ConflictMode = "exclusive"
	ConflictFirstWriterWins ConflictMode = "first_writer_wins"
	ConflictMerge          ConflictMode = "merge"
)

type HookFailurePolicy struct {
	OnTimeout          string `json:"onTimeout"`
	OnRuntimeError     string `json:"onRuntimeError"`
	OnInvalidResult    string `json:"onInvalidResult"`
	OnUnavailable      string `json:"onUnavailable"`
	OnPermissionDenied string `json:"onPermissionDenied"`
}

const (
	FailureFailOpen            = "fail_open"
	FailureFailClosed          = "fail_closed"
	FailureSkip                = "skip"
	FailureDisableContribution = "disable_contribution"
	FailureOpenCircuit         = "open_circuit"
)

func DefaultFailurePolicy() HookFailurePolicy {
	return HookFailurePolicy{
		OnTimeout:          FailureFailOpen,
		OnRuntimeError:     FailureFailOpen,
		OnInvalidResult:    FailureFailOpen,
		OnUnavailable:      FailureSkip,
		OnPermissionDenied: FailureSkip,
	}
}

func StrictFailurePolicy() HookFailurePolicy {
	return HookFailurePolicy{
		OnTimeout:          FailureFailClosed,
		OnRuntimeError:     FailureFailClosed,
		OnInvalidResult:    FailureFailClosed,
		OnUnavailable:      FailureFailClosed,
		OnPermissionDenied: FailureFailClosed,
	}
}

func (p HookFailurePolicy) IsStricterOrEqual(other HookFailurePolicy) bool {
	looseness := func(s string) int {
		switch s {
		case FailureFailClosed:
			return 0
		case FailureOpenCircuit:
			return 1
		case FailureDisableContribution:
			return 2
		case FailureSkip:
			return 3
		case FailureFailOpen:
			return 4
		}
		return 4
	}
	return looseness(p.OnTimeout) <= looseness(other.OnTimeout) &&
		looseness(p.OnRuntimeError) <= looseness(other.OnRuntimeError) &&
		looseness(p.OnInvalidResult) <= looseness(other.OnInvalidResult) &&
		looseness(p.OnUnavailable) <= looseness(other.OnUnavailable) &&
		looseness(p.OnPermissionDenied) <= looseness(other.OnPermissionDenied)
}

type HookExecutionPolicy struct {
	Mode          string `json:"mode"`
	Parallelism   int    `json:"parallelism"`
	StopOnDeny    bool   `json:"stopOnDeny"`
	StopOnFailure bool   `json:"stopOnFailure"`
	ApplyMode     string `json:"applyMode"`
}

const (
	ExecModeSequential    = "sequential"
	ApplyModeSequential   = "sequential_apply"
	ApplyModeCollectThen  = "collect_then_apply"
)

func DefaultExecutionPolicy() HookExecutionPolicy {
	return HookExecutionPolicy{
		Mode:          ExecModeSequential,
		Parallelism:   1,
		StopOnDeny:    true,
		StopOnFailure: false,
		ApplyMode:     ApplyModeSequential,
	}
}

type HookPointDefinition struct {
	HookPointID      string                 `json:"hookPointId"`
	ContractVersion  int                    `json:"contractVersion"`
	Description      string                 `json:"description"`
	SupportedPhases  []HookPhase            `json:"supportedPhases"`
	InputSchema      json.RawMessage        `json:"inputSchema"`
	ResultSchema     json.RawMessage        `json:"resultSchema"`
	AllowedMutations []MutationRule         `json:"allowedMutations"`
	FailurePolicy    HookFailurePolicy      `json:"failurePolicy"`
	ExecutionPolicy  HookExecutionPolicy    `json:"executionPolicy"`
	MaxHandlers      int                    `json:"maxHandlers"`
	DefaultTimeout   time.Duration          `json:"defaultTimeout"`
	MaxTimeout       time.Duration          `json:"maxTimeout"`
	MaxPayloadBytes  int64                  `json:"maxPayloadBytes"`
	MaxResultBytes   int64                  `json:"maxResultBytes"`
	RiskLevel        RiskLevel              `json:"riskLevel"`
	SensitiveFields  []string               `json:"sensitiveFields"`
	RequiredContext  []string               `json:"requiredContext"`
	ThirdPartyAllowed bool                  `json:"thirdPartyAllowed"`
}

func (d HookPointDefinition) SupportsPhase(phase HookPhase) bool {
	for _, p := range d.SupportedPhases {
		if p == phase {
			return true
		}
	}
	return false
}

func (d HookPointDefinition) FindMutationRule(path string) (MutationRule, bool) {
	for _, r := range d.AllowedMutations {
		if matchPath(r.Path, path) {
			return r, true
		}
	}
	return MutationRule{}, false
}

func (d HookPointDefinition) IsSensitive(path string) bool {
	for _, sf := range d.SensitiveFields {
		if matchPath(sf, path) {
			return true
		}
	}
	return false
}

func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '/' {
		if len(path) > len(pattern) && path[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}
