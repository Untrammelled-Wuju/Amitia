package scope

import (
	"context"
	"fmt"
	"sync"
)

type ScopeBindRequest struct {
	SubjectType ScopeSubjectType   `json:"subjectType"`
	SubjectID   string            `json:"subjectId"`
	Scope       ScopeRef          `json:"scope"`
	Source      ScopeBindingSource `json:"source"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

type ScopeBindingFilter struct {
	SubjectType ScopeSubjectType `json:"subjectType,omitempty"`
	SubjectID   string          `json:"subjectId,omitempty"`
	ScopeType   ScopeType       `json:"scopeType,omitempty"`
	State       ScopeBindingState `json:"state,omitempty"`
}

type ScopeInvalidationFilter struct {
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	ExtensionID    string `json:"extensionId,omitempty"`
	ModuleID       string `json:"moduleId,omitempty"`
	SubjectType    ScopeSubjectType `json:"subjectType,omitempty"`
	SubjectID      string          `json:"subjectId,omitempty"`
}

type ScopeManager interface {
	Bind(ctx context.Context, req ScopeBindRequest) (ScopeBinding, error)
	Unbind(ctx context.Context, bindingID string) error
	Evaluate(ctx context.Context, req ScopeEvaluationRequest) ScopeDecision
	Snapshot(ctx context.Context, request ScopeResolveRequest) (ScopeSnapshot, error)
	Invalidate(ctx context.Context, filter ScopeInvalidationFilter) error
	ListBindings(ctx context.Context, filter ScopeBindingFilter) ([]ScopeBinding, error)
}

type DefaultScopeManager struct {
	mu        sync.RWMutex
	store     ScopeStore
	evaluator ScopeEvaluator
	cache     *ScopeCache
	auditor   ScopeAuditor
}

func NewScopeManager(store ScopeStore, evaluator ScopeEvaluator) *DefaultScopeManager {
	return &DefaultScopeManager{
		store:     store,
		evaluator: evaluator,
		cache:     NewScopeCache(),
	}
}

func (m *DefaultScopeManager) SetAuditor(auditor ScopeAuditor) {
	m.auditor = auditor
}

func (m *DefaultScopeManager) Bind(ctx context.Context, req ScopeBindRequest) (ScopeBinding, error) {
	binding, err := NewBinding(req.SubjectType, req.SubjectID, req.Scope, req.Source)
	if err != nil {
		return ScopeBinding{}, fmt.Errorf("create binding: %w", err)
	}
	if req.Metadata != nil {
		binding.Metadata = req.Metadata
	}

	if err := m.store.SaveBinding(ctx, *binding); err != nil {
		return ScopeBinding{}, fmt.Errorf("save binding: %w", err)
	}

	m.cache.InvalidateSubject(req.SubjectType, req.SubjectID)

	if m.auditor != nil {
		m.auditor.RecordBindingCreated(ctx, *binding)
	}

	return *binding, nil
}

func (m *DefaultScopeManager) Unbind(ctx context.Context, bindingID string) error {
	binding, err := m.store.GetBinding(ctx, bindingID)
	if err != nil {
		return fmt.Errorf("get binding: %w", err)
	}

	if err := m.store.DeleteBinding(ctx, bindingID); err != nil {
		return fmt.Errorf("delete binding: %w", err)
	}

	m.cache.InvalidateSubject(binding.SubjectType, binding.SubjectID)

	if m.auditor != nil {
		m.auditor.RecordBindingDeleted(ctx, binding)
	}

	return nil
}

func (m *DefaultScopeManager) Evaluate(ctx context.Context, req ScopeEvaluationRequest) ScopeDecision {
	if req.ParentSnapshot == nil {
		key := cacheKey(req.SubjectType, req.SubjectID, req.CharacterID, req.ConversationID)
		if cached, ok := m.cache.Get(key); ok {
			return cached
		}
		decision := m.evaluator.Evaluate(ctx, req)
		m.cache.Set(key, decision)
		return decision
	}

	return m.evaluator.Evaluate(ctx, req)
}

func (m *DefaultScopeManager) Snapshot(ctx context.Context, req ScopeResolveRequest) (ScopeSnapshot, error) {
	scopes, err := ResolveScope(ctx, req)
	if err != nil {
		return ScopeSnapshot{}, fmt.Errorf("resolve scope: %w", err)
	}
	return CreateSnapshot(req.InvocationID, scopes, req.CharacterID, req.ConversationID, req.ExtensionID, req.ModuleID), nil
}

func (m *DefaultScopeManager) Invalidate(ctx context.Context, filter ScopeInvalidationFilter) error {
	bindings, err := m.store.ListBindings(ctx, ScopeBindingFilter{
		SubjectType: filter.SubjectType,
		SubjectID:   filter.SubjectID,
	})
	if err != nil {
		return fmt.Errorf("list bindings: %w", err)
	}

	for _, b := range bindings {
		if shouldInvalidate(b, filter) {
			b.State = StateRevoked
			if err := m.store.SaveBinding(ctx, b); err != nil {
				return fmt.Errorf("invalidate binding: %w", err)
			}
			if m.auditor != nil {
				m.auditor.RecordBindingRevoked(ctx, b)
			}
		}
	}

	m.cache.Clear()
	return nil
}

func shouldInvalidate(b ScopeBinding, filter ScopeInvalidationFilter) bool {
	if filter.CharacterID != "" && b.Scope.Type == ScopeCharacter && b.Scope.CharacterID == filter.CharacterID {
		return true
	}
	if filter.ConversationID != "" && b.Scope.Type == ScopeConversation && b.Scope.ConversationID == filter.ConversationID {
		return true
	}
	if filter.ExtensionID != "" && b.Scope.Type == ScopeExtension && b.Scope.ExtensionID == filter.ExtensionID {
		return true
	}
	if filter.ModuleID != "" && b.Scope.Type == ScopeModule && b.Scope.ModuleID == filter.ModuleID {
		return true
	}
	return false
}

func (m *DefaultScopeManager) ListBindings(ctx context.Context, filter ScopeBindingFilter) ([]ScopeBinding, error) {
	return m.store.ListBindings(ctx, filter)
}

func cacheKey(subjectType ScopeSubjectType, subjectID, characterID, conversationID string) string {
	return fmt.Sprintf("%s/%s/%s/%s", subjectType, subjectID, characterID, conversationID)
}
