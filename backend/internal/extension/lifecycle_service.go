package extension

import (
	"context"
	"sync"
)

type ExtensionLifecycleService interface {
	Enable(context.Context, string, ExecutionScope) error
	Disable(context.Context, string, ExecutionScope) error
}

type extensionLifecycleService struct {
	registry    SkillRegistry
	repository  *Repository
	agentSkills *AgentSkillService
	mu          sync.Mutex
}

func NewExtensionLifecycleService(registry SkillRegistry, repository *Repository, agentSkills *AgentSkillService) ExtensionLifecycleService {
	return &extensionLifecycleService{registry: registry, repository: repository, agentSkills: agentSkills}
}

func (s *extensionLifecycleService) Enable(ctx context.Context, extensionID string, scope ExecutionScope) error {
	return s.setEnabled(ctx, extensionID, scope, true)
}

func (s *extensionLifecycleService) Disable(ctx context.Context, extensionID string, scope ExecutionScope) error {
	return s.setEnabled(ctx, extensionID, scope, false)
}

func (s *extensionLifecycleService) setEnabled(ctx context.Context, extensionID string, scope ExecutionScope, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repository.ValidateCharacterScope(ctx, scope); err != nil {
		return err
	}
	item, err := s.registry.GetScoped(ctx, extensionID, scope)
	if err != nil {
		return err
	}
	if enabled && !item.Definition.Compatible {
		return NewExtensionError(ErrSkillIncompatible, "Skill is incompatible", item.Definition.CompatibilityReason, false, nil)
	}
	for _, dependency := range item.Definition.Dependencies {
		if _, dependencyErr := s.registry.Get(ctx, dependency); dependencyErr != nil {
			return NewExtensionError(ErrPackageDependencyMissing, "Skill dependency is missing", dependency, false, dependencyErr)
		}
	}
	if item.Definition.Source == SkillSourceInstructions && s.agentSkills != nil {
		if enabled {
			return s.agentSkills.Enable(ctx, scope, extensionID)
		}
		return s.agentSkills.Disable(ctx, scope, extensionID)
	}
	return s.registry.SetScopeEnabled(ctx, extensionID, scope, enabled)
}
