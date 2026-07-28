package kernel

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
)

var explicitSkillPattern = regexp.MustCompile(`(?:^|[\s])\$([a-z0-9]+(?:-[a-z0-9]+)*)(?:\b|$)`)

func (f *ToolFacade) buildAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	if f.agentSkillCatalog == nil {
		return "", nil, nil
	}

	agentScope := agent_skill.AgentSkillScopeGlobal
	if scope.CharacterID != "" {
		agentScope = agent_skill.AgentSkillScopeCharacter
	}

	allSkills := f.agentSkillCatalog.List(agent_skill.CatalogFilter{
		Scope:   agentScope,
		Enabled: boolPtr(true),
	})

	globalSkills := f.agentSkillCatalog.List(agent_skill.CatalogFilter{
		Scope:   agent_skill.AgentSkillScopeGlobal,
		Enabled: boolPtr(true),
	})

	allSkills = append(allSkills, globalSkills...)

	if len(allSkills) == 0 {
		return "", nil, nil
	}

	seen := make(map[string]bool)
	unique := make([]agent_skill.AgentSkillDefinition, 0, len(allSkills))
	for _, s := range allSkills {
		if seen[s.ExtensionID] {
			continue
		}
		seen[s.ExtensionID] = true
		unique = append(unique, s)
	}

	errorsList := []string{}
	activated := []LegacyActivatedSkill{}

	explicitNames := parseExplicitSkillNames(message)
	for _, name := range explicitNames {
		found := false
		for _, skill := range unique {
			if strings.EqualFold(skill.Name, name) || strings.EqualFold(skill.ExtensionID, name) {
				activated = append(activated, skillToLegacyActivated(skill, true))
				found = true
				break
			}
		}
		if !found {
			errorsList = append(errorsList, fmt.Sprintf("agent skill '%s' not found", name))
		}
	}

	if f.activationService != nil {
		autoCandidates := f.activationService.EvaluateAuto(ctx, message, agentScope, scope.CharacterID)
		for _, c := range autoCandidates {
			if c.MatchType != "keyword" {
				continue
			}
			alreadyActivated := false
			for _, a := range activated {
				if a.Name == c.Definition.Name {
					alreadyActivated = true
					break
				}
			}
			if !alreadyActivated {
				activated = append(activated, skillToLegacyActivated(c.Definition, false))
			}
		}
	}

	catalog := renderSkillCatalog(unique)
	return catalog, activated, errorsList
}

func parseExplicitSkillNames(message string) []string {
	matches := explicitSkillPattern.FindAllStringSubmatch(message, -1)
	seen := map[string]bool{}
	result := []string{}
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			result = append(result, match[1])
		}
	}
	return result
}

func renderSkillCatalog(skills []agent_skill.AgentSkillDefinition) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# Available Agent Skills\n\n")
	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**", s.Name))
		if s.DisplayName != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", s.DisplayName))
		}
		sb.WriteString(fmt.Sprintf(": %s\n", s.Description))
		if len(s.Activation.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf("  Keywords: %s\n", strings.Join(s.Activation.Keywords, ", ")))
		}
	}
	sb.WriteString("\nTo activate a skill explicitly, use $skill-name in your message.\n")
	return sb.String()
}

func skillToLegacyActivated(def agent_skill.AgentSkillDefinition, explicit bool) LegacyActivatedSkill {
	prompt := def.Instructions.Text
	return LegacyActivatedSkill{
		ActivationID:        fmt.Sprintf("kernel-%s", def.ExtensionID),
		ExtensionID:         def.ExtensionID,
		Name:                def.Name,
		Source:              def.Source,
		Scope:               string(def.Scope),
		CompatibilityStatus: def.Compatibility.Status,
		Prompt:              prompt,
		BodyTokens:          def.Instructions.TokenCount,
		Explicit:            explicit,
	}
}
