package secret_test

import (
	"context"
	"encoding/json"
	"testing"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/internal/gamehost/secret"
)

func newSecretRPCFixture(t *testing.T) (rpc.HandlerRegistry, *fakeBroker) {
	t.Helper()
	broker := newFakeBroker()
	broker.SeedSecret(refOpenAI, "sk-real")
	identity := newFakeIdentity()
	identity.AddRuntime("rt-1", "plugin-x", "ext-x", "created")
	identity.AddServiceModule("rt-1", "svc-1", "module-kernel", "plugin-x", "ext-x", "running")
	adapter, err := secret.NewSecretLeaseAdapter(broker, identity, &fakeGate{allow: true})
	if err != nil {
		t.Fatalf("new secret adapter: %v", err)
	}
	registry := rpc.NewHostHandlerRegistry()
	if err := secret.NewSecretRPCHandler(adapter).Register(registry); err != nil {
		t.Fatalf("register secret RPC: %v", err)
	}
	return registry, broker
}

func callSecretRPC(t *testing.T, registry rpc.HandlerRegistry, method rpc.Method, payload string) rpc.RPCResponse {
	t.Helper()
	handler, err := registry.Resolve(method)
	if err != nil {
		t.Fatalf("resolve %s: %v", method, err)
	}
	response, err := handler.Handle(context.Background(), rpc.RPCRequest{
		ID:         "request-1",
		Method:     method,
		PluginID:   domain.PluginID("plugin-x"),
		RuntimeID:  domain.RuntimeInstanceID("rt-1"),
		ServiceID:  domain.ServiceID("svc-1"),
		Generation: 1,
		Payload:    json.RawMessage(payload),
	})
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return response
}

func TestSecretRPCBindsIdentityToTrustedRequestAndUsesLeaseIDQueries(t *testing.T) {
	registry, broker := newSecretRPCFixture(t)
	acquire := callSecretRPC(t, registry, "secret.acquire", `{"ref":"secret://provider/openai","purpose":"runtime","required":true,"runtimeId":"spoofed","serviceId":"spoofed","generation":999}`)
	if acquire.Error != nil {
		t.Fatalf("acquire routed error: %+v", acquire.Error)
	}
	var acquireWire map[string]json.RawMessage
	if err := json.Unmarshal(acquire.Payload, &acquireWire); err != nil {
		t.Fatalf("decode acquire: %v", err)
	}
	if _, exists := acquireWire["status"]; exists {
		t.Fatal("secret acquire result must not expose the historical status field")
	}
	var leaseID string
	if err := json.Unmarshal(acquireWire["leaseId"], &leaseID); err != nil || leaseID == "" {
		t.Fatalf("invalid lease id: %q err=%v", leaseID, err)
	}
	lease, ok := broker.GetLease(leaseID)
	if !ok {
		t.Fatal("issued lease not found")
	}
	if lease.RuntimeInstanceID != "rt-1" || lease.ModuleID != "module-kernel" || lease.Generation != 1 {
		t.Fatalf("lease identity was taken from untrusted payload: %+v", lease)
	}

	byRef := callSecretRPC(t, registry, "secret.query", `{"ref":"secret://provider/openai"}`)
	if byRef.Error == nil || byRef.Error.Code != "INVALID_PAYLOAD" {
		t.Fatalf("secret.query must be leaseId-only, got %+v", byRef.Error)
	}

	query := callSecretRPC(t, registry, "secret.query", `{"leaseId":"`+leaseID+`"}`)
	if query.Error != nil {
		t.Fatalf("query routed error: %+v", query.Error)
	}
	var queryWire map[string]json.RawMessage
	if err := json.Unmarshal(query.Payload, &queryWire); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if _, exists := queryWire["status"]; exists {
		t.Fatal("secret query result must not expose the historical status field")
	}
}

func TestSecretRPCAcquireRejectsUnknownPurpose(t *testing.T) {
	registry, _ := newSecretRPCFixture(t)
	response := callSecretRPC(t, registry, "secret.acquire", `{"ref":"secret://provider/openai","purpose":"arbitrary","required":true}`)
	if response.Error == nil || response.Error.Code != "INVALID_PURPOSE" {
		t.Fatalf("secret.acquire must reject non-canonical purpose, got %+v", response.Error)
	}
}

func TestSecretRPCQueryOmitsLeaseMetadataWhenBackingLeaseIsMissing(t *testing.T) {
	registry, broker := newSecretRPCFixture(t)
	acquire := callSecretRPC(t, registry, "secret.acquire", `{"ref":"secret://provider/openai","purpose":"runtime","required":true}`)
	if acquire.Error != nil {
		t.Fatalf("acquire routed error: %+v", acquire.Error)
	}
	var acquired struct {
		LeaseID string `json:"leaseId"`
	}
	if err := json.Unmarshal(acquire.Payload, &acquired); err != nil || acquired.LeaseID == "" {
		t.Fatalf("decode acquired lease: id=%q err=%v", acquired.LeaseID, err)
	}

	broker.mu.Lock()
	delete(broker.leases, kernelsecret.LeaseID(acquired.LeaseID))
	broker.mu.Unlock()

	query := callSecretRPC(t, registry, "secret.query", `{"leaseId":"`+acquired.LeaseID+`"}`)
	if query.Error != nil {
		t.Fatalf("query routed error: %+v", query.Error)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(query.Payload, &wire); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if _, ok := wire["expiresAt"]; ok {
		t.Fatal("missing backing lease must not expose a synthetic expiresAt")
	}
	if _, ok := wire["ref"]; ok {
		t.Fatal("missing backing lease must not expose an empty ref")
	}
	var leaseID string
	if err := json.Unmarshal(wire["leaseId"], &leaseID); err != nil || leaseID != acquired.LeaseID {
		t.Fatalf("query must preserve requested leaseId, got %q err=%v", leaseID, err)
	}
	var valid bool
	if err := json.Unmarshal(wire["valid"], &valid); err != nil || valid {
		t.Fatalf("missing backing lease must be invalid, valid=%t err=%v", valid, err)
	}
}
