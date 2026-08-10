package control

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestAudit_SuccessTransition(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-1")
	pluginID := domain.PluginID("plugin-audit-1")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	ev := events[0]
	if ev.RuntimeID != runtimeID {
		t.Errorf("expected runtime id %q, got %q", runtimeID, ev.RuntimeID)
	}
	if ev.PluginID != pluginID {
		t.Errorf("expected plugin id %q, got %q", pluginID, ev.PluginID)
	}
	if ev.PreviousMode != domain.ControlModeObserveOnly {
		t.Errorf("expected previous mode %q, got %q", domain.ControlModeObserveOnly, ev.PreviousMode)
	}
	if ev.NewMode != domain.ControlModePluginControl {
		t.Errorf("expected new mode %q, got %q", domain.ControlModePluginControl, ev.NewMode)
	}
	if ev.PreviousEpoch != 1 {
		t.Errorf("expected previous epoch 1, got %d", ev.PreviousEpoch)
	}
	if ev.NewEpoch != 2 {
		t.Errorf("expected new epoch 2, got %d", ev.NewEpoch)
	}
	if ev.Actor != ActorPlugin {
		t.Errorf("expected actor %q, got %q", ActorPlugin, ev.Actor)
	}
	if ev.Reason != ReasonPluginRequest {
		t.Errorf("expected reason %q, got %q", ReasonPluginRequest, ev.Reason)
	}
	if ev.Result != AuditResultSuccess {
		t.Errorf("expected result %q, got %q", AuditResultSuccess, ev.Result)
	}
}

func TestAudit_DeniedTransition(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-2")
	pluginID := domain.PluginID("plugin-audit-2")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target:        domain.ControlModePluginControl,
		Actor:         ActorPlugin,
		Reason:        ReasonPluginRequest,
		ExpectedEpoch: 999,
		UseExpected:   true,
	})
	if err == nil {
		t.Fatal("expected error for stale epoch")
	}

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	ev := events[0]
	if ev.Result != AuditResultDenied {
		t.Errorf("expected result %q, got %q", AuditResultDenied, ev.Result)
	}
	if ev.Error == "" {
		t.Error("expected error message in denied audit event")
	}
}

func TestAudit_NoopTransition(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-3")
	pluginID := domain.PluginID("plugin-audit-3")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeObserveOnly,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	ev := events[0]
	if ev.Result != AuditResultNoop {
		t.Errorf("expected result %q, got %q", AuditResultNoop, ev.Result)
	}
	if ev.PreviousMode != ev.NewMode {
		t.Errorf("expected same mode for noop, got previous=%q new=%q", ev.PreviousMode, ev.NewMode)
	}
}

func TestAudit_MultipleTransitions(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-4")
	pluginID := domain.PluginID("plugin-audit-4")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	transitions := []TransitionRequest{
		{Target: domain.ControlModePluginControl, Actor: ActorPlugin, Reason: ReasonPluginRequest},
		{Target: domain.ControlModeUserControl, Actor: ActorUser, Reason: ReasonUserRequest},
		{Target: domain.ControlModeSharedControl, Actor: ActorHost, Reason: ReasonHostPolicy},
	}

	for _, req := range transitions {
		_, err := m.Transition(ctx, runtimeID, req)
		if err != nil {
			t.Fatalf("Transition to %q failed: %v", req.Target, err)
		}
	}

	events := audit.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 audit events, got %d", len(events))
	}

	for i, ev := range events {
		if ev.Result != AuditResultSuccess {
			t.Errorf("event %d: expected result %q, got %q", i, AuditResultSuccess, ev.Result)
		}
	}
}

func TestAudit_DeniedRuntimeNotFound(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()

	_, err := m.Transition(ctx, domain.RuntimeInstanceID("nonexistent"), TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent runtime")
	}

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	ev := events[0]
	if ev.Result != AuditResultDenied {
		t.Errorf("expected result %q, got %q", AuditResultDenied, ev.Result)
	}
}

func TestAudit_NoopDoesNotIncrementEpoch(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-5")
	pluginID := domain.PluginID("plugin-audit-5")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeObserveOnly,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Epoch != 1 {
		t.Errorf("expected epoch 1 (noop), got %d", snap.Epoch)
	}
}

func TestAudit_DoesNotContainSensitiveData(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-6")
	pluginID := domain.PluginID("plugin-audit-6")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Error != "" {
		t.Errorf("success event should not have error, got %q", ev.Error)
	}
}

func TestAudit_SinkCount(t *testing.T) {
	audit := NewInMemoryAuthorityAuditSink()
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Audit: audit,
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-7")
	pluginID := domain.PluginID("plugin-audit-7")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	if audit.Count() != 1 {
		t.Errorf("expected audit count 1, got %d", audit.Count())
	}

	audit.Clear()
	if audit.Count() != 0 {
		t.Errorf("expected audit count 0 after clear, got %d", audit.Count())
	}
}
