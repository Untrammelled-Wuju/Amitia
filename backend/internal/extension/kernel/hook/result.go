package hook

import (
	"encoding/json"
	"fmt"
)

type HookDecision string

const (
	DecisionContinue HookDecision = "continue"
	DecisionAllow    HookDecision = "allow"
	DecisionDeny     HookDecision = "deny"
	DecisionSkip     HookDecision = "skip"
	DecisionReplace  HookDecision = "replace"
	DecisionReject   HookDecision = "reject"
)

func (d HookDecision) Valid() bool {
	switch d {
	case DecisionContinue, DecisionAllow, DecisionDeny, DecisionSkip, DecisionReplace, DecisionReject:
		return true
	}
	return false
}

func (d HookDecision) AllowedForPhase(phase HookPhase) bool {
	allowed := phase.AllowedDecisions()
	for _, a := range allowed {
		if a == d {
			return true
		}
	}
	return false
}

type MutationOperation struct {
	Operation string          `json:"operation"`
	Path      string          `json:"path"`
	Value     json.RawMessage `json:"value,omitempty"`
}

const (
	MutationReplace = "replace"
	MutationAdd     = "add"
	MutationRemove  = "remove"
)

func ValidMutationOperation(op string) bool {
	switch op {
	case MutationReplace, MutationAdd, MutationRemove:
		return true
	}
	return false
}

type HookResult struct {
	Decision HookDecision        `json:"decision"`
	Patch    []MutationOperation `json:"patch,omitempty"`
	Output   json.RawMessage     `json:"output,omitempty"`
	Metadata map[string]any      `json:"metadata,omitempty"`
}

func ContinueResult() HookResult {
	return HookResult{Decision: DecisionContinue}
}

func DenyResult(reason string) HookResult {
	return HookResult{
		Decision: DecisionDeny,
		Metadata: map[string]any{"reason": reason},
	}
}

func AllowResult() HookResult {
	return HookResult{Decision: DecisionAllow}
}

func SkipResult() HookResult {
	return HookResult{Decision: DecisionSkip}
}

func ReplaceResult(patch []MutationOperation) HookResult {
	return HookResult{
		Decision: DecisionReplace,
		Patch:    patch,
	}
}

func (r HookResult) ValidateSize(maxResultBytes int64) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("hook: marshal result: %w", err)
	}
	if int64(len(data)) > maxResultBytes {
		return NewHookError(ErrCodeHookResultTooLarge, fmt.Sprintf("result %d bytes exceeds max %d", len(data), maxResultBytes))
	}
	return nil
}

func (r HookResult) ValidatePatchLimits(maxOps int, maxPathLen int) error {
	if len(r.Patch) > maxOps {
		return NewHookError(ErrCodeHookTooManyOps, fmt.Sprintf("patch operations %d exceeds max %d", len(r.Patch), maxOps))
	}
	for _, op := range r.Patch {
		if len(op.Path) > maxPathLen {
			return NewHookError(ErrCodeHookPathTooLong, fmt.Sprintf("path length %d exceeds max %d", len(op.Path), maxPathLen))
		}
		if !ValidMutationOperation(op.Operation) {
			return NewHookError(ErrCodeHookResultInvalid, fmt.Sprintf("invalid operation: %s", op.Operation))
		}
	}
	return nil
}

type PipelineResult struct {
	OperationID   string          `json:"operationId"`
	HookPointID   string          `json:"hookPointId"`
	Aborted       bool            `json:"aborted"`
	AbortReason   string          `json:"abortReason,omitempty"`
	Decision      HookDecision    `json:"decision"`
	Transformed   bool            `json:"transformed"`
	FinalPayload  json.RawMessage `json:"finalPayload,omitempty"`
	Executions    []HookExecution `json:"executions"`
	TotalDuration int64           `json:"totalDurationMs"`
	Depth         int             `json:"depth"`
}

type HookExecution struct {
	ContributionID string       `json:"contributionId"`
	ExtensionID    string       `json:"extensionId"`
	Phase          HookPhase    `json:"phase"`
	Sequence       int          `json:"sequence"`
	Status         string       `json:"status"`
	Decision       HookDecision `json:"decision"`
	Error          string       `json:"error,omitempty"`
	ErrorCode      string       `json:"errorCode,omitempty"`
	DurationMs     int64        `json:"durationMs"`
	StartedAt      string       `json:"startedAt"`
	InputHash      string       `json:"inputHash,omitempty"`
	ResultHash     string       `json:"resultHash,omitempty"`
	MutationCount  int          `json:"mutationCount"`
	CircuitState   string       `json:"circuitState,omitempty"`
}

const (
	StatusSuccess     = "success"
	StatusFailed      = "failed"
	StatusSkipped     = "skipped"
	StatusTimeout     = "timeout"
	StatusDenied      = "denied"
	StatusCancelled   = "cancelled"
	StatusCircuitOpen = "circuit_open"
)
