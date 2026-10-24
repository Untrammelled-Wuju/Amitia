package business

import (
	"errors"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedContinuityExecutionBindingUsesOriginalOwnerTransaction(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(fmt.Sprint(coordinated), func(t *testing.T) {
			engine, _, service, _ := engineHarness(t)
			engine.data = continuitySignalPort{engine.data.(testDataPort)}
			if coordinated {
				if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
					t.Fatal(err)
				}
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role"}
			documents := []OwnedContinuity{}
			for index := 0; index < 3; index++ {
				request.RequestID = fmt.Sprint("create-goal-", index)
				document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: fmt.Sprint("goal-", index)})
				if err != nil {
					t.Fatal(err)
				}
				documents = append(documents, document)
			}
			bind := func(index int, role, requestID string) error {
				request.RequestID = requestID
				binding := continuity.NewThreadBinding(documents[index].Thread.ID, "task_run", "shared-task", role, "task_lifecycle", 1)
				document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: documents[index].Thread.ID, ExpectedRevision: documents[index].Thread.Revision, Action: "bind", Binding: &binding})
				if err == nil {
					documents[index] = document
				}
				return err
			}
			if err := bind(0, "execution", "execution-goal"); err != nil {
				t.Fatal(err)
			}
			if err := bind(1, "execution", "conflicting-goal"); !errors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("execution conflict not enforced at owner: %v", err)
			}
			for _, index := range []int{1, 2} {
				if err := bind(index, "dependency", fmt.Sprint("dependency-", index)); err != nil {
					t.Fatalf("shared dependency rejected: %v", err)
				}
			}
			request.RequestID = "list-goals"
			listed, _, err := engine.ListContinuity(t.Context(), request, false)
			if err != nil || len(listed) != 3 {
				t.Fatalf("owner readback failed: %d %v", len(listed), err)
			}
			executions, references := 0, 0
			for _, document := range listed {
				for _, binding := range document.Bindings {
					if binding.Role == "execution" {
						executions++
					} else if binding.Role == "dependency" {
						references++
					}
				}
			}
			if executions != 1 || references != 2 {
				t.Fatalf("unexpected durable bindings: execution=%d dependency=%d", executions, references)
			}
		})
	}
}
