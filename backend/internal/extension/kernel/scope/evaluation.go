package scope

import (
	"context"
	"time"
)

const (
	ReasonNoBinding            = "no_binding"
	ReasonCharacterMismatch    = "character_mismatch"
	ReasonConversationMismatch = "conversation_mismatch"
	ReasonExtensionMismatch    = "extension_mismatch"
	ReasonModuleDisabled       = "module_disabled"
	ReasonResourceMismatch     = "resource_mismatch"
	ReasonBindingExpired       = "binding_expired"
	ReasonBindingRevoked       = "binding_revoked"
	ReasonParentScopeTooNarrow = "parent_scope_too_narrow"
	ReasonConversationNotOwned = "conversation_not_owned_by_character"
	ReasonCharacterDeleted     = "character_deleted"
	ReasonConversationDeleted  = "conversation_deleted"
	ReasonExtensionDisabled    = "extension_disabled"
	ReasonInvocationExpired    = "invocation_expired"
	ReasonSessionExpired       = "session_expired"
)

type ScopeEvaluationRequest struct {
	SubjectType    ScopeSubjectType `json:"subjectType"`
	SubjectID      string           `json:"subjectId"`
	CharacterID    string           `json:"characterId,omitempty"`
	ConversationID string           `json:"conversationId,omitempty"`
	ExtensionID    string           `json:"extensionId,omitempty"`
	ModuleID       string           `json:"moduleId,omitempty"`
	ResourceType   string           `json:"resourceType,omitempty"`
	ResourceID     string           `json:"resourceId,omitempty"`
	InvocationID   string           `json:"invocationId,omitempty"`
	SessionID      string           `json:"sessionId,omitempty"`
	Generation     int64            `json:"generation,omitempty"`
	ParentSnapshot *ScopeSnapshot   `json:"parentSnapshot,omitempty"`
}

type ScopeEvaluator interface {
	Evaluate(ctx context.Context, req ScopeEvaluationRequest) ScopeDecision
}

type DefaultScopeEvaluator struct {
	store   ScopeStore
	checker ScopeRelationChecker
}

func NewScopeEvaluator(store ScopeStore, checker ScopeRelationChecker) *DefaultScopeEvaluator {
	return &DefaultScopeEvaluator{store: store, checker: checker}
}

func (e *DefaultScopeEvaluator) Evaluate(ctx context.Context, req ScopeEvaluationRequest) ScopeDecision {
	bindings, err := e.store.ListBindings(ctx, ScopeBindingFilter{
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
	})
	if err != nil || len(bindings) == 0 {
		return ScopeDecision{
			Allowed: false,
			Reasons: []ScopeReason{{Code: ReasonNoBinding, Description: "no scope binding found"}},
		}
	}
	if e.checker != nil {
		if req.CharacterID != "" && e.checker.IsCharacterDeleted(ctx, req.CharacterID) {
			return ScopeDecision{Allowed: false, Reasons: []ScopeReason{{Code: ReasonCharacterDeleted, SubjectID: req.CharacterID}}}
		}
		if req.ConversationID != "" && e.checker.IsConversationDeleted(ctx, req.ConversationID) {
			return ScopeDecision{Allowed: false, Reasons: []ScopeReason{{Code: ReasonConversationDeleted, SubjectID: req.ConversationID}}}
		}
		if req.CharacterID != "" && req.ConversationID != "" && !e.checker.ConversationBelongsToCharacter(ctx, req.ConversationID, req.CharacterID) {
			return ScopeDecision{Allowed: false, Reasons: []ScopeReason{{Code: ReasonConversationNotOwned, SubjectID: req.ConversationID}}}
		}
	}

	var matched []ScopeBinding
	for _, b := range bindings {
		if !b.IsActive() {
			continue
		}
		if e.matchesContext(ctx, b.Scope, req) {
			matched = append(matched, b)
		}
	}

	if len(matched) == 0 {
		reasons := e.buildDenialReasons(bindings, req)
		return ScopeDecision{Allowed: false, Reasons: reasons}
	}

	if req.ParentSnapshot != nil {
		if !e.checkInheritance(matched, *req.ParentSnapshot) {
			return ScopeDecision{
				Allowed: false,
				Reasons: []ScopeReason{{Code: ReasonParentScopeTooNarrow, Description: "child scope would exceed parent scope"}},
			}
		}
	}

	return ScopeDecision{Allowed: true, Matched: matched}
}

func (e *DefaultScopeEvaluator) matchesContext(ctx context.Context, scope ScopeRef, req ScopeEvaluationRequest) bool {
	switch scope.Type {
	case ScopeGlobal:
		return true
	case ScopeCharacter:
		return req.CharacterID != "" && scope.CharacterID == req.CharacterID
	case ScopeConversation:
		if req.ConversationID == "" || scope.ConversationID != req.ConversationID {
			return false
		}
		return req.CharacterID == "" || e.checker != nil && e.checker.ConversationBelongsToCharacter(ctx, req.ConversationID, req.CharacterID)
	case ScopeExtension:
		return req.ExtensionID != "" && scope.ExtensionID == req.ExtensionID
	case ScopeModule:
		return req.ExtensionID != "" && req.ModuleID != "" &&
			scope.ExtensionID == req.ExtensionID && scope.ModuleID == req.ModuleID
	case ScopeResource:
		if scope.ExtensionID != "" && (req.ExtensionID == "" || scope.ExtensionID != req.ExtensionID) {
			return false
		}
		if scope.ModuleID != "" && (req.ModuleID == "" || scope.ModuleID != req.ModuleID) {
			return false
		}
		if scope.ResourceType != "" && (req.ResourceType == "" || scope.ResourceType != req.ResourceType) {
			return false
		}
		if scope.ResourceID != "" && (req.ResourceID == "" || scope.ResourceID != req.ResourceID) {
			return false
		}
		return e.checker != nil && e.checker.ResourceOwnedBy(ctx, req.ResourceID, req.ResourceType, req.ExtensionID, req.ModuleID)
	case ScopeInvocation:
		if scope.ExtensionID != "" && (req.ExtensionID == "" || scope.ExtensionID != req.ExtensionID) {
			return false
		}
		if scope.ModuleID != "" && (req.ModuleID == "" || scope.ModuleID != req.ModuleID) {
			return false
		}
		if scope.InvocationID != "" && (req.InvocationID == "" || scope.InvocationID != req.InvocationID) {
			return false
		}
		if e.checker == nil || !e.checker.InvocationOwnedBy(ctx, req.InvocationID, req.ExtensionID, req.ModuleID) {
			return false
		}
		return req.ParentSnapshot == nil || req.ParentSnapshot.InvocationID == req.InvocationID || e.checker.InvocationIsChildOf(ctx, req.InvocationID, req.ParentSnapshot.InvocationID)
	case ScopeSession:
		if scope.ExtensionID != "" && (req.ExtensionID == "" || scope.ExtensionID != req.ExtensionID) {
			return false
		}
		if scope.ModuleID != "" && (req.ModuleID == "" || scope.ModuleID != req.ModuleID) {
			return false
		}
		if scope.SessionID != "" && (req.SessionID == "" || scope.SessionID != req.SessionID) {
			return false
		}
		return e.checker != nil && e.checker.SessionValid(ctx, req.SessionID, req.ExtensionID, req.ModuleID, req.Generation)
	default:
		return false
	}
}

func (e *DefaultScopeEvaluator) buildDenialReasons(bindings []ScopeBinding, req ScopeEvaluationRequest) []ScopeReason {
	reasons := make([]ScopeReason, 0)
	for _, b := range bindings {
		switch {
		case b.State == StateExpired || b.ExpiresAt != nil && time.Now().After(*b.ExpiresAt):
			reasons = append(reasons, ScopeReason{
				Code:      ReasonBindingExpired,
				SubjectID: b.SubjectID,
			})
		case b.State == StateRevoked:
			reasons = append(reasons, ScopeReason{
				Code:      ReasonBindingRevoked,
				SubjectID: b.SubjectID,
			})
		case b.Scope.Type == ScopeCharacter && b.Scope.CharacterID != req.CharacterID:
			reasons = append(reasons, ScopeReason{
				Code:      ReasonCharacterMismatch,
				SubjectID: b.SubjectID,
			})
		case b.Scope.Type == ScopeResource:
			if b.Scope.ExtensionID != "" && req.ExtensionID != "" && b.Scope.ExtensionID != req.ExtensionID {
				reasons = append(reasons, ScopeReason{
					Code:      ReasonExtensionMismatch,
					SubjectID: b.SubjectID,
				})
			} else if b.Scope.ResourceType != "" && req.ResourceType != "" && b.Scope.ResourceType != req.ResourceType {
				reasons = append(reasons, ScopeReason{
					Code:      ReasonResourceMismatch,
					SubjectID: b.SubjectID,
				})
			}
		case b.Scope.Type == ScopeInvocation:
			if b.Scope.ExtensionID != "" && req.ExtensionID != "" && b.Scope.ExtensionID != req.ExtensionID {
				reasons = append(reasons, ScopeReason{
					Code:      ReasonExtensionMismatch,
					SubjectID: b.SubjectID,
				})
			} else if b.Scope.InvocationID != "" && req.InvocationID != "" && b.Scope.InvocationID != req.InvocationID {
				reasons = append(reasons, ScopeReason{
					Code:      ReasonInvocationExpired,
					SubjectID: b.SubjectID,
				})
			}
		case b.Scope.Type == ScopeSession:
			if b.Scope.ExtensionID != "" && req.ExtensionID != "" && b.Scope.ExtensionID != req.ExtensionID {
				reasons = append(reasons, ScopeReason{
					Code:      ReasonExtensionMismatch,
					SubjectID: b.SubjectID,
				})
			} else if b.Scope.SessionID != "" && req.SessionID != "" && b.Scope.SessionID != req.SessionID {
				reasons = append(reasons, ScopeReason{
					Code:      ReasonSessionExpired,
					SubjectID: b.SubjectID,
				})
			}
		}
	}
	return reasons
}

func (e *DefaultScopeEvaluator) checkInheritance(matched []ScopeBinding, parent ScopeSnapshot) bool {
	for _, b := range matched {
		if !parent.Contains(b.Scope) {
			return false
		}
	}
	return true
}

type ScopeRelationChecker interface {
	ConversationBelongsToCharacter(ctx context.Context, conversationID, characterID string) bool
	IsCharacterDeleted(ctx context.Context, characterID string) bool
	IsConversationDeleted(ctx context.Context, conversationID string) bool
	ResourceOwnedBy(ctx context.Context, resourceID, resourceType, extensionID, moduleID string) bool
	InvocationOwnedBy(ctx context.Context, invocationID, extensionID, moduleID string) bool
	InvocationIsChildOf(ctx context.Context, invocationID, parentInvocationID string) bool
	SessionValid(ctx context.Context, sessionID, extensionID, moduleID string, generation int64) bool
}
