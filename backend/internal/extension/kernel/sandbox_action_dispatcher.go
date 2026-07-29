package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox_webui"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type sandboxActionDispatcherDeps struct {
	getSession      func(sessionID string) (*sandbox_webui.WebSession, error)
	getContribution func(contributionID string) (*ui_contribution.UIContributionDefinition, error)
	executeAction   func(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error)
}

func buildSandboxActionDispatcher(deps sandboxActionDispatcherDeps) *sandbox_webui.BridgeActionDispatcher {
	return sandbox_webui.NewBridgeActionDispatcher(func(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error) {
		session, err := deps.getSession(sessionID)
		if err != nil {
			return nil, fmt.Errorf("sandbox: session not found: %w", err)
		}
		if session.State != sandbox_webui.SessionStateActive && session.State != sandbox_webui.SessionStateReady {
			return nil, fmt.Errorf("sandbox: session %s is not active (state: %s)", sessionID, session.State)
		}
		if session.ScopeSnapshotID == "" {
			return nil, fmt.Errorf("sandbox: session %s missing scope snapshot (fail closed)", sessionID)
		}
		if !session.IsActionAllowed(actionID) {
			return nil, fmt.Errorf("sandbox: action %s not allowed for session %s", actionID, sessionID)
		}
		def, err := deps.getContribution(session.ContributionID)
		if err != nil || def == nil {
			return nil, fmt.Errorf("sandbox: contribution not found for session %s", sessionID)
		}
		action := findActionByID(def, actionID)
		if action == nil {
			return nil, fmt.Errorf("sandbox: action %s not declared in contribution", actionID)
		}
		return deps.executeAction(ctx, UIActionExecContext{
			SessionID:            sessionID,
			ContributionID:       session.ContributionID,
			ExtensionID:          session.ExtensionID,
			ModuleID:             session.ModuleID,
			Generation:           session.Generation,
			ScopeSnapshotID:      session.ScopeSnapshotID,
			PermissionSnapshotID: session.PermissionSnapshotID,
			CharacterID:          session.CharacterID,
			ConversationID:       session.ConversationID,
		}, action, input)
	})
}
