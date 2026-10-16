package agent

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestRemoteBrowserReadDoesNotReuseStaleSnapshot(t *testing.T) {
	dispatcher := NewRuntimeDispatcher()
	calls := 0
	dispatcher.RegisterCancellable(RemoteBrowserReadHandler, func(_ context.Context, invocation protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{InvocationID: invocation.InvocationID, Status: "success"}, nil
	})
	handler := dispatcher.ResolveContext(RemoteBrowserReadHandler)
	invoke := protocol.RuntimeInvokePayload{Handler: RemoteBrowserReadHandler, InvocationID: "one", IdempotencyKey: "same-read-key", RuntimeType: "browser"}
	if _, err := handler(context.Background(), invoke); err != nil {
		t.Fatal(err)
	}
	if _, err := handler(context.Background(), invoke); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("browser snapshot cached for 24 hours: calls=%d", calls)
	}
}
