package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestOwnedDataAlwaysRevalidatesAuthorityInsteadOfReturningCachedReply(t *testing.T) {
	dispatcher := NewRuntimeDispatcher()
	closed := false
	calls := 0
	expired := errors.New("authority expired")
	dispatcher.RegisterCancellable("coordination.data", func(context.Context, protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		calls++
		if closed {
			return nil, expired
		}
		return &protocol.RuntimeResultPayload{Result: []byte(`{"ownerId":"device"}`)}, nil
	})
	handler := dispatcher.Resolve("coordination.data")
	invocation := protocol.RuntimeInvokePayload{InvocationID: "same-id", DeviceID: "device", SpaceID: "core", Handler: "coordination.data", Input: []byte(`{"operation":"snapshot"}`)}
	if _, err := handler(invocation); err != nil {
		t.Fatal(err)
	}
	closed = true
	if _, err := handler(invocation); !errors.Is(err, expired) {
		t.Fatalf("cached data bypassed authority: %v", err)
	}
	if calls != 2 {
		t.Fatal("authority handler was skipped")
	}
}
