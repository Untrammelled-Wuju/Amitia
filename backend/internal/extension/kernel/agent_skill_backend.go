package kernel

import "context"

type SkillCatalogEntry struct {
	ExtensionID         string
	Name                string
	Description         string
	Scope               string
	CompatibilityStatus string
}

type SkillActivationResult struct {
	ActivationID        string
	ExtensionID         string
	Name                string
	Tokens              int
	Scope               string
	CompatibilityStatus string
	ContentHash         string
	Explicit            bool
}

type SkillActivePrompt struct {
	ActivationID        string
	ExtensionID         string
	Name                string
	Body                string
	BodyTokens          int
	Explicit            bool
	CompatibilityStatus string
	Source              string
	Scope               string
}

type AgentSkillBackend interface {
	ResolveCatalog(ctx context.Context, scope LegacyScope) ([]SkillCatalogEntry, error)
	Activate(ctx context.Context, scope LegacyScope, name string, explicit bool) (SkillActivationResult, error)
	ActivePrompts(ctx context.Context, scope LegacyScope) ([]SkillActivePrompt, error)
	EndRound(scope LegacyScope)
}
