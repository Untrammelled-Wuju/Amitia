package kernel

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
)

func TestRound53InstalledSkillResourceRejectsRevokedAndRoleScoped(t *testing.T) {
	catalog := agent_skill.NewAgentSkillCatalog()
	for _, def := range []agent_skill.AgentSkillDefinition{
		{ID: "revoked", ExtensionID: "revoked", ContributionID: "revoked", OwnerSpaceID: "space-a", Scope: agent_skill.AgentSkillScopeSpace, ScopeID: "space-a", Enabled: false},
		{ID: "role-only", ExtensionID: "role-only", ContributionID: "role-only", OwnerSpaceID: "space-a", Scope: agent_skill.AgentSkillScopeCharacter, ScopeID: "character-a", Enabled: true},
		{ID: "other-space", ExtensionID: "other-space", ContributionID: "other-space", OwnerSpaceID: "space-b", Scope: agent_skill.AgentSkillScopeSpace, ScopeID: "space-b", Enabled: true},
	} {
		if err := catalog.Register(def); err != nil {
			t.Fatal(err)
		}
	}
	runtime := NewAgentSkillRuntime(catalog, nil, "")
	for _, skillID := range []string{"revoked", "role-only", "other-space"} {
		_, _, err := runtime.ReadInstalledResource(context.Background(), "space-a", skillID, "reference.md")
		if err == nil || !strings.Contains(err.Error(), "denied") && skillID != "other-space" {
			t.Fatalf("skill %q authorization error: %v", skillID, err)
		}
	}
}
