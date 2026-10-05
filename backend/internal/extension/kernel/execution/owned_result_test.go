package execution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestOwnedToolResultCacheContainsOnlyLocationAfterOwnerAcknowledgement(t *testing.T) {
	storage := NewExecutionIdempotencyStorage(newTestIdempotencyDB(t))
	guard := NewIdempotencyGuard(storage)
	t.Cleanup(guard.stopCleanup)
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{ResourceOwnerID: "device"})
	acknowledged := false
	ctx = WithOwnedToolResultStore(ctx, func(_ context.Context, result capability.UnifiedToolResult) (OwnedToolResultReference, error) {
		if string(result.Structured) != `{"deviceSecret":"private text"}` {
			t.Fatal("owner did not receive complete result")
		}
		acknowledged = true
		return OwnedToolResultReference{OwnedToolResult: true, OwnerID: "device", ResourceID: "action", SHA256: strings.Repeat("a", 64)}, nil
	})
	reservation, _, err := guard.Begin(ctx, IdempotencyIdentity{ToolID: "tool", CallerKey: "action"}, "fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	result := capability.NewToolSuccessResult("invoke", "tool")
	result.Structured = json.RawMessage(`{"deviceSecret":"private text"}`)
	result.Content = []capability.ToolContent{{Type: capability.ToolContentText, Text: "private text"}}
	if err := guard.Complete(ctx, reservation, &result); err != nil {
		t.Fatal(err)
	}
	record, err := storage.Find(t.Context(), reservation.IdempotencyKey)
	if err != nil || !acknowledged || record.State != IdempotencyStateDone {
		t.Fatalf("owner acknowledgement missing: %+v %v", record, err)
	}
	if strings.Contains(string(record.WorkResultJSON), "private text") || strings.Contains(string(record.WorkResultJSON), "deviceSecret") || !strings.Contains(string(record.WorkResultJSON), "ownedToolResult") {
		t.Fatalf("Core cached device body: %s", record.WorkResultJSON)
	}
}

func TestOwnedToolResultWithoutOwnerAckCannotBecomeCompleted(t *testing.T) {
	storage := NewExecutionIdempotencyStorage(newTestIdempotencyDB(t))
	guard := NewIdempotencyGuard(storage)
	t.Cleanup(guard.stopCleanup)
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{ResourceOwnerID: "device"})
	ctx = WithOwnedToolResultStore(ctx, func(context.Context, capability.UnifiedToolResult) (OwnedToolResultReference, error) {
		return OwnedToolResultReference{}, errors.New("owner offline")
	})
	identity := IdempotencyIdentity{ToolID: "tool", CallerKey: "action"}
	reservation, _, err := guard.Begin(ctx, identity, "fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	result := capability.NewToolSuccessResult("invoke", "tool")
	if err := guard.Complete(ctx, reservation, &result); err == nil {
		t.Fatal("missing owner acknowledgement reported success")
	}
	record, err := storage.Find(t.Context(), reservation.IdempotencyKey)
	if err != nil || record.State != IdempotencyStateIndeterminate || len(record.WorkResultJSON) != 0 {
		t.Fatalf("unknown result persisted as completed: %+v %v", record, err)
	}
	if _, _, err := guard.Begin(ctx, identity, "fingerprint"); !errors.Is(err, ErrIdempotencyIndeterminate) {
		t.Fatalf("unknown action replayed: %v", err)
	}
}
