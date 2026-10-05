package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestRuntimeCancelBeforeExecutionRejectsLateInvocation(t *testing.T) {
	dispatcher := NewRuntimeDispatcher()
	dispatcher.RegisterCancellable("write", func(context.Context, protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		t.Fatal("cancelled invocation executed")
		return nil, nil
	})
	dispatcher.CancelInvocation("before-resolve")
	if _, err := dispatcher.Resolve("write")(protocol.RuntimeInvokePayload{InvocationID: "before-resolve"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("early cancellation lost: %v", err)
	}
	handler := dispatcher.Resolve("write")
	dispatcher.CancelInvocation("before-register")
	if _, err := handler(protocol.RuntimeInvokePayload{InvocationID: "before-register"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("registration cancellation lost: %v", err)
	}
}

func TestRuntimeDisconnectRejectsResolvedHandlersFromOldConnection(t *testing.T) {
	dispatcher := NewRuntimeDispatcher()
	calls := 0
	dispatcher.RegisterCancellable("write", func(context.Context, protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "success"}, nil
	})
	old := dispatcher.Resolve("write")
	dispatcher.CancelAllInvocations("disconnected")
	if _, err := old(protocol.RuntimeInvokePayload{InvocationID: "old"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("old resolved handler survived disconnect: %v", err)
	}
	if _, err := dispatcher.Resolve("write")(protocol.RuntimeInvokePayload{InvocationID: "new"}); err != nil || calls != 1 {
		t.Fatalf("new connection failed: calls=%d err=%v", calls, err)
	}
}
