package execution

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func testTrace() InvocationTrace {
	return InvocationTrace{RootExecutionID: "root-1", TraceID: "trace-1", InvocationID: "inv-1"}
}

func TestInvocationTraceRequiresIdentity(t *testing.T) {
	cases := []struct {
		name  string
		trace InvocationTrace
		field string
	}{
		{name: "root", trace: InvocationTrace{TraceID: "trace-1", InvocationID: "inv-1"}, field: "trace.rootExecutionId"},
		{name: "trace", trace: InvocationTrace{RootExecutionID: "root-1", InvocationID: "inv-1"}, field: "trace.traceId"},
		{name: "invocation", trace: InvocationTrace{RootExecutionID: "root-1", TraceID: "trace-1"}, field: "trace.invocationId"},
	}
	for _, tc := range cases {
		err := tc.trace.Validate()
		if err == nil {
			t.Fatalf("%s: empty identity must fail", tc.name)
		}
		var invalid *InvocationContractInvalidError
		if !errors.As(err, &invalid) {
			t.Fatalf("%s: error = %v want *InvocationContractInvalidError", tc.name, err)
		}
		if invalid.Field != tc.field {
			t.Fatalf("%s: field = %s want %s", tc.name, invalid.Field, tc.field)
		}
		if !errors.Is(err, ErrInvocationContractInvalid) {
			t.Fatalf("%s: error lost the sentinel: %v", tc.name, err)
		}
	}
	if err := testTrace().Validate(); err != nil {
		t.Fatalf("valid trace rejected: %v", err)
	}
	fmt.Printf("invocation trace guards fields=root,trace,invocation error=ErrInvocationContractInvalid\n")
}

func TestInvocationTraceChildKeepsRoot(t *testing.T) {
	parent := testTrace()
	child, err := parent.Child("inv-2")
	if err != nil {
		t.Fatalf("Child: %v", err)
	}
	if child.RootExecutionID != parent.RootExecutionID || child.TraceID != parent.TraceID {
		t.Fatalf("child = %+v lost root or trace", child)
	}
	if child.InvocationID != "inv-2" || child.ParentInvocationID != parent.InvocationID {
		t.Fatalf("child = %+v want inv-2 under inv-1", child)
	}
	if _, err := parent.Child("inv-1"); !errors.Is(err, ErrInvocationContractInvalid) {
		t.Fatalf("self child error = %v want ErrInvocationContractInvalid", err)
	}
	if _, err := parent.Child("  "); !errors.Is(err, ErrInvocationContractInvalid) {
		t.Fatalf("blank child error = %v want ErrInvocationContractInvalid", err)
	}
	fmt.Printf("invocation trace child root=%s trace=%s parent=%s child=%s\n", child.RootExecutionID, child.TraceID, child.ParentInvocationID, child.InvocationID)
}

func TestInvocationContractRequiresIdempotencyForMutating(t *testing.T) {
	if _, err := NewInvocationContract(testTrace(), "memory.search", SideEffectClassWrite, ""); err == nil {
		t.Fatalf("write without an idempotency key must fail")
	} else {
		var invalid *InvocationContractInvalidError
		if !errors.As(err, &invalid) || invalid.Field != "idempotencyKey" {
			t.Fatalf("write error = %v want idempotencyKey", err)
		}
	}
	if _, err := NewInvocationContract(testTrace(), "memory.search", SideEffectClassExternal, " "); !errors.Is(err, ErrInvocationContractInvalid) {
		t.Fatalf("external without a key must fail with the sentinel")
	}
	read, err := NewInvocationContract(testTrace(), "memory.search", SideEffectClassRead, "")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if read.SideEffect.Mutating() {
		t.Fatalf("read side effect must not be mutating")
	}
	if _, err := NewInvocationContract(testTrace(), "", SideEffectClassRead, ""); !errors.Is(err, ErrInvocationContractInvalid) {
		t.Fatalf("empty capability must fail with the sentinel")
	}
	if _, err := NewInvocationContract(testTrace(), "memory.search", SideEffectClass("hybrid"), ""); !errors.Is(err, ErrInvocationContractInvalid) {
		t.Fatalf("unknown side effect must fail with the sentinel")
	}
	fmt.Printf("invocation contract mutating=write,external requiresKey=true readMutating=%v\n", read.SideEffect.Mutating())
}

func TestInvocationOutcomeSuccessAndPreDispatch(t *testing.T) {
	read, err := NewInvocationContract(testTrace(), "memory.search", SideEffectClassRead, "")
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	outcome, classifyErr := ClassifyInvocationOutcome(read, true, nil)
	if outcome != InvocationOutcomeSuccess || classifyErr != nil {
		t.Fatalf("outcome = %s err = %v want success", outcome, classifyErr)
	}
	write, err := NewInvocationContract(testTrace(), "memory.write", SideEffectClassWrite, "key-1")
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	outcome, classifyErr = ClassifyInvocationOutcome(write, false, context.DeadlineExceeded)
	if outcome != InvocationOutcomeFailed {
		t.Fatalf("pre-dispatch outcome = %s want failed", outcome)
	}
	if !errors.Is(classifyErr, context.DeadlineExceeded) {
		t.Fatalf("pre-dispatch error lost the cause: %v", classifyErr)
	}
	if errors.Is(classifyErr, ErrInvocationOutcomeUnknown) {
		t.Fatalf("pre-dispatch timeout must never be unknown")
	}
	fmt.Printf("invocation outcome success=success preDispatch=write+timeout result=failed unknown=%v\n", errors.Is(classifyErr, ErrInvocationOutcomeUnknown))
}

func TestInvocationOutcomePostDispatchWriteIsUnknown(t *testing.T) {
	write, err := NewInvocationContract(testTrace(), "memory.write", SideEffectClassWrite, "key-1")
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	outcome, classifyErr := ClassifyInvocationOutcome(write, true, fmt.Errorf("transport: %w", context.DeadlineExceeded))
	if outcome != InvocationOutcomeUnknown {
		t.Fatalf("outcome = %s want unknown", outcome)
	}
	var unknown *UnknownInvocationOutcomeError
	if !errors.As(classifyErr, &unknown) {
		t.Fatalf("error = %v want *UnknownInvocationOutcomeError", classifyErr)
	}
	if !errors.Is(classifyErr, ErrInvocationOutcomeUnknown) {
		t.Fatalf("error lost the unknown sentinel: %v", classifyErr)
	}
	if !errors.Is(classifyErr, context.DeadlineExceeded) {
		t.Fatalf("unknown outcome must preserve the original cause: %v", classifyErr)
	}
	if unknown.RetrySafe() {
		t.Fatalf("an unknown write outcome must not be retry safe")
	}
	if unknown.SideEffect != SideEffectClassWrite || unknown.Trace.InvocationID != "inv-1" {
		t.Fatalf("unknown error = %+v", unknown)
	}
	fmt.Printf("invocation outcome write postDispatch=timeout outcome=unknown retrySafe=%v cause=%v\n", unknown.RetrySafe(), errors.Is(classifyErr, context.DeadlineExceeded))
}

func TestInvocationOutcomePostDispatchReadIsFailed(t *testing.T) {
	read, err := NewInvocationContract(testTrace(), "memory.search", SideEffectClassRead, "")
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	outcome, classifyErr := ClassifyInvocationOutcome(read, true, context.DeadlineExceeded)
	if outcome != InvocationOutcomeFailed {
		t.Fatalf("read outcome = %s want failed", outcome)
	}
	if errors.Is(classifyErr, ErrInvocationOutcomeUnknown) {
		t.Fatalf("a read timeout must stay failed")
	}
	if !IsAmbiguousInvocationFailure(context.DeadlineExceeded) || !IsAmbiguousInvocationFailure(ErrInvocationTransportLost) {
		t.Fatalf("deadline and transport loss must be ambiguous")
	}
	if IsAmbiguousInvocationFailure(nil) || IsAmbiguousInvocationFailure(errors.New("permission denied")) {
		t.Fatalf("plain errors must not be ambiguous")
	}
	fmt.Printf("invocation outcome read postDispatch=timeout outcome=failed ambiguous=true\n")
}

func TestInvocationContractJournalEntryCarriesTrace(t *testing.T) {
	write, err := NewInvocationContract(testTrace(), "memory.write", SideEffectClassWrite, "key-1")
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	entry := write.JournalEntry(JournalEntryExecutionStarted, "dispatch")
	if entry.RootExecutionID != "root-1" || entry.TraceID != "trace-1" || entry.InvocationID != "inv-1" {
		t.Fatalf("entry = %+v", entry)
	}
	journal := NewInMemoryJournal()
	journal.Record(entry)
	journal.Record(write.JournalEntry(JournalEntryExecutionFailed, "unknown outcome"))
	entries := journal.EntriesForRoot("root-1")
	if len(entries) != 2 {
		t.Fatalf("entries = %d want 2", len(entries))
	}
	for _, item := range entries {
		if item.InvocationID != "inv-1" || item.TraceID != "trace-1" {
			t.Fatalf("journal entry lost the trace: %+v", item)
		}
	}
	fmt.Printf("invocation journal root=%s trace=%s invocation=%s entries=%d\n", "root-1", "trace-1", "inv-1", len(entries))
}
