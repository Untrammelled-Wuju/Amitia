package business

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type completionFailurePort struct{ coordination.DataPort }

func (p completionFailurePort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	for _, mutation := range commit.Mutations {
		if mutation.Kind == "message" && strings.HasSuffix(mutation.ID, "/assistant") {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
	}
	return p.DataPort.Commit(ctx, commit)
}

func TestOwnedCompletionRunsAfterOwnerCommitAndPreservesFailure(t *testing.T) {
	for _, scenario := range []string{"saved", "commit-failed", "model-failed"} {
		t.Run(scenario, func(t *testing.T) {
			engine, db, _, model := engineHarness(t)
			if scenario == "commit-failed" {
				engine.data = completionFailurePort{engine.data}
			}
			called := 0
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				if inference.RegisterCompletion == nil {
					t.Fatal("engine omitted turn completion")
				}
				inference.RegisterCompletion(func(ctx context.Context, generation Generation, cause error) error {
					called++
					row, err := coordination.NewOwnershipStore(db, inference.Scope.ResourceOwnerID).Get(ctx, "message", "completion/assistant")
					if err != nil {
						t.Fatal(err)
					}
					if scenario == "saved" {
						if cause != nil || row == nil || generation.Text != "confirmed" {
							t.Fatalf("completion preceded save: %+v %v", generation, cause)
						}
					} else if cause == nil || row != nil {
						t.Fatalf("failed reply appeared saved: row=%+v cause=%v", row, cause)
					}
					return nil
				})
				if scenario == "model-failed" {
					return Generation{}, errors.New("controlled model failure")
				}
				return Generation{Text: "confirmed"}, nil
			}
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "completion", Message: "check commit"})
			if called != 1 || (err == nil) != (scenario == "saved") {
				t.Fatalf("completion=%d err=%v", called, err)
			}
		})
	}
}
