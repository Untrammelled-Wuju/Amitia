package agent

import (
	"context"
	"testing"
	"time"
)

func TestTaskLeaseStopsExpiredExecutionAndNeverResurrectsIt(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lease := &taskLeaseTimer{cancel: cancel}
	defer lease.Close()
	if err := lease.Confirm(20 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expired task lease did not stop the device execution")
	}
	if err := lease.Confirm(time.Minute); err == nil {
		t.Fatal("expired task execution was resurrected by a late lease ACK")
	}
}

func TestTaskLeaseRenewalAndCloseDoNotCancelFinishedOrRenewedExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lease := &taskLeaseTimer{cancel: cancel}
	if err := lease.Confirm(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := lease.Confirm(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	lease.Close()
	if err := lease.Confirm(time.Minute); err == nil {
		t.Fatal("closed task lease accepted another ACK")
	}
	select {
	case <-ctx.Done():
		t.Fatal("closing a finished lease falsely cancelled the task")
	default:
	}
}

func TestMissingDeviceTaskCannotEmitCancellationAcknowledgement(t *testing.T) {
	worker := NewTaskWorker(nil)
	if err := worker.CancelTask(t.Context(), "missing", "attempt", "lease"); err == nil {
		t.Fatal("missing execution claimed a cancellation acknowledgement")
	}
}
