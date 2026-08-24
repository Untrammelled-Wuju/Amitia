package control

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type alwaysNegotiatedFeatureChecker struct{}

func (alwaysNegotiatedFeatureChecker) HasNegotiatedCapability(string, domain.Capability) bool {
	return true
}

func TestAuthorityRPCWireContractUsesCanonicalModesAndOptionalEpoch(t *testing.T) {
	ctx := context.Background()
	takeover, manager, runtimeReader, _ := setupTakeoverService(t)
	runtimeID := domain.RuntimeInstanceID("runtime-rpc-1")
	pluginID := domain.PluginID("plugin-rpc-1")
	if _, err := manager.Create(ctx, runtimeID, pluginID); err != nil {
		t.Fatalf("create authority: %v", err)
	}
	runtimeReader.SetActive(runtimeID, true)
	runtimeReader.SetReady(runtimeID, true)

	parent := NewAuthorityRPCHandler(manager, takeover, nil)
	parent.SetNegotiatedFeatureChecker(alwaysNegotiatedFeatureChecker{})
	base := rpc.RPCRequest{
		ID:           "request-1",
		PluginID:     pluginID,
		RuntimeID:    runtimeID,
		ServiceID:    "service-1",
		Generation:   1,
		ConnectionID: "connection-1",
		Payload:      json.RawMessage(`{}`),
	}

	snapshotResponse, err := (authoritySnapshotRPCHandler{parent}).Handle(ctx, base)
	if err != nil || snapshotResponse.Error != nil {
		t.Fatalf("snapshot err=%v response=%+v", err, snapshotResponse.Error)
	}
	var snapshot authoritySnapshotRPCResult
	if err := json.Unmarshal(snapshotResponse.Payload, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Mode != string(domain.ControlModeObserveOnly) || snapshot.Epoch == 0 || snapshot.UpdatedAt.IsZero() {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	base.ID = "request-2"
	takeoverResponse, err := (authorityTakeoverRPCHandler{parent}).Handle(ctx, base)
	if err != nil || takeoverResponse.Error != nil {
		t.Fatalf("takeover err=%v response=%+v", err, takeoverResponse.Error)
	}
	var takeoverWire authorityMutationRPCResult
	if err := json.Unmarshal(takeoverResponse.Payload, &takeoverWire); err != nil {
		t.Fatalf("decode takeover: %v", err)
	}
	if takeoverWire.NewMode != string(domain.ControlModeUserControl) || takeoverWire.NewEpoch <= takeoverWire.PreviousEpoch {
		t.Fatalf("unexpected takeover result: %+v", takeoverWire)
	}

	// expectedEpoch and targetMode are optional in the public SDK. Omitting both
	// must release to the service default instead of silently treating epoch=0
	// as an optimistic-concurrency assertion.
	base.ID = "request-3"
	releaseResponse, err := (authorityReleaseRPCHandler{parent}).Handle(ctx, base)
	if err != nil || releaseResponse.Error != nil {
		t.Fatalf("release err=%v response=%+v", err, releaseResponse.Error)
	}
	var releaseWire authorityMutationRPCResult
	if err := json.Unmarshal(releaseResponse.Payload, &releaseWire); err != nil {
		t.Fatalf("decode release: %v", err)
	}
	if releaseWire.NewMode != string(DefaultReleaseTarget) || releaseWire.NewEpoch <= releaseWire.PreviousEpoch {
		t.Fatalf("unexpected release result: %+v", releaseWire)
	}
}

func TestAuthorityRPCPreservesAuthorityErrorCode(t *testing.T) {
	response := authorityRPCResponse("request-1", nil, &AuthorityError{Code: domain.ErrPermissionDenied, Message: "denied"})
	if response.Error == nil {
		t.Fatal("expected routed error")
	}
	if response.Error.Code != string(domain.ErrPermissionDenied) {
		t.Fatalf("code=%q want=%q", response.Error.Code, domain.ErrPermissionDenied)
	}
}
