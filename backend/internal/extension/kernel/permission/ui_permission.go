package permission

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type UIPermissionChecker struct{}

func NewUIPermissionChecker() *UIPermissionChecker {
	return &UIPermissionChecker{}
}

func (c *UIPermissionChecker) CheckContributionPermissions(def *ui_contribution.UIContributionDefinition, grantedPerms []string) (missing []string, err error) {
	missing = make([]string, 0)
	if def == nil {
		return missing, nil
	}
	granted := make(map[string]bool, len(grantedPerms))
	for _, p := range grantedPerms {
		granted[p] = true
	}
	for _, req := range def.Permissions {
		if req.Required && !granted[req.Name] {
			missing = append(missing, req.Name)
		}
	}
	return missing, nil
}

func (c *UIPermissionChecker) CheckContributionScopes(def *ui_contribution.UIContributionDefinition, grantedScopes []string) (missing []string, err error) {
	missing = make([]string, 0)
	if def == nil {
		return missing, nil
	}
	if len(def.ScopeRule.RequiredScopes) == 0 {
		return missing, nil
	}
	granted := make(map[string]bool, len(grantedScopes))
	for _, s := range grantedScopes {
		granted[s] = true
	}
	for _, sc := range def.ScopeRule.RequiredScopes {
		if !granted[sc] {
			missing = append(missing, sc)
		}
	}
	return missing, nil
}

func (c *UIPermissionChecker) ValidateSessionRequest(def *ui_contribution.UIContributionDefinition, grantedScopes, grantedPerms []string) error {
	missingPerms, _ := c.CheckContributionPermissions(def, grantedPerms)
	if len(missingPerms) > 0 {
		return fmt.Errorf("ui_contribution: missing required permissions: %v", missingPerms)
	}
	missingScopes, _ := c.CheckContributionScopes(def, grantedScopes)
	if len(missingScopes) > 0 {
		return fmt.Errorf("ui_contribution: missing required scopes: %v", missingScopes)
	}
	return nil
}

type SessionAuthorization struct {
	ExtensionID   string
	ModuleID      string
	Generation    int64
	GrantedPerms  []string
	GrantedScopes []string
	ScopeSnapshot *scope.ScopeSnapshot
}

type UISessionAuthorizer struct {
	broker   PermissionBroker
	scopeMgr scope.ScopeManager
}

func NewUISessionAuthorizer(broker PermissionBroker, scopeMgr scope.ScopeManager) *UISessionAuthorizer {
	return &UISessionAuthorizer{broker: broker, scopeMgr: scopeMgr}
}

func (a *UISessionAuthorizer) AuthorizeSession(
	ctx context.Context,
	def *ui_contribution.UIContributionDefinition,
	characterID, conversationID string,
) (*SessionAuthorization, error) {
	if a == nil {
		return nil, fmt.Errorf("ui_authorizer: not configured")
	}
	if def == nil {
		return nil, fmt.Errorf("ui_authorizer: nil definition")
	}

	extID := string(def.ExtensionID)
	modID := string(def.ModuleID)

	permReqs := make([]PermissionRequirement, 0, len(def.Permissions))
	for _, p := range def.Permissions {
		if !p.Required {
			continue
		}
		permReqs = append(permReqs, PermissionRequirement{
			PermissionID: p.Name,
			Scope:        resolvePermScope(p.Scope, characterID, conversationID, extID),
		})
	}

	if len(permReqs) > 0 {
		if a.broker == nil {
			return nil, fmt.Errorf("ui_authorizer: permission broker not configured (fail-closed)")
		}
		subject := SubjectForExtension(extID)
		result := a.broker.Evaluate(ctx, PermissionEvaluationRequest{
			Subject:      subject,
			Requirements: permReqs,
		})
		if result.Decision != DecisionAllow && result.Decision != DecisionAllowOnce &&
			result.Decision != DecisionAllowSession && result.Decision != DecisionAllowPersistent {
			missingNames := make([]string, 0, len(result.Missing))
			for _, m := range result.Missing {
				missingNames = append(missingNames, m.PermissionID)
			}
			return nil, fmt.Errorf("ui_authorizer: permission denied (decision=%s missing=%v)", result.Decision, missingNames)
		}
	}

	grantedPerms := make([]string, 0, len(def.Permissions))
	for _, p := range def.Permissions {
		if p.Required {
			grantedPerms = append(grantedPerms, p.Name)
		}
	}

	grantedScopes := make([]string, 0, len(def.ScopeRule.RequiredScopes))
	grantedScopes = append(grantedScopes, def.ScopeRule.RequiredScopes...)

	if len(def.ScopeRule.RequiredScopes) > 0 || characterID != "" || conversationID != "" {
		if a.scopeMgr == nil {
			return nil, fmt.Errorf("ui_authorizer: scope manager not configured (fail-closed)")
		}
		subjectType := scope.SubjectUIContribution
		subjectID := string(def.ContributionID)
		if extID != "" {
			subjectID = extID
		}
		decision := a.scopeMgr.Evaluate(ctx, scope.ScopeEvaluationRequest{
			SubjectType:    subjectType,
			SubjectID:      subjectID,
			CharacterID:    characterID,
			ConversationID: conversationID,
			ExtensionID:    extID,
			ModuleID:       modID,
			Generation:     def.Integrity.Generation,
		})
		if !decision.Allowed && len(def.ScopeRule.RequiredScopes) > 0 {
			reasonCodes := make([]string, 0, len(decision.Reasons))
			for _, r := range decision.Reasons {
				reasonCodes = append(reasonCodes, r.Code)
			}
			return nil, fmt.Errorf("ui_authorizer: scope denied (reasons=%v)", reasonCodes)
		}
	}

	auth := &SessionAuthorization{
		ExtensionID:   extID,
		ModuleID:      modID,
		Generation:    def.Integrity.Generation,
		GrantedPerms:  grantedPerms,
		GrantedScopes: grantedScopes,
	}
	return auth, nil
}

func resolvePermScope(scopeStr, characterID, conversationID, extID string) PermissionScope {
	switch scopeStr {
	case "character", "Character":
		return ScopeForCharacter(characterID)
	case "conversation", "Conversation":
		return ScopeForConversation(conversationID)
	case "extension", "Extension":
		return ScopeForExtension(extID)
	default:
		return ScopeGlobalOnly()
	}
}
