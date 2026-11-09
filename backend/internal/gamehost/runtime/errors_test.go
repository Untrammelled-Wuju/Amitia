package runtime

import (
	"fmt"
	"testing"
)

func TestIsTopologyErrorSeesWrappedErrors(t *testing.T) {
	base := NewTopologyError(ErrNotFound, "runtime not found")

	if !IsTopologyError(base, ErrNotFound) {
		t.Fatal("direct topology error must still match")
	}
	wrapped := fmt.Errorf("topology snapshot: %w", base)
	if !IsTopologyError(wrapped, ErrNotFound) {
		t.Fatalf("wrapped topology error was not recognised: %v", wrapped)
	}
	if IsTopologyError(wrapped, ErrInvalidArgument) {
		t.Fatal("wrapped error reported the wrong code")
	}
	if IsTopologyError(fmt.Errorf("plain failure"), ErrNotFound) {
		t.Fatal("plain error must not match a topology code")
	}
}

func TestIsExecutionErrorSeesWrappedErrors(t *testing.T) {
	base := NewExecutionError(ErrInvalidArgument, "rt", "plugin", "", "", "bad input")
	wrapped := fmt.Errorf("start runtime: %w", base)

	if !IsExecutionError(wrapped, ErrInvalidArgument) {
		t.Fatalf("wrapped execution error was not recognised: %v", wrapped)
	}
	if IsExecutionError(wrapped, ErrNotFound) {
		t.Fatal("wrapped execution error reported the wrong code")
	}
}

func TestIsRuntimeRestartErrorSeesWrappedErrors(t *testing.T) {
	base := &RuntimeRestartError{Code: ErrRestartFailed, RuntimeID: "rt"}
	wrapped := fmt.Errorf("restart: %w", base)

	if !IsRuntimeRestartError(wrapped, ErrRestartFailed) {
		t.Fatalf("wrapped restart error was not recognised: %v", wrapped)
	}
	if IsRuntimeRestartError(wrapped, ErrNotFound) {
		t.Fatal("wrapped restart error reported the wrong code")
	}
}
