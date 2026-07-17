package prompt

import (
	"strings"
	"testing"
)

func TestAgentSkillPromptPlacementAndTrace(t *testing.T) {
	gateway := NewGateway()
	messages, trace, err := gateway.BuildMessages(BuildRequest{
		BaseIdentity:              "base",
		CharacterConfig:           "character",
		AgentSkillContext:         "<active_agent_skill>instruction</active_agent_skill>",
		AgentSkillCatalogIncluded: true,
		AgentSkillTrace: []AgentSkillTrace{{
			ActivationID: "activation-1", Name: "code-review", Explicit: true, BodyTokens: 10, ScriptsUsed: false, InstructionPosition: "after_character_rules", Status: "activated",
		}},
		MemoryContext:    "memory",
		CurrentUserInput: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) < 3 || messages[0].Role != "system" || !strings.Contains(messages[0].Content, "<active_agent_skill>instruction</active_agent_skill>") || !strings.Contains(messages[len(messages)-2].Content, "memory") || messages[len(messages)-1].Role != "user" {
		t.Fatalf("unexpected Agent Skill prompt placement: %+v", messages)
	}
	if trace == nil || !trace.AgentSkillCatalogIncluded || !trace.QualityFlags.AgentSkillSectionUsed || len(trace.AgentSkills) != 1 || trace.AgentSkills[0].ScriptsUsed {
		t.Fatalf("unexpected Agent Skill trace: %+v", trace)
	}
}
