package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox_webui"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func makeTestSandboxSession(state sandbox_webui.SessionState, scopeSnapID, permSnapID, charID, convID string) *sandbox_webui.WebSession {
	return &sandbox_webui.WebSession{
		SessionID:            "sess_test_001",
		ContributionID:       "contrib_test_001",
		ExtensionID:          "ext_test_001",
		ModuleID:             "mod_test_001",
		Generation:           1,
		State:                state,
		AllowedActions:       []string{"action_copy", "action_tool", "action_workflow"},
		CharacterID:          charID,
		ConversationID:       convID,
		ScopeSnapshotID:      scopeSnapID,
		PermissionSnapshotID: permSnapID,
		ExpiresAt:            time.Now().UTC().Add(1 * time.Hour),
	}
}

func makeTestSandboxContribution() *ui_contribution.UIContributionDefinition {
	return &ui_contribution.UIContributionDefinition{
		ContributionID: "contrib_test_001",
		ExtensionID:    "ext_test_001",
		ModuleID:       "mod_test_001",
		Actions: []ui_contribution.UIActionDefinition{
			{
				ActionID: "action_copy",
				Title:    ui_contribution.LocalizedText{Default: "Copy"},
				Target:   ui_contribution.UIActionTarget{Type: ui_contribution.ActionTargetCopy},
			},
			{
				ActionID: "action_tool",
				Title:    ui_contribution.LocalizedText{Default: "Tool"},
				Target:   ui_contribution.UIActionTarget{Type: ui_contribution.ActionTargetTool, ToolID: "tool_001"},
			},
			{
				ActionID: "action_workflow",
				Title:    ui_contribution.LocalizedText{Default: "Workflow"},
				Target:   ui_contribution.UIActionTarget{Type: ui_contribution.ActionTargetWorkflow, WorkflowID: "wf_001"},
			},
		},
	}
}

func TestSandboxActionDispatcher_CompleteContext(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_123",
		"perm_snap_456",
		"char_001",
		"conv_001",
	)
	contrib := makeTestSandboxContribution()

	var capturedCtx UIActionExecContext
	var capturedAction *ui_contribution.UIActionDefinition

	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			capturedCtx = execCtx
			capturedAction = action
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	result, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_copy", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("DispatchAction failed: %v", err)
	}

	if capturedCtx.SessionID != "sess_test_001" {
		t.Errorf("expected SessionID sess_test_001, got %s", capturedCtx.SessionID)
	}
	if capturedCtx.ScopeSnapshotID != "scope_snap_123" {
		t.Errorf("expected ScopeSnapshotID scope_snap_123, got %s", capturedCtx.ScopeSnapshotID)
	}
	if capturedCtx.PermissionSnapshotID != "perm_snap_456" {
		t.Errorf("expected PermissionSnapshotID perm_snap_456, got %s", capturedCtx.PermissionSnapshotID)
	}
	if capturedCtx.CharacterID != "char_001" {
		t.Errorf("expected CharacterID char_001, got %s", capturedCtx.CharacterID)
	}
	if capturedCtx.ConversationID != "conv_001" {
		t.Errorf("expected ConversationID conv_001, got %s", capturedCtx.ConversationID)
	}
	if capturedCtx.ExtensionID != "ext_test_001" {
		t.Errorf("expected ExtensionID ext_test_001, got %s", capturedCtx.ExtensionID)
	}
	if capturedCtx.ModuleID != "mod_test_001" {
		t.Errorf("expected ModuleID mod_test_001, got %s", capturedCtx.ModuleID)
	}
	if capturedCtx.Generation != 1 {
		t.Errorf("expected Generation 1, got %d", capturedCtx.Generation)
	}
	if capturedAction == nil || capturedAction.ActionID != "action_copy" {
		t.Errorf("expected action action_copy, got %+v", capturedAction)
	}

	var res map[string]any
	json.Unmarshal(result, &res)
	if res["ok"] != true {
		t.Errorf("expected ok true, got %v", res["ok"])
	}
}

func TestSandboxActionDispatcher_FailClosedOnMissingScopeSnapshot(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"",
		"perm_snap_456",
		"char_001",
		"conv_001",
	)
	contrib := makeTestSandboxContribution()

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_copy", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing scope snapshot, got nil")
	}
	if execCalled {
		t.Fatal("executeAction should not be called when ScopeSnapshotID is empty")
	}
}

func TestSandboxActionDispatcher_RejectExpiredSession(t *testing.T) {
	contrib := makeTestSandboxContribution()

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return nil, sandbox_webui.ErrSessionExpired
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_copy", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
	if execCalled {
		t.Fatal("executeAction should not be called for expired session")
	}
}

func TestSandboxActionDispatcher_RejectNonActiveSession(t *testing.T) {
	states := []sandbox_webui.SessionState{
		sandbox_webui.SessionStateClosed,
		sandbox_webui.SessionStateClosing,
		sandbox_webui.SessionStateSuspended,
		sandbox_webui.SessionStateFailed,
		sandbox_webui.SessionStateQuarantined,
		sandbox_webui.SessionStateExpired,
	}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			session := makeTestSandboxSession(
				state,
				"scope_snap_123",
				"perm_snap_456",
				"char_001",
				"conv_001",
			)
			contrib := makeTestSandboxContribution()

			execCalled := false
			dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
				getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
					return session, nil
				},
				getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
					return contrib, nil
				},
				executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
					execCalled = true
					return json.Marshal(map[string]any{"ok": true})
				},
			})

			_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_copy", json.RawMessage(`{}`))
			if err == nil {
				t.Fatalf("expected error for session state %s, got nil", state)
			}
			if execCalled {
				t.Fatalf("executeAction should not be called for session state %s", state)
			}
		})
	}
}

func TestSandboxActionDispatcher_AllowReadySession(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateReady,
		"scope_snap_123",
		"perm_snap_456",
		"char_001",
		"conv_001",
	)
	contrib := makeTestSandboxContribution()

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_copy", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("expected success for ready session, got error: %v", err)
	}
	if !execCalled {
		t.Fatal("executeAction should be called for ready session")
	}
}

func TestSandboxActionDispatcher_RejectSessionNotFound(t *testing.T) {
	contrib := makeTestSandboxContribution()

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return nil, sandbox_webui.ErrSessionNotFound
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "nonexistent_session", "action_copy", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
	if execCalled {
		t.Fatal("executeAction should not be called for nonexistent session")
	}
}

func TestSandboxActionDispatcher_RejectActionNotAllowed(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_123",
		"perm_snap_456",
		"char_001",
		"conv_001",
	)
	session.AllowedActions = []string{"action_copy"}

	contrib := makeTestSandboxContribution()

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for disallowed action, got nil")
	}
	if execCalled {
		t.Fatal("executeAction should not be called for disallowed action")
	}
}

func TestSandboxActionDispatcher_RejectContributionNotFound(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_123",
		"perm_snap_456",
		"char_001",
		"conv_001",
	)

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return nil, errors.New("contribution not found")
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_copy", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing contribution, got nil")
	}
	if execCalled {
		t.Fatal("executeAction should not be called when contribution is missing")
	}
}

func TestSandboxActionDispatcher_RejectActionNotDeclared(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_123",
		"perm_snap_456",
		"char_001",
		"conv_001",
	)
	session.AllowedActions = []string{"undeclared_action"}

	contrib := makeTestSandboxContribution()

	execCalled := false
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			execCalled = true
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "undeclared_action", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for undeclared action, got nil")
	}
	if execCalled {
		t.Fatal("executeAction should not be called for undeclared action")
	}
}

func TestSandboxActionDispatcher_CrossCharacterIsolation(t *testing.T) {
	sessionCharA := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_A",
		"perm_snap_A",
		"char_A",
		"conv_A",
	)
	sessionCharB := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_B",
		"perm_snap_B",
		"char_B",
		"conv_B",
	)
	contrib := makeTestSandboxContribution()

	var capturedCtx UIActionExecContext
	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			if sessionID == "sess_char_A" {
				return sessionCharA, nil
			}
			return sessionCharB, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			capturedCtx = execCtx
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	dispatcher.DispatchAction(context.Background(), "sess_char_A", "action_copy", json.RawMessage(`{}`))
	if capturedCtx.CharacterID != "char_A" {
		t.Errorf("expected CharacterID char_A, got %s", capturedCtx.CharacterID)
	}
	if capturedCtx.ScopeSnapshotID != "scope_snap_A" {
		t.Errorf("expected ScopeSnapshotID scope_snap_A, got %s", capturedCtx.ScopeSnapshotID)
	}

	dispatcher.DispatchAction(context.Background(), "sess_char_B", "action_copy", json.RawMessage(`{}`))
	if capturedCtx.CharacterID != "char_B" {
		t.Errorf("expected CharacterID char_B, got %s", capturedCtx.CharacterID)
	}
	if capturedCtx.ScopeSnapshotID != "scope_snap_B" {
		t.Errorf("expected ScopeSnapshotID scope_snap_B, got %s", capturedCtx.ScopeSnapshotID)
	}
}

func TestSandboxActionDispatcher_WorkflowActionPassesScopeGate(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_wf",
		"perm_snap_wf",
		"char_wf",
		"conv_wf",
	)
	contrib := makeTestSandboxContribution()

	var capturedCtx UIActionExecContext
	var capturedActionType ui_contribution.UIActionTargetType

	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			capturedCtx = execCtx
			capturedActionType = action.Target.Type
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_workflow", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("workflow action dispatch failed: %v", err)
	}

	if capturedActionType != ui_contribution.ActionTargetWorkflow {
		t.Errorf("expected action type workflow, got %s", capturedActionType)
	}
	if capturedCtx.ScopeSnapshotID != "scope_snap_wf" {
		t.Errorf("workflow action should receive ScopeSnapshotID, got %s", capturedCtx.ScopeSnapshotID)
	}
	if capturedCtx.PermissionSnapshotID != "perm_snap_wf" {
		t.Errorf("workflow action should receive PermissionSnapshotID, got %s", capturedCtx.PermissionSnapshotID)
	}
	if capturedCtx.CharacterID != "char_wf" {
		t.Errorf("workflow action should receive CharacterID, got %s", capturedCtx.CharacterID)
	}
	if capturedCtx.ConversationID != "conv_wf" {
		t.Errorf("workflow action should receive ConversationID, got %s", capturedCtx.ConversationID)
	}
}

func TestSandboxActionDispatcher_ToolActionPassesScopeGate(t *testing.T) {
	session := makeTestSandboxSession(
		sandbox_webui.SessionStateActive,
		"scope_snap_tool",
		"perm_snap_tool",
		"char_tool",
		"conv_tool",
	)
	contrib := makeTestSandboxContribution()

	var capturedCtx UIActionExecContext

	dispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: func(sessionID string) (*sandbox_webui.WebSession, error) {
			return session, nil
		},
		getContribution: func(id string) (*ui_contribution.UIContributionDefinition, error) {
			return contrib, nil
		},
		executeAction: func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			capturedCtx = execCtx
			return json.Marshal(map[string]any{"ok": true})
		},
	})

	_, err := dispatcher.DispatchAction(context.Background(), "sess_test_001", "action_tool", json.RawMessage(`{"input":"data"}`))
	if err != nil {
		t.Fatalf("tool action dispatch failed: %v", err)
	}

	if capturedCtx.ScopeSnapshotID != "scope_snap_tool" {
		t.Errorf("tool action should receive ScopeSnapshotID, got %s", capturedCtx.ScopeSnapshotID)
	}
	if capturedCtx.PermissionSnapshotID != "perm_snap_tool" {
		t.Errorf("tool action should receive PermissionSnapshotID, got %s", capturedCtx.PermissionSnapshotID)
	}
}
