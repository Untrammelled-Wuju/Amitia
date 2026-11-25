package plugincontract

import (
	"errors"
	"fmt"
	"strings"
)

var ErrRouteFallbackInvalid = errors.New("plugincontract: route fallback input invalid")

type RouteOperation string

const (
	RouteOperationRead  RouteOperation = "read"
	RouteOperationWrite RouteOperation = "write"
)

type RouteOutcome string

const (
	RouteOutcomeSuccess     RouteOutcome = "success"
	RouteOutcomeFailed      RouteOutcome = "failed"
	RouteOutcomeUnavailable RouteOutcome = "unavailable"
	RouteOutcomeUnknown     RouteOutcome = "unknown"
)

type RouteFallbackAction string

const (
	RouteFallbackStop   RouteFallbackAction = "stop"
	RouteFallbackSwitch RouteFallbackAction = "switch_provider"
)

type RouteFallbackDecision struct {
	Operation     RouteOperation      `json:"operation"`
	Outcome       RouteOutcome        `json:"outcome"`
	Action        RouteFallbackAction `json:"action"`
	Reason        string              `json:"reason"`
	Alternatives  int                 `json:"alternatives"`
	SwitchAllowed bool                `json:"switchAllowed"`
	RetryAllowed  bool                `json:"retryAllowed"`
}

type RouteFallbackInvalidError struct {
	Field  string
	Value  string
	Reason string
}

func (e *RouteFallbackInvalidError) Error() string {
	return fmt.Sprintf("plugincontract: route fallback field %q value %q: %s", e.Field, e.Value, e.Reason)
}

func (e *RouteFallbackInvalidError) Unwrap() error { return ErrRouteFallbackInvalid }

func DecideRouteFallback(operation RouteOperation, outcome RouteOutcome, alternatives int) (RouteFallbackDecision, error) {
	if alternatives < 0 {
		return RouteFallbackDecision{}, &RouteFallbackInvalidError{Field: "alternatives", Value: fmt.Sprintf("%d", alternatives), Reason: "must not be negative"}
	}
	kind := RouteOperation(strings.TrimSpace(string(operation)))
	switch kind {
	case RouteOperationRead, RouteOperationWrite:
	default:
		return RouteFallbackDecision{}, &RouteFallbackInvalidError{Field: "operation", Value: string(operation), Reason: "want read or write"}
	}
	result := RouteOutcome(strings.TrimSpace(string(outcome)))
	switch result {
	case RouteOutcomeSuccess, RouteOutcomeFailed, RouteOutcomeUnavailable, RouteOutcomeUnknown:
	default:
		return RouteFallbackDecision{}, &RouteFallbackInvalidError{Field: "outcome", Value: string(outcome), Reason: "want success, failed, unavailable or unknown"}
	}
	decision := RouteFallbackDecision{
		Operation:    kind,
		Outcome:      result,
		Action:       RouteFallbackStop,
		Alternatives: alternatives,
	}
	switch {
	case result == RouteOutcomeSuccess:
		decision.Reason = "no fallback needed"
	case kind == RouteOperationWrite && result == RouteOutcomeUnknown:
		decision.Reason = "unknown write result must never switch or re-execute"
	case kind == RouteOperationWrite && result == RouteOutcomeUnavailable:
		decision.Reason = "write provider unavailable must not switch"
	case kind == RouteOperationWrite:
		decision.Reason = "write failures are surfaced to the caller"
	case result == RouteOutcomeUnavailable && alternatives > 0:
		decision.Action = RouteFallbackSwitch
		decision.SwitchAllowed = true
		decision.Reason = "read provider unavailable and an alternate candidate exists"
	case result == RouteOutcomeUnavailable:
		decision.Reason = "read provider unavailable without an alternate candidate"
	default:
		decision.Reason = "read failures are surfaced to the caller"
	}
	return decision, nil
}
