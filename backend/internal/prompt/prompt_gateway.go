package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Gateway struct {
	Builder   *Builder
	Renderer  *Renderer
	Validator *Validator
}

func NewGateway() *Gateway {
	return &Gateway{
		Builder:   NewBuilder(),
		Renderer:  NewRenderer(),
		Validator: NewValidator(),
	}
}

func (g *Gateway) BuildMessages(req BuildRequest) ([]GwMessage, *PromptTrace, error) {
	ir := g.Builder.Build(req)

	if err := g.Validator.ValidateIR(ir); err != nil {
		return nil, nil, err
	}

	messages, err := g.Renderer.Render(ir)
	if err != nil {
		return nil, nil, err
	}

	if err := g.Validator.ValidateMessages(messages); err != nil {
		return nil, nil, err
	}

	trace := computePromptTrace(ir, messages)
	trace.AgentSkillCatalogIncluded = req.AgentSkillCatalogIncluded
	trace.AgentSkills = append([]AgentSkillTrace(nil), req.AgentSkillTrace...)

	return messages, trace, nil
}

func computePromptTrace(ir GwIR, messages []GwMessage) *PromptTrace {
	var fullPrompt strings.Builder
	for _, m := range messages {
		fullPrompt.WriteString(m.Role)
		fullPrompt.WriteString(": ")
		fullPrompt.WriteString(m.Content)
		fullPrompt.WriteString("\n")
	}
	hash := sha256.Sum256([]byte(fullPrompt.String()))
	promptHash := hex.EncodeToString(hash[:])

	trace := &PromptTrace{
		PromptHash: promptHash,
		QualityFlags: QualityFlags{
			PersonaSectionUsed:    sectionUsed(ir.Sections, "personality_raw"),
			EmotionSectionUsed:    sectionUsed(ir.Sections, "emotion_fusion_raw"),
			MemorySectionUsed:     sectionUsed(ir.Sections, "memory_inject_raw"),
			AgentSkillSectionUsed: sectionUsed(ir.Sections, "agent_skill_instructions"),
			IntimacyBoundaryUsed:  sectionUsed(ir.Sections, "adult_intimacy_raw"),
		},
	}

	for _, s := range ir.Sections {
		if s.Content == "" {
			continue
		}
		st := SectionTrace{
			SectionName:    s.ID,
			SourceProject:  s.SourceProject,
			SourceFile:     s.SourceFile,
			SourceConstant: s.SourceConstant,
			PromptHash:     promptHash,
			RenderedLength: len(s.Content),
			Enabled:        s.Enabled,
		}
		trace.Sections = append(trace.Sections, st)
	}

	return trace
}

func SetTraceQualityFlags(trace *PromptTrace, flags QualityFlags) {
	if trace == nil {
		return
	}
	trace.QualityFlags = flags
}

func sectionUsed(sections []GwSection, id string) bool {
	for _, s := range sections {
		if s.ID == id && s.Enabled && s.Content != "" {
			return true
		}
	}
	return false
}
