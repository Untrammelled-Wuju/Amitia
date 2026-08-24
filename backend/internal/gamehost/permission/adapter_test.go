package permission_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/domain"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
)

type fakeBroker struct {
	mu         sync.Mutex
	grants     map[string]map[string]kernelpermission.PermissionDecision
	calls      int
	evaluateFn func(request kernelpermission.PermissionEvaluationRequest) kernelpermission.PermissionEvaluationResult
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{
		grants: make(map[string]map[string]kernelpermission.PermissionDecision),
	}
}

func (b *fakeBroker) addGrant(subjectKey, permID string, decision kernelpermission.PermissionDecision) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.grants[subjectKey] == nil {
		b.grants[subjectKey] = make(map[string]kernelpermission.PermissionDecision)
	}
	b.grants[subjectKey][permID] = decision
}

func (b *fakeBroker) Evaluate(ctx context.Context, request kernelpermission.PermissionEvaluationRequest) kernelpermission.PermissionEvaluationResult {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()

	if b.evaluateFn != nil {
		return b.evaluateFn(request)
	}

	if len(request.Requirements) == 0 {
		return kernelpermission.PermissionEvaluationResult{Decision: kernelpermission.DecisionAllow}
	}

	result := kernelpermission.PermissionEvaluationResult{
		Decision:      kernelpermission.DecisionAllow,
		MatchedGrants: make([]kernelpermission.PermissionGrant, 0),
		Missing:       make([]kernelpermission.PermissionRequirement, 0),
		Reasons:       make([]kernelpermission.PermissionReason, 0),
	}

	subjectKey := string(request.Subject.Type) + ":" + request.Subject.ID
	b.mu.Lock()
	grants := b.grants[subjectKey]
	b.mu.Unlock()

	for _, req := range request.Requirements {
		if grants != nil {
			if d, ok := grants[req.PermissionID]; ok {
				if d == kernelpermission.DecisionAllow {
					result.MatchedGrants = append(result.MatchedGrants, kernelpermission.PermissionGrant{
						GrantID:      "grant-" + req.PermissionID,
						Subject:      request.Subject,
						PermissionID: req.PermissionID,
						Decision:     kernelpermission.DecisionAllow,
						IssuedAt:     time.Now(),
					})
					result.Reasons = append(result.Reasons, kernelpermission.PermissionReason{Code: "grant_matched", Permission: req.PermissionID})
					continue
				}
			}
		}
		result.Missing = append(result.Missing, req)
		result.Reasons = append(result.Reasons, kernelpermission.PermissionReason{Code: "missing_grant", Permission: req.PermissionID})
	}

	if len(result.Missing) > 0 {
		result.Decision = kernelpermission.DecisionDeny
	} else {
		result.Decision = kernelpermission.DecisionAllow
	}

	return result
}

func (b *fakeBroker) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

type fakeResolver struct {
	mu       sync.Mutex
	plugins  map[string]string
	runtimes map[string]fakeRuntimeInfo
	services map[string]fakeServiceInfo
}

type fakeRuntimeInfo struct {
	pluginID string
	state    domain.RuntimeState
}

type fakeServiceInfo struct {
	pluginID string
	moduleID string
}

func newFakeResolver() *fakeResolver {
	return &fakeResolver{
		plugins:  make(map[string]string),
		runtimes: make(map[string]fakeRuntimeInfo),
		services: make(map[string]fakeServiceInfo),
	}
}

func (r *fakeResolver) addPlugin(pluginID, extID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[pluginID] = extID
}

func (r *fakeResolver) addRuntime(runtimeID, pluginID string, state domain.RuntimeState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runtimes[runtimeID] = fakeRuntimeInfo{pluginID: pluginID, state: state}
}

func (r *fakeResolver) addService(runtimeID, serviceID, pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[runtimeID+"/"+serviceID] = fakeServiceInfo{pluginID: pluginID, moduleID: serviceID}
}

func (r *fakeResolver) ResolveExtensionID(pluginID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	extID, ok := r.plugins[pluginID]
	return extID, ok
}

func (r *fakeResolver) RuntimeExists(runtimeID string) (string, domain.RuntimeState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.runtimes[runtimeID]
	if !ok {
		return "", "", fmt.Errorf("runtime not found")
	}
	return info.pluginID, info.state, nil
}

func (r *fakeResolver) ServiceExists(runtimeID string, serviceID string) (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.services[runtimeID+"/"+serviceID]
	if !ok {
		return "", "", fmt.Errorf("service not found")
	}
	return info.pluginID, info.moduleID, nil
}

func (r *fakeResolver) GetRuntimeState(runtimeID string) (domain.RuntimeState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.runtimes[runtimeID]
	if !ok {
		return "", fmt.Errorf("runtime not found")
	}
	return info.state, nil
}

func buildTestAdapter(broker *fakeBroker, policy ghpermission.PermissionDecisionHostPolicy) (*ghpermission.EffectivePermissionAdapter, *fakeResolver, *fakeBroker) {
	resolver := newFakeResolver()
	mapper := ghpermission.NewGameHostSubjectMapper(resolver)
	var adapter *ghpermission.EffectivePermissionAdapter
	if policy != nil {
		adapter = ghpermission.NewEffectivePermissionAdapter(broker, policy, mapper)
	} else {
		adapter = ghpermission.NewEffectivePermissionAdapter(broker, nil, mapper)
	}
	return adapter, resolver, broker
}

func TestCheck_AllAllow(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)
	broker.addGrant("runtime:rt-1", "gamehost.channel.use", kernelpermission.DecisionAllow)
	broker.addGrant("runtime:rt-1", "gamehost.host_api.invoke", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if !result.Allowed() {
		t.Fatalf("expected ALLOW, got DENY reason=%s detail=%s", result.Reason, result.Detail)
	}

	view := adapter.ResolveRuntimePermissions(context.Background(), ghpermission.EffectiveSubject{RuntimeID: "rt-1", PluginID: "plugin-1", ExtensionID: "ext-1"}, "gamehost.control", "gamehost.channel.use", "gamehost.host_api.invoke")
	if len(view.AllowedPermissions()) != 3 {
		t.Fatalf("expected 3 allowed permissions, got %d", len(view.AllowedPermissions()))
	}
}

func TestCheck_NotGranted(t *testing.T) {
	broker := newFakeBroker()
	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for missing grant, got ALLOW")
	}
	if result.Reason != ghpermission.ReasonNotGranted {
		t.Fatalf("expected reason not_granted, got %s", result.Reason)
	}
}

func TestCheck_InvalidSubjectEmptyRuntimeID(t *testing.T) {
	broker := newFakeBroker()
	adapter, _, _ := buildTestAdapter(broker, nil)

	result := adapter.Check(context.Background(), ghpermission.EffectiveSubject{}, "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for empty runtime id")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_InvalidSubjectEmptyPluginID(t *testing.T) {
	broker := newFakeBroker()
	adapter, _, _ := buildTestAdapter(broker, nil)

	result := adapter.Check(context.Background(), ghpermission.EffectiveSubject{RuntimeID: "rt-1"}, "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for empty plugin id")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_InvalidSubjectEmptyExtensionID(t *testing.T) {
	broker := newFakeBroker()
	adapter, _, _ := buildTestAdapter(broker, nil)

	result := adapter.Check(context.Background(), ghpermission.EffectiveSubject{RuntimeID: "rt-1", PluginID: "plugin-1"}, "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for empty extension id")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_EmptyPermissionID(t *testing.T) {
	broker := newFakeBroker()
	adapter, _, _ := buildTestAdapter(broker, nil)

	subject := ghpermission.EffectiveSubject{RuntimeID: "rt-1", PluginID: "plugin-1", ExtensionID: "ext-1"}
	result := adapter.Check(context.Background(), subject, "")
	if result.Allowed() {
		t.Fatal("expected DENY for empty permission id")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_PluginNotInRegistry(t *testing.T) {
	broker := newFakeBroker()
	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for plugin not in registry")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_RuntimeNotFound(t *testing.T) {
	broker := newFakeBroker()
	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for runtime not found")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_PluginMismatch(t *testing.T) {
	broker := newFakeBroker()
	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-2", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-2", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for plugin mismatch")
	}
	if result.Reason != ghpermission.ReasonInvalidSubject {
		t.Fatalf("expected reason invalid_subject, got %s", result.Reason)
	}
}

func TestCheck_HostPolicyDeny(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)

	policy := func(ctx context.Context, subject ghpermission.EffectiveSubject, permID string) (bool, bool) {
		return false, true
	}

	adapter, resolver, _ := buildTestAdapter(broker, policy)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for host policy deny")
	}
	if result.Reason != ghpermission.ReasonPolicyDenied {
		t.Fatalf("expected reason host_policy_denied, got %s", result.Reason)
	}
}

func TestCheck_PolicyUnhandled(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)

	policy := func(ctx context.Context, subject ghpermission.EffectiveSubject, permID string) (bool, bool) {
		return false, false
	}

	adapter, resolver, _ := buildTestAdapter(broker, policy)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if !result.Allowed() {
		t.Fatalf("expected ALLOW when policy unhandled, got DENY reason=%s", result.Reason)
	}
}

func TestCheck_ServiceLevel_Isolation(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.channel.use", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)
	resolver.addService("rt-1", "svc-1", "plugin-1")
	resolver.addService("rt-1", "svc-2", "plugin-1")

	result1 := adapter.CheckServicePermission(context.Background(), "rt-1", "plugin-1", "svc-1", "gamehost.channel.use")
	if !result1.Allowed() {
		t.Fatalf("expected ALLOW for svc-1 with runtime grant, got DENY reason=%s", result1.Reason)
	}

	result2 := adapter.CheckServicePermission(context.Background(), "rt-1", "plugin-1", "svc-2", "gamehost.channel.use")
	if !result2.Allowed() {
		t.Fatalf("expected ALLOW for svc-2 with runtime grant (service inherits runtime grant), got DENY reason=%s", result2.Reason)
	}
}

func TestCheck_ServiceRevokeByRuntime(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.channel.use", kernelpermission.DecisionAllow)

	adapter, resolver, b := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)
	resolver.addService("rt-1", "svc-1", "plugin-1")

	result1 := adapter.CheckServicePermission(context.Background(), "rt-1", "plugin-1", "svc-1", "gamehost.channel.use")
	if !result1.Allowed() {
		t.Fatalf("expected ALLOW before revoke, got DENY reason=%s", result1.Reason)
	}

	b.addGrant("runtime:rt-1", "gamehost.channel.use", kernelpermission.DecisionDeny)

	result2 := adapter.CheckServicePermission(context.Background(), "rt-1", "plugin-1", "svc-1", "gamehost.channel.use")
	if result2.Allowed() {
		t.Fatal("expected DENY after runtime grant revoke, got ALLOW")
	}
}

func TestCheck_CrossRuntimeSpoof(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)
	resolver.addPlugin("plugin-2", "ext-2")
	resolver.addRuntime("rt-2", "plugin-2", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-2", "plugin-1", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for cross runtime spoof")
	}
}

func TestCheck_CrossServiceSpoof(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1/service:svc-1", "gamehost.channel.use", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)
	resolver.addService("rt-1", "svc-1", "plugin-1")

	result := adapter.CheckServicePermission(context.Background(), "rt-1", "plugin-1", "other-service", "gamehost.channel.use")
	if result.Allowed() {
		t.Fatal("expected DENY for cross-service spoof")
	}
}

func TestCheck_PermissionRevoke(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)

	adapter, resolver, b := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result1 := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if !result1.Allowed() {
		t.Fatalf("expected ALLOW before revoke, got DENY reason=%s", result1.Reason)
	}

	b.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionDeny)

	result2 := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if result2.Allowed() {
		t.Fatal("expected DENY after revoke, got ALLOW")
	}
}

func TestResolveRuntimePermissions_SubsetRelation(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	subject := ghpermission.EffectiveSubject{RuntimeID: "rt-1", PluginID: "plugin-1", ExtensionID: "ext-1"}
	view := adapter.ResolveRuntimePermissions(context.Background(), subject, "gamehost.control", "gamehost.channel.use")

	allowed := view.AllowedPermissions()
	if len(allowed) != 1 {
		t.Fatalf("expected 1 allowed, got %d", len(allowed))
	}
	if allowed[0] != "gamehost.control" {
		t.Fatalf("expected gamehost.control allowed, got %s", allowed[0])
	}
}

func TestConcurrentCheck_NoRace(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
			if !result.Allowed() {
				t.Errorf("expected ALLOW in concurrent check, got DENY reason=%s", result.Reason)
			}
		}()
	}
	wg.Wait()
}

func TestEffectiveView_AllowedPermissions(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)
	broker.addGrant("runtime:rt-1", "gamehost.channel.use", kernelpermission.DecisionAllow)

	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	subject := ghpermission.EffectiveSubject{RuntimeID: "rt-1", PluginID: "plugin-1", ExtensionID: "ext-1"}
	view := adapter.ResolveRuntimePermissions(context.Background(), subject, "gamehost.control", "gamehost.channel.use", "gamehost.host_api.invoke")

	if !view.Allowed("gamehost.control") {
		t.Fatal("expected gamehost.control allowed")
	}
	if !view.Allowed("gamehost.channel.use") {
		t.Fatal("expected gamehost.channel.use allowed")
	}
	if view.Allowed("gamehost.host_api.invoke") {
		t.Fatal("expected gamehost.host_api.invoke denied")
	}

	if len(view.DenyReasons()) != 1 {
		t.Fatalf("expected 1 deny reason, got %d", len(view.DenyReasons()))
	}
}

func TestCheck_HighRiskPermission(t *testing.T) {
	broker := newFakeBroker()
	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	result := adapter.CheckRuntimePermission(context.Background(), "rt-1", "plugin-1", "gamehost.control")
	if result.Allowed() {
		t.Fatal("expected DENY for high risk permission without grant")
	}
	if result.Decision != ghpermission.DecisionDenied {
		t.Fatalf("expected DecisionDenied, got %d", result.Decision)
	}
}

func TestMapServiceSubjectRejectsEmptyServiceID(t *testing.T) {
	resolver := newFakeResolver()
	mapper := ghpermission.NewGameHostSubjectMapper(resolver)
	if _, err := mapper.MapServiceSubject("rt-1", "plugin-1", ""); err == nil {
		t.Fatal("expected empty service id to be rejected for service-scoped subject")
	}
}

func TestEffectiveViewRevisionStableAndOrderIndependent(t *testing.T) {
	broker := newFakeBroker()
	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionAllow)
	adapter, resolver, _ := buildTestAdapter(broker, nil)
	resolver.addPlugin("plugin-1", "ext-1")
	resolver.addRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	subject := ghpermission.EffectiveSubject{RuntimeID: "rt-1", PluginID: "plugin-1", ExtensionID: "ext-1"}
	first := adapter.ResolveRuntimePermissions(context.Background(), subject, "gamehost.control", "gamehost.channel.use")
	time.Sleep(time.Millisecond)
	second := adapter.ResolveRuntimePermissions(context.Background(), subject, "gamehost.channel.use", "gamehost.control")
	if first.Revision == "" || second.Revision == "" {
		t.Fatal("permission revision must not be empty")
	}
	if first.Revision != second.Revision {
		t.Fatalf("same effective permission state must have stable revision: %q vs %q", first.Revision, second.Revision)
	}

	broker.addGrant("runtime:rt-1", "gamehost.control", kernelpermission.DecisionDeny)
	third := adapter.ResolveRuntimePermissions(context.Background(), subject, "gamehost.control", "gamehost.channel.use")
	if third.Revision == first.Revision {
		t.Fatalf("permission decision change must change revision: %q", third.Revision)
	}
}
