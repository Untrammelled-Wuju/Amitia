package plugincontract

import (
	"errors"
	"fmt"
	"testing"
)

func routeFallbackRow(t *testing.T, operation RouteOperation, outcome RouteOutcome, alternatives int) RouteFallbackDecision {
	t.Helper()
	decision, err := DecideRouteFallback(operation, outcome, alternatives)
	if err != nil {
		t.Fatalf("DecideRouteFallback(%s, %s, %d): %v", operation, outcome, alternatives, err)
	}
	return decision
}

func TestRouteFallbackReadUnavailableSwitches(t *testing.T) {
	decision := routeFallbackRow(t, RouteOperationRead, RouteOutcomeUnavailable, 1)
	if decision.Action != RouteFallbackSwitch || !decision.SwitchAllowed || decision.RetryAllowed {
		t.Fatalf("decision = %+v", decision)
	}
	fmt.Printf(
		"route fallback read unavailable action=%s alternatives=%d switchAllowed=%v retryAllowed=%v\n",
		decision.Action, decision.Alternatives, decision.SwitchAllowed, decision.RetryAllowed,
	)
}

func TestRouteFallbackReadUnavailableWithoutAlternate(t *testing.T) {
	decision := routeFallbackRow(t, RouteOperationRead, RouteOutcomeUnavailable, 0)
	if decision.Action != RouteFallbackStop || decision.SwitchAllowed {
		t.Fatalf("decision = %+v", decision)
	}
	fmt.Printf(
		"route fallback read unavailable empty action=%s alternatives=%d reason=%s\n",
		decision.Action, decision.Alternatives, decision.Reason,
	)
}

func TestRouteFallbackWriteUnavailableStops(t *testing.T) {
	decision := routeFallbackRow(t, RouteOperationWrite, RouteOutcomeUnavailable, 1)
	if decision.Action != RouteFallbackStop || decision.SwitchAllowed || decision.RetryAllowed {
		t.Fatalf("decision = %+v", decision)
	}
	fmt.Printf(
		"route fallback write unavailable action=%s switchAllowed=%v retryAllowed=%v reason=%s\n",
		decision.Action, decision.SwitchAllowed, decision.RetryAllowed, decision.Reason,
	)
}

func TestRouteFallbackWriteFailedStops(t *testing.T) {
	decision := routeFallbackRow(t, RouteOperationWrite, RouteOutcomeFailed, 1)
	if decision.Action != RouteFallbackStop || decision.SwitchAllowed {
		t.Fatalf("decision = %+v", decision)
	}
	fmt.Printf(
		"route fallback write failed action=%s switchAllowed=%v reason=%s\n",
		decision.Action, decision.SwitchAllowed, decision.Reason,
	)
}

func TestRouteFallbackUnknownWriteNeverSwitches(t *testing.T) {
	decision := routeFallbackRow(t, RouteOperationWrite, RouteOutcomeUnknown, 3)
	if decision.Action != RouteFallbackStop || decision.SwitchAllowed || decision.RetryAllowed {
		t.Fatalf("unknown write decision = %+v", decision)
	}
	if decision.Alternatives != 3 {
		t.Fatalf("alternatives = %d want 3", decision.Alternatives)
	}
	fmt.Printf(
		"route fallback write unknown action=%s alternatives=%d switchAllowed=%v retryAllowed=%v reason=%s\n",
		decision.Action, decision.Alternatives, decision.SwitchAllowed, decision.RetryAllowed, decision.Reason,
	)
}

func TestRouteFallbackReadFailedStops(t *testing.T) {
	decision := routeFallbackRow(t, RouteOperationRead, RouteOutcomeFailed, 1)
	if decision.Action != RouteFallbackStop || decision.SwitchAllowed {
		t.Fatalf("decision = %+v", decision)
	}
	fmt.Printf(
		"route fallback read failed action=%s switchAllowed=%v\n",
		decision.Action, decision.SwitchAllowed,
	)
}

func TestRouteFallbackTruthTable(t *testing.T) {
	operations := []RouteOperation{RouteOperationRead, RouteOperationWrite}
	outcomes := []RouteOutcome{RouteOutcomeSuccess, RouteOutcomeFailed, RouteOutcomeUnavailable, RouteOutcomeUnknown}
	rows := 0
	writeSwitches := 0
	unknownWriteSwitches := 0
	for _, operation := range operations {
		for _, outcome := range outcomes {
			decision := routeFallbackRow(t, operation, outcome, 2)
			rows++
			if operation == RouteOperationWrite && decision.SwitchAllowed {
				writeSwitches++
			}
			if operation == RouteOperationWrite && outcome == RouteOutcomeUnknown && decision.SwitchAllowed {
				unknownWriteSwitches++
			}
		}
	}
	if writeSwitches != 0 || unknownWriteSwitches != 0 {
		t.Fatalf("writeSwitches = %d unknownWriteSwitches = %d want 0/0", writeSwitches, unknownWriteSwitches)
	}
	if rows != 8 {
		t.Fatalf("rows = %d want 8", rows)
	}
	fmt.Printf(
		"route fallback table rows=%d writeSwitches=%d unknownWritesSwitch=%d\n",
		rows, writeSwitches, unknownWriteSwitches,
	)
}

func TestRouteFallbackInvalidInput(t *testing.T) {
	_, err := DecideRouteFallback(RouteOperation("mutate"), RouteOutcomeFailed, 1)
	var invalid *RouteFallbackInvalidError
	if !errors.As(err, &invalid) || !errors.Is(err, ErrRouteFallbackInvalid) {
		t.Fatalf("error = %v want *RouteFallbackInvalidError", err)
	}
	if invalid.Field != "operation" || invalid.Value != "mutate" {
		t.Fatalf("invalid = %+v", invalid)
	}
	_, err = DecideRouteFallback(RouteOperationRead, RouteOutcome("partial"), 1)
	if !errors.As(err, &invalid) || invalid.Field != "outcome" {
		t.Fatalf("outcome error = %v", err)
	}
	_, err = DecideRouteFallback(RouteOperationRead, RouteOutcomeFailed, -1)
	if !errors.As(err, &invalid) || invalid.Field != "alternatives" {
		t.Fatalf("alternatives error = %v", err)
	}
	fmt.Printf(
		"route fallback invalid fields=%s,%s,%s error=%s\n",
		"operation", "outcome", "alternatives", "ErrRouteFallbackInvalid",
	)
}
