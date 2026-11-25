package execution_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

func TestInvocationChainSpansHostAndProvider(t *testing.T) {
	route := host_api.Route{Method: host_api.MethodToolExecute, Version: 1, SideEffectLevel: host_api.SideEffectWrite}
	call := host_api.CallRequest{
		CallID:          "call-1",
		Method:          route.Method,
		Version:         1,
		RootExecutionID: "root-1",
		TraceID:         "trace-1",
		InvocationID:    "inv-1",
		ParentID:        "inv-0",
		IdempotencyKey:  "idem-1",
	}
	hostContract, err := route.InvocationContract(call)
	if err != nil {
		t.Fatalf("host contract: %v", err)
	}
	if hostContract.SideEffect != execution.SideEffectClassWrite || !hostContract.SideEffect.Mutating() {
		t.Fatalf("host side effect = %s want write", hostContract.SideEffect)
	}
	if hostContract.CapabilityID != "host.tool.execute" {
		t.Fatalf("host capability = %s", hostContract.CapabilityID)
	}

	providerRequest := capability.ProviderInvocationRequest{
		CapabilityID:   capability.CapabilityID(string(call.Method)),
		Trace:          call.InvocationTrace(),
		SideEffect:     route.SideEffectLevel.Class(),
		IdempotencyKey: call.IdempotencyKey,
		Deadline:       call.Deadline,
	}
	providerContract, err := providerRequest.InvocationContract()
	if err != nil {
		t.Fatalf("provider contract: %v", err)
	}
	if providerContract.Trace != hostContract.Trace {
		t.Fatalf("trace changed across the hop: host=%+v provider=%+v", hostContract.Trace, providerContract.Trace)
	}
	if providerContract.Trace.InvocationID != "inv-1" || providerContract.Trace.ParentInvocationID != "inv-0" {
		t.Fatalf("provider trace = %+v", providerContract.Trace)
	}
	if providerContract.IdempotencyKey != hostContract.IdempotencyKey {
		t.Fatalf("idempotency key changed across the hop: %s vs %s", hostContract.IdempotencyKey, providerContract.IdempotencyKey)
	}

	child, err := providerContract.Trace.Child("inv-2")
	if err != nil {
		t.Fatalf("child trace: %v", err)
	}
	childContract, err := execution.NewInvocationContract(child, "memory.write", execution.SideEffectClassWrite, "idem-2")
	if err != nil {
		t.Fatalf("child contract: %v", err)
	}

	outcome, classifyErr := execution.ClassifyInvocationOutcome(childContract, true, fmt.Errorf("bridge: %w", execution.ErrInvocationTransportLost))
	if outcome != execution.InvocationOutcomeUnknown {
		t.Fatalf("child outcome = %s want unknown", outcome)
	}
	var unknown *execution.UnknownInvocationOutcomeError
	if !errors.As(classifyErr, &unknown) || unknown.Trace.ParentInvocationID != "inv-1" {
		t.Fatalf("unknown error lost the parent link: %v", classifyErr)
	}

	journal := execution.NewInMemoryJournal()
	journal.Record(hostContract.JournalEntry(execution.JournalEntryExecutionStarted, "host call"))
	journal.Record(providerContract.JournalEntry(execution.JournalEntryToolInvoke, "provider invoke"))
	journal.Record(childContract.JournalEntry(execution.JournalEntryExecutionFailed, "unknown outcome"))
	entries := journal.EntriesForRoot("root-1")
	if len(entries) != 3 {
		t.Fatalf("entries = %d want 3", len(entries))
	}
	for _, entry := range entries[:2] {
		if entry.TraceID != "trace-1" || entry.InvocationID != "inv-1" {
			t.Fatalf("entry lost the trace: %+v", entry)
		}
	}
	if entries[2].InvocationID != "inv-2" || entries[2].TraceID != "trace-1" || entries[2].RootExecutionID != "root-1" {
		t.Fatalf("child entry = %+v", entries[2])
	}

	dangling := host_api.CallRequest{CallID: "call-2", Method: host_api.MethodToolExecute, TraceID: "trace-9", InvocationID: "inv-9", IdempotencyKey: "idem-9"}
	if _, err := route.InvocationContract(dangling); !errors.Is(err, execution.ErrInvocationContractInvalid) {
		t.Fatalf("missing root error = %v want ErrInvocationContractInvalid", err)
	}

	fmt.Printf(
		"invocation chain root=%s trace=%s invocation=%s parent=%s child=%s hops=%s>%s entries=%d sideEffect=%s outcome=%s\n",
		entries[0].RootExecutionID, entries[0].TraceID, entries[0].InvocationID, "inv-0", entries[2].InvocationID,
		"host_api", "capability", len(entries), providerContract.SideEffect, outcome,
	)
}
