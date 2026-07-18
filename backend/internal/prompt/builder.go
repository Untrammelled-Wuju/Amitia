package prompt

import (
	"github.com/u-ai/backend/config"
)

type BuildRequest struct {
	SystemPrompt string

	CharacterName       string
	CharacterConfig     string
	CompiledPersonality string
	RuntimePlan         string
	ExpressionPlan      string

	BaseIdentity     string
	PersonalityRaw   string
	EmotionFusionRaw string
	AdultIntimacyRaw string
	MemoryInjectRaw  string
	MemoryExtractRaw string
	OutputShapeRaw   string
	AntiRepeatRaw    string
	ChannelShortRaw  string

	ProactiveRaw             string
	ProactivePersonality     string
	ProactiveRelationship    string
	ProactiveEmotion         string
	ProactiveMemory          string
	ProactiveScene           string
	ProactiveTimeContext     string
	ProactiveRecentContext   string
	ProactiveTaskInstruction string

	ProfileContext            string
	TemporalContext           string
	MemoryContext             string
	Worldbook                 string
	PluginContext             string
	AgentSkillContext         string
	AgentSkillCatalogIncluded bool
	AgentSkillTrace           []AgentSkillTrace
	History                   string

	ToolResults    string
	MultimodalText string

	CurrentUserInput  string
	TraceOnly         string
	DropEmptySections bool
}

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func zeroPromptFlags() config.PromptFeatureFlags {
	return config.PromptFeatureFlags{}
}

type buildContext struct {
	req      BuildRequest
	flags    config.PromptFeatureFlags
	sections []GwSection
}

func (b *Builder) Build(req BuildRequest) GwIR {
	flags := zeroPromptFlags()
	if config.AppCfg != nil {
		flags = config.AppCfg.Prompt
	}
	ctx := buildContext{req: req, flags: flags}
	appendPolicySections(&ctx)
	appendCharacterSections(&ctx)
	appendContextSections(&ctx)
	appendSanitizerSections(&ctx)
	appendProactiveSections(&ctx)
	appendUserAndTraceSections(&ctx)
	if req.DropEmptySections {
		var filtered []GwSection
		for _, s := range ctx.sections {
			if s.Content != "" {
				filtered = append(filtered, s)
			}
		}
		ctx.sections = filtered
	}

	return GwIR{Sections: ctx.sections}
}
