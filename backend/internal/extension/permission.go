package extension

import (
	"context"
	"sync"
)

type PermissionEvaluator interface {
	Evaluate(context.Context, ExtensionIdentity, string, PermissionScope) PermissionDecision
}

type RuntimePermissionEvaluator interface {
	PermissionEvaluator
	EvaluateExecution(context.Context, ExtensionIdentity, string, ExecutionScope) PermissionDecision
}

type DefaultPermissionEvaluator struct {
	repository   *Repository
	mu           sync.RWMutex
	systemPolicy map[string]map[string]PermissionDecision
}

func NewPermissionEvaluator(repository *Repository) *DefaultPermissionEvaluator {
	return &DefaultPermissionEvaluator{repository: repository, systemPolicy: map[string]map[string]PermissionDecision{}}
}

func (e *DefaultPermissionEvaluator) GrantSystemPolicy(skillID, capability string, decision PermissionDecision) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.systemPolicy[skillID] == nil {
		e.systemPolicy[skillID] = map[string]PermissionDecision{}
	}
	e.systemPolicy[skillID][capability] = decision
}

func (e *DefaultPermissionEvaluator) Evaluate(ctx context.Context, subject ExtensionIdentity, capability string, scope PermissionScope) PermissionDecision {
	execution := ExecutionScope{}
	switch scope.Type {
	case ScopeCharacter:
		execution.CharacterID = scope.ID
	case ScopeConversation:
		execution.ConversationID = scope.ID
	case ScopeChannel:
		execution.Channel = scope.ID
	case ScopeSession:
		execution.SessionID = scope.ID
	}
	return e.EvaluateExecution(ctx, subject, capability, execution)
}

func (e *DefaultPermissionEvaluator) EvaluateExecution(ctx context.Context, subject ExtensionIdentity, capability string, scope ExecutionScope) PermissionDecision {
	return e.evaluateExecution(ctx, subject, capability, scope, true)
}

func (e *DefaultPermissionEvaluator) PreviewExecution(ctx context.Context, subject ExtensionIdentity, capability string, scope ExecutionScope) PermissionDecision {
	return e.evaluateExecution(ctx, subject, capability, scope, false)
}

func (e *DefaultPermissionEvaluator) evaluateExecution(ctx context.Context, subject ExtensionIdentity, capability string, scope ExecutionScope, consume bool) PermissionDecision {
	if _, ok := Capability(capability); !ok {
		return DecisionDeny
	}
	if e.repository != nil {
		decision, found, err := e.repository.ResolveGrant(ctx, subject, capability, scope, consume)
		if err == nil && found {
			return decision
		}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if byCapability := e.systemPolicy[subject.SkillID]; byCapability != nil {
		if decision := byCapability[capability]; decision != "" {
			if decision == DecisionAllowSession && (scope.Trigger != TriggerLLM || scope.SessionID == "") {
				return DecisionDeny
			}
			return decision
		}
	}
	return DecisionDeny
}
