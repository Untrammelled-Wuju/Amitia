package task_runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRemoteTaskCancelAcknowledgementRequiresBoundWorkerCompletion(t *testing.T) {
	for _, scenario := range []string{"missing", "cancel", "disconnect", "deadline", "unbound_completion", "wrong_completion", "confirmed"} {
		t.Run(scenario, func(t *testing.T) {
			manager := NewPendingTaskManager()
			request := TaskExecutionRequest{Run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task"}, AttemptID: "attempt"}
			if scenario == "missing" {
				if manager.WaitForCancelAck(t.Context(), "run") {
					t.Fatal("absent pending task treated as worker acknowledgement")
				}
				return
			}
			deadline := time.Minute
			if scenario == "deadline" {
				deadline = 5 * time.Millisecond
			}
			pending, err := manager.Register(request, "session", 7, deadline)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Cancel("run", "cleanup")
			switch scenario {
			case "cancel":
				manager.Cancel("run", "cancel requested")
			case "disconnect":
				manager.CancelAll("session", "disconnected")
			case "unbound_completion":
				manager.Complete("run", false, "stopped")
			case "wrong_completion":
				if manager.CompleteBound("run", "old-attempt", pending.LeaseID, "session", 7, false, "stopped") {
					t.Fatal("replaced worker completion accepted")
				}
			case "confirmed":
				if !manager.CompleteBound("run", "attempt", pending.LeaseID, "session", 7, false, "stopped") {
					t.Fatal("bound worker completion rejected")
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
			defer cancel()
			if acknowledged := manager.WaitForCancelAck(ctx, "run"); acknowledged != (scenario == "confirmed") {
				t.Fatalf("incorrect worker acknowledgement: %v", acknowledged)
			}
			if scenario == "confirmed" {
				if _, err := manager.Register(request, "session", 8, time.Minute); err != nil {
					t.Fatal(err)
				}
				nextCtx, nextCancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
				defer nextCancel()
				if manager.WaitForCancelAck(nextCtx, "run") {
					t.Fatal("new execution borrowed earlier acknowledgement")
				}
			}
		})
	}
}

func TestRemoteTaskAuthorityConfirmationFailureDoesNotAcknowledgeCancellation(t *testing.T) {
	manager := NewPendingTaskManager()
	request := TaskExecutionRequest{Run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task"}, AttemptID: "attempt"}
	pending, err := manager.Register(request, "session", 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Cancel("run", "cleanup")
	fail := true
	if err := manager.BindAuthority("run", "attempt", "session", 7, func(context.Context) error {
		if fail {
			return errors.New("authority acknowledgement unavailable")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if manager.CompleteBound("run", "attempt", pending.LeaseID, "session", 7, false, "stopped") {
		t.Fatal("unconfirmed authority closure acknowledged")
	}
	if _, ok := manager.Get("run"); !ok {
		t.Fatal("unconfirmed task discarded")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()
	if manager.WaitForCancelAck(ctx, "run") {
		t.Fatal("unconfirmed authority closure became cancellation ACK")
	}
	fail = false
	if !manager.CompleteBound("run", "attempt", pending.LeaseID, "session", 7, false, "stopped") || !manager.WaitForCancelAck(t.Context(), "run") {
		t.Fatal("confirmed authority closure could not complete")
	}
}
