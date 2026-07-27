package wasm_runtime

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
)

type TrapKind string

const (
	TrapUnreachable         TrapKind = "wasm_trap_unreachable"
	TrapMemoryOutOfBounds   TrapKind = "wasm_trap_memory_out_of_bounds"
	TrapIntegerDivideByZero TrapKind = "wasm_trap_integer_divide_by_zero"
	TrapIntegerOverflow     TrapKind = "wasm_trap_integer_overflow"
	TrapIndirectCall        TrapKind = "wasm_trap_indirect_call"
	TrapStackOverflow       TrapKind = "wasm_trap_stack_overflow"
	TrapHostError           TrapKind = "wasm_trap_host_error"
	TrapCancelled           TrapKind = "wasm_trap_cancelled"
	TrapTimeout             TrapKind = "wasm_trap_timeout"
	TrapUnknown             TrapKind = "wasm_trap_unknown"
)

type TrapInfo struct {
	Kind         TrapKind
	Message      string
	Stack        string
	InvocationID string
	ModuleID     string
	ExtensionID  string
}

func (t TrapInfo) Error() string {
	return fmt.Sprintf("wasm trap: %s: %s", t.Kind, t.Message)
}

func ClassifyError(err error) TrapKind {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return TrapTimeout
	}
	if errors.Is(err, context.Canceled) {
		return TrapCancelled
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "unreachable"):
		return TrapUnreachable
	case strings.Contains(lower, "out of bounds") || strings.Contains(lower, "out_of_bounds") || strings.Contains(lower, "memory"):
		return TrapMemoryOutOfBounds
	case strings.Contains(lower, "divide by zero") || strings.Contains(lower, "div by zero"):
		return TrapIntegerDivideByZero
	case strings.Contains(lower, "overflow"):
		return TrapIntegerOverflow
	case strings.Contains(lower, "indirect call") || strings.Contains(lower, "indirect_call"):
		return TrapIndirectCall
	case strings.Contains(lower, "stack overflow") || strings.Contains(lower, "stack_overflow"):
		return TrapStackOverflow
	case strings.Contains(lower, "cancel"):
		return TrapCancelled
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
		return TrapTimeout
	case strings.Contains(lower, "host"):
		return TrapHostError
	default:
		return TrapUnknown
	}
}

func NewTrapInfo(kind TrapKind, msg string, invocationID, moduleID, extensionID string) *TrapInfo {
	return &TrapInfo{
		Kind:         kind,
		Message:      msg,
		Stack:        captureStack(),
		InvocationID: invocationID,
		ModuleID:     moduleID,
		ExtensionID:  extensionID,
	}
}

func captureStack() string {
	return string(debug.Stack())
}

func SafeExecute(recoverFn func(kind TrapKind, msg string)) {
	if r := recover(); r != nil {
		msg := fmt.Sprintf("%v", r)
		recoverFn(ClassifyPanic(r), msg)
	}
}

func ClassifyPanic(r interface{}) TrapKind {
	msg := fmt.Sprintf("%v", r)
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "out of bounds") || strings.Contains(lower, "memory"):
		return TrapMemoryOutOfBounds
	case strings.Contains(lower, "stack overflow"):
		return TrapStackOverflow
	case strings.Contains(lower, "divide") || strings.Contains(lower, "div"):
		return TrapIntegerDivideByZero
	default:
		return TrapUnknown
	}
}

type TrapRecorder struct {
	traps []*TrapInfo
}

func NewTrapRecorder() *TrapRecorder {
	return &TrapRecorder{}
}

func (r *TrapRecorder) Record(t *TrapInfo) {
	r.traps = append(r.traps, t)
}

func (r *TrapRecorder) List() []*TrapInfo {
	return r.traps
}

func (r *TrapRecorder) Count() int {
	return len(r.traps)
}

func (r *TrapRecorder) CountByKind(kind TrapKind) int {
	count := 0
	for _, t := range r.traps {
		if t.Kind == kind {
			count++
		}
	}
	return count
}
