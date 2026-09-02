package interaction

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestProviderErrorCanFallbackOnlyForKnownNotExecutedErrors(t *testing.T) {
	t.Parallel()

	fallbackCodes := []string{
		INTERACTION_UNSUPPORTED,
		INTERACTION_UNAVAILABLE,
		INTERACTION_ACTION_UNSUPPORTED,
		INTERACTION_ROOT_UNAVAILABLE,
		INTERACTION_ADB_UNAVAILABLE,
		INTERACTION_SHIZUKU_UNAVAILABLE,
		INTERACTION_NATIVE_HOST_UNAVAILABLE,
		INTERACTION_DISPLAY_UNAVAILABLE,
	}
	for _, code := range fallbackCodes {
		code := code
		t.Run("fallback_"+code, func(t *testing.T) {
			t.Parallel()
			if !providerErrorCanFallback(&Error{Code: code, Message: "not executed"}) {
				t.Fatalf("expected %s to allow fallback", code)
			}
		})
	}

	stopCodes := []string{
		INTERACTION_TIMEOUT,
		INTERACTION_CANCELLED,
		INTERACTION_CONTEXT_CHANGED,
		INTERACTION_OUTCOME_UNKNOWN,
		INTERACTION_NODE_STALE,
		INTERACTION_ACTION_FAILED,
		INTERACTION_VERIFICATION_FAILED,
	}
	for _, code := range stopCodes {
		code := code
		t.Run("stop_"+code, func(t *testing.T) {
			t.Parallel()
			if providerErrorCanFallback(&Error{Code: code, Message: "execution may have happened"}) {
				t.Fatalf("expected %s to stop fallback", code)
			}
		})
	}

	if providerErrorCanFallback(nil) {
		t.Fatal("nil error must not allow fallback")
	}
	if providerErrorCanFallback(context.Canceled) {
		t.Fatal("context cancellation must not allow fallback")
	}
	if providerErrorCanFallback(context.DeadlineExceeded) {
		t.Fatal("deadline exceeded must not allow fallback")
	}
	if providerErrorCanFallback(errors.New("opaque transport error")) {
		t.Fatal("unknown errors must fail closed and stop fallback")
	}
}

func TestExecuteRankedProviderFallsBackOnlyWhenFirstProviderDidNotExecute(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, DefaultPolicy())
	var calls []string
	candidates := []providerRouteCandidate{
		{
			name:      "primary",
			strategy:  "primary",
			provider:  struct{}{},
			baseScore: 100,
			execute: func() error {
				calls = append(calls, "primary")
				return &Error{Code: INTERACTION_UNAVAILABLE, Message: "provider was unavailable before execution"}
			},
		},
		{
			name:      "secondary",
			strategy:  "secondary",
			provider:  struct{}{},
			baseScore: 90,
			execute: func() error {
				calls = append(calls, "secondary")
				return nil
			},
		},
	}

	strategy, err := svc.executeRankedProvider(context.Background(), candidates)
	if err != nil {
		t.Fatalf("executeRankedProvider returned error: %v", err)
	}
	if strategy != "secondary" {
		t.Fatalf("strategy = %q, want secondary", strategy)
	}
	if want := []string{"primary", "secondary"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestExecuteRankedProviderDoesNotRepeatAfterUnknownOutcome(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, DefaultPolicy())
	var calls []string
	unknown := &Error{Code: INTERACTION_OUTCOME_UNKNOWN, Message: "transport failed after dispatch"}
	candidates := []providerRouteCandidate{
		{
			name:      "primary",
			strategy:  "primary",
			provider:  struct{}{},
			baseScore: 100,
			execute: func() error {
				calls = append(calls, "primary")
				return unknown
			},
		},
		{
			name:      "secondary",
			strategy:  "secondary",
			provider:  struct{}{},
			baseScore: 90,
			execute: func() error {
				calls = append(calls, "secondary")
				return nil
			},
		},
	}

	strategy, err := svc.executeRankedProvider(context.Background(), candidates)
	if !errors.Is(err, unknown) {
		t.Fatalf("error = %v, want original unknown-outcome error", err)
	}
	if strategy != "" {
		t.Fatalf("strategy = %q, want empty on failure", strategy)
	}
	if want := []string{"primary"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v; secondary provider must not execute", calls, want)
	}
}
