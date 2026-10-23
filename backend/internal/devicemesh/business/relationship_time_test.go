package business

import (
	"context"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"testing"
)

type relationshipLifecyclePort struct {
	testDataPort
	events []coordination.OwnedChatEffectsRequest
}

func (p *relationshipLifecyclePort) CommitOwnedChatEffects(_ context.Context, _ coordination.ExecutionScope, input coordination.OwnedChatEffectsRequest) error {
	p.events = append(p.events, input)
	return nil
}

func TestOwnedRelationshipLifecycleReleasesFailedAndFinishesSavedReply(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			engine, _, _, model := engineHarness(t)
			port := &relationshipLifecyclePort{testDataPort: engine.data.(testDataPort)}
			engine.data = port
			if fail {
				model.generate = func(context.Context, Inference) (Generation, error) {
					return Generation{}, errors.New("model unavailable")
				}
			}
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "request", Message: "hello"})
			if (err != nil) != fail {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(port.events) == 0 || !port.events[len(port.events)-1].Abort {
				t.Fatal("failed request cleanup missing")
			}
			completed := 0
			for _, event := range port.events {
				if !event.Abort {
					completed++
				}
			}
			if fail && completed != 0 || !fail && completed != 1 {
				t.Fatalf("completion count %d", completed)
			}
		})
	}
}
