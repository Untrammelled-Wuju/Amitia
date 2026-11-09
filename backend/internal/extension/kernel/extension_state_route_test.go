package kernel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type quotaRejectingStateStore struct{}

func (quotaRejectingStateStore) Get(context.Context, string, string, string) (json.RawMessage, int64, bool, error) {
	return nil, 0, false, nil
}

func (quotaRejectingStateStore) Set(context.Context, string, string, string, json.RawMessage) (int64, error) {
	return 0, sqlite.ErrExtensionStateQuotaExceeded
}

func (quotaRejectingStateStore) CAS(context.Context, string, string, string, int64, json.RawMessage) (bool, int64, error) {
	return false, 0, sqlite.ErrExtensionStateQuotaExceeded
}

func (quotaRejectingStateStore) Delete(context.Context, string, string, string) error {
	return nil
}

func (quotaRejectingStateStore) List(context.Context, string, string) ([]sqlite.ExtensionStateEntry, error) {
	return nil, nil
}

func TestStateCASReturnsQuotaExceeded(t *testing.T) {
	gateway := makeHealthTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{StateStore: quotaRejectingStateStore{}}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "state-quota",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "session-1",
			ExtensionID: "com.example/quota",
			ModuleID:    "main",
			Generation:  1,
		},
		Method:   host_api.MethodStateCAS,
		Version:  1,
		Input:    json.RawMessage(`{"key":"state","expectedVersion":0,"newValue":{"value":1}}`),
		Deadline: time.Now().Add(time.Second),
	})
	if result.Status != host_api.StatusRejected || result.Error == nil || result.Error.Code != host_api.ErrorCodeQuotaExceeded {
		t.Fatalf("unexpected quota response: %+v", result)
	}
}
