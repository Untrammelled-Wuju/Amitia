package permission

import (
	"context"
	"testing"
	"time"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestApprovalCoordinator_ApproveConsumesAllowOnce(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	coordinator, err := NewApprovalCoordinator(broker)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator: %v", err)
	}
	coordinator.SetTTL(2 * time.Second)

	subject := EffectiveSubject{
		RuntimeID:   "runtime-1",
		PluginID:    "plugin-1",
		ServiceID:   "service-1",
		ExtensionID: "extension-1",
	}
	request := kernelpermission.PermissionEvaluationRequest{
		Subject: subject.KernelSubject(),
		Requirements: []kernelpermission.PermissionRequirement{{
			PermissionID: kernelpermission.PermissionGameHostControl,
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resultCh := make(chan kernelpermission.PermissionEvaluationResult, 1)
	go func() {
		resultCh <- coordinator.Evaluate(ctx, subject, kernelpermission.PermissionGameHostControl, request)
	}()

	var approval PendingApproval
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending := coordinator.ListPending()
		if len(pending) == 1 {
			approval = pending[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if approval.ID == "" {
		t.Fatal("expected one pending approval")
	}
	if err := coordinator.Approve(approval.ID, "test-user", "approved for this operation"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.Decision != kernelpermission.DecisionAllow {
			t.Fatalf("expected approved operation to resume with allow, got %q (%v)", result.Decision, result.Reasons)
		}
	case <-ctx.Done():
		t.Fatalf("approval evaluation did not resume: %v", ctx.Err())
	}

	active, err := broker.ListGrants(context.Background(), kernelpermission.PermissionGrantFilter{
		Subject:    ptrPermissionSubject(subject.KernelSubject()),
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	for _, grant := range active {
		if grant.PermissionID == kernelpermission.PermissionGameHostControl && grant.IsOneTime() {
			t.Fatalf("allow_once grant %q remained active after approved operation resumed", grant.GrantID)
		}
	}

	second := broker.Evaluate(context.Background(), request)
	if second.Decision != kernelpermission.DecisionRequireApproval {
		t.Fatalf("expected a second operation to require a fresh approval, got %q", second.Decision)
	}
}

func TestDefaultPermissionBroker_EvaluateDoesNotConsumeAllowOnce(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	subject := kernelpermission.SubjectForExtension("extension-1")
	grant, err := broker.Grant(context.Background(), kernelpermission.PermissionGrantRequest{
		Subject:      subject,
		PermissionID: kernelpermission.PermissionGameHostControl,
		Scope:        kernelpermission.ScopeForExtension("extension-1"),
		Decision:     kernelpermission.DecisionAllowOnce,
		IssuedBy:     kernelpermission.IssuerUser,
	})
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	request := kernelpermission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []kernelpermission.PermissionRequirement{{
			PermissionID: kernelpermission.PermissionGameHostControl,
			Scope:        kernelpermission.ScopeForExtension("extension-1"),
		}},
	}
	for i := 0; i < 2; i++ {
		result := broker.Evaluate(context.Background(), request)
		if result.Decision != kernelpermission.DecisionAllowOnce {
			t.Fatalf("evaluation %d: expected allow_once without consumption, got %q", i+1, result.Decision)
		}
	}
	stored, found, err := storage.GetByGrantID(context.Background(), grant.GrantID)
	if err != nil || !found {
		t.Fatalf("GetByGrantID: found=%v err=%v", found, err)
	}
	if stored.RevokedAt != nil {
		t.Fatal("permission evaluation consumed allow_once grant; evaluation must be side-effect free")
	}
}

func ptrPermissionSubject(subject kernelpermission.PermissionSubject) *kernelpermission.PermissionSubject {
	return &subject
}

func TestApprovalCoordinator_PendingIncludesGenericTarget(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	coordinator, err := NewApprovalCoordinator(broker)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator: %v", err)
	}
	coordinator.SetTTL(2 * time.Second)

	subject := EffectiveSubject{RuntimeID: "runtime-target", PluginID: "plugin-target", ServiceID: "service-target", ExtensionID: "extension-target"}
	request := kernelpermission.PermissionEvaluationRequest{
		Subject: subject.KernelSubject(),
		Requirements: []kernelpermission.PermissionRequirement{{
			PermissionID: kernelpermission.PermissionGameHostArtifactDeploy,
			Scope:        kernelpermission.PermissionScope{Type: kernelpermission.ScopeModule, ID: subject.ServiceID},
		}},
		Target: kernelpermission.PermissionTarget{Type: "directory", ID: "bridge", Path: "/tmp/example-game"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resultCh := make(chan kernelpermission.PermissionEvaluationResult, 1)
	go func() {
		resultCh <- coordinator.Evaluate(ctx, subject, kernelpermission.PermissionGameHostArtifactDeploy, request)
	}()

	var approval PendingApproval
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending := coordinator.ListPending()
		if len(pending) == 1 {
			approval = pending[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if approval.ID == "" {
		t.Fatal("expected pending approval")
	}
	if approval.Target.Type != "directory" || approval.Target.ID != "bridge" || approval.Target.Path != "/tmp/example-game" {
		t.Fatalf("pending approval target = %#v", approval.Target)
	}
	if err := coordinator.Reject(approval.ID, "test-user", "target not approved"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	select {
	case result := <-resultCh:
		if result.Decision != kernelpermission.DecisionDeny {
			t.Fatalf("expected rejected target approval to deny, got %q", result.Decision)
		}
	case <-ctx.Done():
		t.Fatalf("approval evaluation did not resume: %v", ctx.Err())
	}
}

func TestApprovalCoordinator_ContextCancellationDoesNotLeaveAllowOnceGrant(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	coordinator, err := NewApprovalCoordinator(broker)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator: %v", err)
	}
	coordinator.SetTTL(5 * time.Second)

	subject := EffectiveSubject{RuntimeID: "runtime-cancel", PluginID: "plugin-cancel", ServiceID: "service-cancel", ExtensionID: "extension-cancel"}
	request := kernelpermission.PermissionEvaluationRequest{
		Subject: subject.KernelSubject(),
		Requirements: []kernelpermission.PermissionRequirement{{
			PermissionID: kernelpermission.PermissionGameHostControl,
			Scope:        kernelpermission.PermissionScope{Type: kernelpermission.ScopeModule, ID: subject.ServiceID},
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan kernelpermission.PermissionEvaluationResult, 1)
	go func() {
		resultCh <- coordinator.Evaluate(ctx, subject, kernelpermission.PermissionGameHostControl, request)
	}()

	var approval PendingApproval
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending := coordinator.ListPending()
		if len(pending) == 1 {
			approval = pending[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if approval.ID == "" {
		cancel()
		t.Fatal("expected pending approval")
	}
	cancel()

	select {
	case result := <-resultCh:
		if result.Decision != kernelpermission.DecisionDeny {
			t.Fatalf("expected cancelled approval to deny, got %q", result.Decision)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled approval did not return")
	}

	active, err := broker.ListGrants(context.Background(), kernelpermission.PermissionGrantFilter{Subject: ptrPermissionSubject(subject.KernelSubject()), ActiveOnly: true})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	for _, grant := range active {
		if grant.PermissionID == kernelpermission.PermissionGameHostControl && grant.IsOneTime() {
			t.Fatalf("allow_once grant %q remained after cancellation", grant.GrantID)
		}
	}
	if err := coordinator.Approve(approval.ID, "late-user", "too late"); err == nil {
		t.Fatal("expected approval after cancellation to be rejected")
	}
}

func TestApprovalCoordinator_SerializesSamePermissionRequests(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	coordinator, err := NewApprovalCoordinator(broker)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator: %v", err)
	}
	coordinator.SetTTL(3 * time.Second)

	subject := EffectiveSubject{RuntimeID: "runtime-serial", PluginID: "plugin-serial", ServiceID: "service-serial", ExtensionID: "extension-serial"}
	request := kernelpermission.PermissionEvaluationRequest{
		Subject: subject.KernelSubject(),
		Requirements: []kernelpermission.PermissionRequirement{{
			PermissionID: kernelpermission.PermissionGameHostControl,
			Scope:        kernelpermission.PermissionScope{Type: kernelpermission.ScopeModule, ID: subject.ServiceID},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan kernelpermission.PermissionEvaluationResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			resultCh <- coordinator.Evaluate(ctx, subject, kernelpermission.PermissionGameHostControl, request)
		}()
	}

	first := waitForSinglePendingApproval(t, coordinator, time.Second)
	if err := coordinator.Approve(first.ID, "test-user", "first"); err != nil {
		t.Fatalf("approve first: %v", err)
	}
	firstResult := <-resultCh
	if firstResult.Decision != kernelpermission.DecisionAllow {
		t.Fatalf("first decision = %q", firstResult.Decision)
	}

	second := waitForSinglePendingApproval(t, coordinator, time.Second)
	if second.ID == first.ID {
		t.Fatal("second request reused first approval")
	}
	if err := coordinator.Approve(second.ID, "test-user", "second"); err != nil {
		t.Fatalf("approve second: %v", err)
	}
	secondResult := <-resultCh
	if secondResult.Decision != kernelpermission.DecisionAllow {
		t.Fatalf("second decision = %q", secondResult.Decision)
	}
}

func waitForSinglePendingApproval(t *testing.T, coordinator *ApprovalCoordinator, timeout time.Duration) PendingApproval {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pending := coordinator.ListPending()
		if len(pending) == 1 {
			return pending[0]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected exactly one pending approval")
	return PendingApproval{}
}

func TestApprovalCoordinator_DoesNotSuspendPersistentPermission(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	coordinator, err := NewApprovalCoordinator(broker)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator: %v", err)
	}
	subject := EffectiveSubject{RuntimeID: "runtime-persistent", PluginID: "plugin-persistent", ServiceID: "service-persistent", ExtensionID: "extension-persistent"}
	request := kernelpermission.PermissionEvaluationRequest{
		Subject: subject.KernelSubject(),
		Requirements: []kernelpermission.PermissionRequirement{{
			PermissionID: kernelpermission.PermissionGameHostChannelUse,
			Scope:        kernelpermission.PermissionScope{Type: kernelpermission.ScopeModule, ID: subject.ServiceID},
		}},
	}

	result := coordinator.Evaluate(context.Background(), subject, kernelpermission.PermissionGameHostChannelUse, request)
	if result.Decision != kernelpermission.DecisionRequireApproval {
		t.Fatalf("expected persistent permission to remain require_approval, got %q", result.Decision)
	}
	if pending := coordinator.ListPending(); len(pending) != 0 {
		t.Fatalf("persistent permission inspection created %d GameHost pending approvals", len(pending))
	}
}

func TestEffectivePermissionResolveView_DoesNotCreatePendingApproval(t *testing.T) {
	registry := kernelpermission.NewPermissionDefinitionRegistry()
	storage := kernelpermission.NewMemoryPermissionStorage()
	broker := kernelpermission.NewDefaultPermissionBroker(registry, storage)
	defer broker.Close()

	coordinator, err := NewApprovalCoordinator(broker)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator: %v", err)
	}
	adapter := NewEffectivePermissionAdapter(broker, nil, nil)
	adapter.SetApprovalCoordinator(coordinator)
	subject := EffectiveSubject{RuntimeID: "runtime-view", PluginID: "plugin-view", ServiceID: "service-view", ExtensionID: "extension-view"}

	view := adapter.ResolveServicePermissions(context.Background(), subject, kernelpermission.PermissionGameHostControl)
	if len(view.Checks) != 1 || view.Checks[0].Decision != DecisionRequireApproval {
		t.Fatalf("expected passive view to report require_approval, got %#v", view.Checks)
	}
	if pending := coordinator.ListPending(); len(pending) != 0 {
		t.Fatalf("passive permission view created %d pending approvals", len(pending))
	}
}
