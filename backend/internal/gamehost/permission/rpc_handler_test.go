package permission

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type permissionRPCFakeBroker struct {
	decision kernelpermission.PermissionDecision
	calls    int
	last     kernelpermission.PermissionEvaluationRequest
}

func (b *permissionRPCFakeBroker) Evaluate(_ context.Context, request kernelpermission.PermissionEvaluationRequest) kernelpermission.PermissionEvaluationResult {
	b.calls++
	b.last = request
	decision := b.decision
	if decision == "" {
		decision = kernelpermission.DecisionAllow
	}
	result := kernelpermission.PermissionEvaluationResult{Decision: decision}
	if decision == kernelpermission.DecisionDeny {
		result.Reasons = []kernelpermission.PermissionReason{{Code: "missing_grant"}}
	}
	return result
}

type permissionRPCFakeResolver struct{}

func (permissionRPCFakeResolver) ResolveExtensionID(pluginID string) (string, bool) {
	if pluginID != "plugin-1" {
		return "", false
	}
	return "ext-1", true
}
func (permissionRPCFakeResolver) RuntimeExists(runtimeID string) (string, domain.RuntimeState, error) {
	if runtimeID != "runtime-1" {
		return "", "", fmt.Errorf("runtime not found")
	}
	return "plugin-1", domain.RuntimeStateRunning, nil
}
func (permissionRPCFakeResolver) ServiceExists(runtimeID, serviceID string) (string, string, error) {
	if runtimeID != "runtime-1" || serviceID != "service-1" {
		return "", "", fmt.Errorf("service not found")
	}
	return "plugin-1", "module-1", nil
}
func (permissionRPCFakeResolver) GetRuntimeState(runtimeID string) (domain.RuntimeState, error) {
	if runtimeID != "runtime-1" {
		return "", fmt.Errorf("runtime not found")
	}
	return domain.RuntimeStateRunning, nil
}

type permissionRPCPluginResolver struct {
	descriptor domain.PluginDescriptor
}

func (r permissionRPCPluginResolver) Get(_ context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error) {
	if pluginID != r.descriptor.ID {
		return domain.PluginDescriptor{}, fmt.Errorf("plugin not found")
	}
	return r.descriptor.Clone(), nil
}

func newPermissionRPCTestHandler(t *testing.T, broker *permissionRPCFakeBroker) *PermissionRPCHandler {
	t.Helper()
	mapper := NewGameHostSubjectMapper(permissionRPCFakeResolver{})
	adapter := NewEffectivePermissionAdapter(broker, nil, mapper)
	descriptor := domain.PluginDescriptor{
		ID:                  "plugin-1",
		ExtensionID:         "ext-1",
		Name:                "Plugin",
		Version:             "1.0.0",
		ProtocolVersion:     "amitia-game-host/1",
		RequiredPermissions: []string{kernelpermission.PermissionGameHostControl},
		Services: []domain.ServiceDescriptor{{
			ID:       "service-1",
			Name:     "Service",
			Kind:     domain.ServiceKindProcess,
			Required: true,
			Metadata: map[string]string{"moduleId": "module-1"},
		}},
	}
	h, err := NewPermissionRPCHandler(adapter, permissionRPCPluginResolver{descriptor: descriptor})
	if err != nil {
		t.Fatalf("NewPermissionRPCHandler: %v", err)
	}
	return h
}

func permissionRPCTestRequest(method rpc.Method, payload any) rpc.RPCRequest {
	raw, _ := json.Marshal(payload)
	return rpc.RPCRequest{
		ID:         "req-1",
		PluginID:   "plugin-1",
		RuntimeID:  "runtime-1",
		ServiceID:  "service-1",
		Generation: 7,
		Namespace:  "permission",
		Method:     method,
		Payload:    raw,
	}
}

func decodePermissionRPCPayload(t *testing.T, response rpc.RPCResponse) map[string]any {
	t.Helper()
	if response.Error != nil {
		t.Fatalf("unexpected routed error: %+v", response.Error)
	}
	var out map[string]any
	if err := json.Unmarshal(response.Payload, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestPermissionRPCCheckUsesReadOnlyServiceScopedEvaluation(t *testing.T) {
	broker := &permissionRPCFakeBroker{decision: kernelpermission.DecisionAllow}
	h := newPermissionRPCTestHandler(t, broker)
	response, err := h.Handle(context.Background(), permissionRPCTestRequest(MethodPermissionCheck, map[string]any{
		"permissionId": kernelpermission.PermissionGameHostControl,
		"serviceId":    "service-1",
	}))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := decodePermissionRPCPayload(t, response)
	if out["decision"] != RPCDecisionAllowed {
		t.Fatalf("decision=%v want %s", out["decision"], RPCDecisionAllowed)
	}
	if broker.calls != 1 {
		t.Fatalf("broker calls=%d want 1", broker.calls)
	}
	if got := broker.last.Subject.ModuleID; got != "module-1" {
		t.Fatalf("kernel module id=%q want module-1", got)
	}
	if len(broker.last.Requirements) != 1 || broker.last.Requirements[0].Scope.ID != "module-1" {
		t.Fatalf("unexpected module scope: %+v", broker.last.Requirements)
	}
	if got := broker.last.ExecutionContext.ModuleID; got != "module-1" {
		t.Fatalf("execution module id=%q want module-1", got)
	}
}

func TestPermissionRPCCheckWithoutServiceIDStillBindsTrustedService(t *testing.T) {
	broker := &permissionRPCFakeBroker{decision: kernelpermission.DecisionAllow}
	h := newPermissionRPCTestHandler(t, broker)
	response, err := h.Handle(context.Background(), permissionRPCTestRequest(MethodPermissionCheck, map[string]any{
		"permissionId": kernelpermission.PermissionGameHostControl,
	}))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := decodePermissionRPCPayload(t, response)
	if out["decision"] != RPCDecisionAllowed {
		t.Fatalf("decision=%v want %s", out["decision"], RPCDecisionAllowed)
	}
	if broker.calls != 1 {
		t.Fatalf("broker calls=%d want 1", broker.calls)
	}
	if got := broker.last.Subject.ModuleID; got != "module-1" {
		t.Fatalf("kernel module id=%q want module-1", got)
	}
	if len(broker.last.Requirements) != 1 || broker.last.Requirements[0].Scope.ID != "module-1" {
		t.Fatalf("unexpected module scope: %+v", broker.last.Requirements)
	}
}

func TestPermissionRPCRejectsUndeclaredWithoutConsultingBroker(t *testing.T) {
	broker := &permissionRPCFakeBroker{decision: kernelpermission.DecisionAllow}
	h := newPermissionRPCTestHandler(t, broker)
	response, err := h.Handle(context.Background(), permissionRPCTestRequest(MethodPermissionCheck, map[string]any{
		"permissionId": "filesystem.write",
		"serviceId":    "service-1",
	}))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := decodePermissionRPCPayload(t, response)
	if out["decision"] != RPCDecisionDenied || out["reason"] != string(ReasonNotDeclared) {
		t.Fatalf("unexpected response: %#v", out)
	}
	if broker.calls != 0 {
		t.Fatalf("undeclared permission consulted broker %d times", broker.calls)
	}
}

func TestPermissionRPCRejectsCrossServiceIdentity(t *testing.T) {
	broker := &permissionRPCFakeBroker{decision: kernelpermission.DecisionAllow}
	h := newPermissionRPCTestHandler(t, broker)
	response, err := h.Handle(context.Background(), permissionRPCTestRequest(MethodPermissionCheck, map[string]any{
		"permissionId": kernelpermission.PermissionGameHostControl,
		"serviceId":    "other-service",
	}))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if response.Error == nil || response.Error.Code != "IDENTITY_MISMATCH" {
		t.Fatalf("expected identity mismatch, got %+v", response.Error)
	}
	if broker.calls != 0 {
		t.Fatalf("identity mismatch consulted broker %d times", broker.calls)
	}
}

func TestPermissionRPCSnapshotContainsOnlyEffectiveDeclaredPermissions(t *testing.T) {
	broker := &permissionRPCFakeBroker{decision: kernelpermission.DecisionAllow}
	h := newPermissionRPCTestHandler(t, broker)
	response, err := h.Handle(context.Background(), permissionRPCTestRequest(MethodPermissionSnapshot, map[string]any{
		"serviceId": "service-1",
	}))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := decodePermissionRPCPayload(t, response)
	if out["isValid"] != true {
		t.Fatalf("snapshot invalid: %#v", out)
	}
	if snapshotID, _ := out["snapshotId"].(string); snapshotID == "" {
		t.Fatalf("missing snapshot id: %#v", out)
	}
	perms, ok := out["grantedPerms"].([]any)
	if !ok || len(perms) != 1 || perms[0] != kernelpermission.PermissionGameHostControl {
		t.Fatalf("unexpected grantedPerms: %#v", out["grantedPerms"])
	}
	scopes, ok := out["grantedScopes"].([]any)
	if !ok || len(scopes) != 1 || scopes[0] != "module:module-1" {
		t.Fatalf("unexpected grantedScopes: %#v", out["grantedScopes"])
	}
}
