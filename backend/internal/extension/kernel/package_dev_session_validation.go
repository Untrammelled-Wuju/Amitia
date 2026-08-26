package kernel

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
)

// validateDeveloperSessionBinding is the single admission rule for unsigned
// development packages. Preview, confirmation, install, and update reuse this
// rule so an unsigned package cannot advance under a stale or cross-boundary
// developer trust snapshot.
func validateDeveloperSessionBinding(
	sessions *dev_mode.SessionManager,
	workspaces *dev_mode.WorkspaceRegistry,
	sessionID, userID, extensionID string,
) error {
	if !packageDevelopmentModeEnabled() {
		return fmt.Errorf("kernel: developer mode is disabled")
	}
	sessionID = strings.TrimSpace(sessionID)
	userID = strings.TrimSpace(userID)
	extensionID = strings.TrimSpace(extensionID)
	if sessionID == "" || userID == "" || extensionID == "" || sessions == nil || workspaces == nil {
		return fmt.Errorf("kernel: developer session binding unavailable")
	}
	session, err := sessions.Validate(sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID || string(session.ExtensionID) != extensionID || session.PolicyVersion != packagePolicyVersion || session.Environment != "development" || !session.DevTrustSnapshot {
		return fmt.Errorf("kernel: developer session binding mismatch")
	}
	workspace, err := workspaces.Get(session.WorkspaceID)
	if err != nil {
		return err
	}
	if workspace.OwnerUserID != userID || string(workspace.ExtensionID) != extensionID || !workspace.DevTrust || workspace.DevTrustVersion != session.DevTrustVersion {
		return fmt.Errorf("kernel: developer workspace trust invalid")
	}
	if len(session.Scopes) != 1 || session.Scopes[0] != "extensions.install.unsigned" {
		return fmt.Errorf("kernel: developer session scope invalid")
	}
	return nil
}
