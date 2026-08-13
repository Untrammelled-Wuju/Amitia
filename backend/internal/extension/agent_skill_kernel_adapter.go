package extension

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel"
)

type agentSkillServiceAdapter struct {
	service *AgentSkillService
}

func NewAgentSkillKernelAdapter(service *AgentSkillService) kernel.AgentSkillBackend {
	return &agentSkillServiceAdapter{service: service}
}

func (a *agentSkillServiceAdapter) ResolveCatalog(ctx context.Context, scope kernel.LegacyScope) ([]kernel.SkillCatalogEntry, error) {
	extScope := legacyScopeToKernel(scope)
	catalog, err := a.service.ResolveCatalog(ctx, extScope)
	if err != nil {
		return nil, err
	}
	result := make([]kernel.SkillCatalogEntry, 0, len(catalog))
	for _, item := range catalog {
		result = append(result, kernel.SkillCatalogEntry{
			ExtensionID:         item.ExtensionID,
			Name:                item.Name,
			Description:         item.Description,
			Scope:               string(item.Scope),
			CompatibilityStatus: string(item.Compatibility),
		})
	}
	return result, nil
}

func (a *agentSkillServiceAdapter) Activate(ctx context.Context, scope kernel.LegacyScope, name string, explicit bool) (kernel.SkillActivationResult, error) {
	extScope := legacyScopeToKernel(scope)
	activated, err := a.service.Activate(ctx, ActivateAgentSkillRequest{
		Scope:    extScope,
		NameOrID: name,
		Explicit: explicit,
	})
	if err != nil {
		return kernel.SkillActivationResult{}, err
	}
	return kernel.SkillActivationResult{
		ActivationID:        activated.ActivationID,
		ExtensionID:         activated.Definition.ExtensionID,
		Name:                activated.Definition.Name,
		Tokens:              activated.BodyTokens,
		Scope:               string(activated.Definition.Scope),
		CompatibilityStatus: string(activated.Definition.CompatibilityStatus),
		ContentHash:         activated.Definition.ContentHash,
		Explicit:            activated.Explicit,
	}, nil
}

func (a *agentSkillServiceAdapter) ActivePrompts(ctx context.Context, scope kernel.LegacyScope) ([]kernel.SkillActivePrompt, error) {
	extScope := legacyScopeToKernel(scope)
	_, activatedSkills, _ := a.service.PreparePrompt(ctx, extScope, "")
	result := make([]kernel.SkillActivePrompt, 0, len(activatedSkills))
	for _, item := range activatedSkills {
		result = append(result, kernel.SkillActivePrompt{
			ActivationID:        item.ActivationID,
			ExtensionID:         item.Definition.ExtensionID,
			Name:                item.Definition.Name,
			Body:                item.Prompt,
			BodyTokens:          item.BodyTokens,
			Explicit:            item.Explicit,
			CompatibilityStatus: string(item.Definition.CompatibilityStatus),
			Source:              string(item.Definition.Source),
			Scope:               string(item.Definition.Scope),
		})
	}
	return result, nil
}

func (a *agentSkillServiceAdapter) EndRound(scope kernel.LegacyScope) {
	a.service.EndRound(legacyScopeToKernel(scope))
}

func legacyScopeToKernel(scope kernel.LegacyScope) ExecutionScope {
	return ExecutionScope{
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		TraceID:        scope.TraceID,
		RequestID:      scope.RequestID,
		ToolCallID:     scope.ToolCallID,
		CorrelationID:  scope.CorrelationID,
		CausationID:    scope.CausationID,
	}
}
