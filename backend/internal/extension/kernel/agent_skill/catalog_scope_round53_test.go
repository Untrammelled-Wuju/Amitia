package agent_skill

import "testing"

func TestRound53CatalogSpaceOwnershipFailClosed(t *testing.T) {
	catalog := NewAgentSkillCatalog()
	global := AgentSkillDefinition{ID: "shared", ExtensionID: "shared", ContributionID: "contribution", Scope: AgentSkillScopeGlobal, Enabled: true}
	owned := AgentSkillDefinition{ID: "shared", ExtensionID: "shared", ContributionID: "contribution", OwnerSpaceID: "space-a", Scope: AgentSkillScopeSpace, ScopeID: "space-a", Enabled: false}
	for _, item := range []AgentSkillDefinition{global, owned} {
		if err := catalog.Register(item); err != nil {
			t.Fatal(err)
		}
	}
	if rows := catalog.List(CatalogFilter{}); len(rows) != 1 || rows[0].OwnerSpaceID != "" {
		t.Fatalf("unscoped list leaked owner-specific skill: %+v", rows)
	}
	if rows := catalog.List(CatalogFilter{SpaceID: "space-b"}); len(rows) != 1 || rows[0].OwnerSpaceID != "" {
		t.Fatalf("other space leaked skill: %+v", rows)
	}
	if rows := catalog.List(CatalogFilter{SpaceID: "space-a"}); len(rows) != 2 {
		t.Fatalf("expected global and owner skill: %+v", rows)
	}
	if got, ok := catalog.Get("shared"); !ok || got.OwnerSpaceID != "" {
		t.Fatalf("unscoped Get returned tenant skill: %+v, %v", got, ok)
	}
	for i := 0; i < 100; i++ {
		if got, ok := catalog.GetScoped("space-a", "shared"); !ok || got.OwnerSpaceID != "space-a" {
			t.Fatalf("owner-scoped lookup not preferred: %+v, %v", got, ok)
		}
		if got, ok := catalog.GetScoped("space-b", "shared"); !ok || got.OwnerSpaceID != "" {
			t.Fatalf("foreign lookup returned owner skill: %+v, %v", got, ok)
		}
	}
	if err := catalog.SetEnabled("shared", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := catalog.GetScoped("space-a", "shared"); got.Enabled {
		t.Fatal("owner override unexpectedly enabled")
	}
	if err := catalog.SetEnabledForSpace("space-a", "shared", "contribution", true); err != nil {
		t.Fatal(err)
	}
	if got, ok := catalog.GetScoped("space-a", "shared"); !ok || !got.Enabled {
		t.Fatal("owner update did not apply")
	}
	if got, ok := catalog.GetScoped("space-b", "shared"); !ok || got.Enabled {
		t.Fatal("owner update changed global skill")
	}
}

func TestRound53SkillScopeRejectsMissingOrUnknownIdentity(t *testing.T) {
	for _, test := range []struct {
		scope                          AgentSkillScope
		id                             string
		space, character, conversation string
		allowed                        bool
	}{
		{AgentSkillScopeGlobal, "", "", "", "", true},
		{AgentSkillScopeGlobal, "space-a", "space-a", "", "", false},
		{AgentSkillScopeSpace, "space-a", "space-a", "", "", true},
		{AgentSkillScopeSpace, "", "", "", "", false},
		{AgentSkillScopeCharacter, "character-a", "space-a", "character-a", "", true},
		{AgentSkillScopeCharacter, "", "space-a", "", "", false},
		{AgentSkillScopeConversation, "chat-a", "space-a", "character-a", "chat-a", true},
		{AgentSkillScopeConversation, "", "space-a", "character-a", "", false},
		{AgentSkillScope("untrusted"), "", "space-a", "character-a", "chat-a", false},
	} {
		def := AgentSkillDefinition{Scope: test.scope, ScopeID: test.id}
		if got := def.MatchesScope(test.space, test.character, test.conversation); got != test.allowed {
			t.Fatalf("scope=%q id=%q got=%v want=%v", test.scope, test.id, got, test.allowed)
		}
	}
}

func TestRound53StateIDLookupDoesNotSwitchContributions(t *testing.T) {
	catalog := NewAgentSkillCatalog()
	for _, item := range []AgentSkillDefinition{
		{ID: "alias", ExtensionID: "shared-extension", ContributionID: "skill-one", OwnerSpaceID: "space-a", Scope: AgentSkillScopeSpace, ScopeID: "space-a", Enabled: true},
		{ID: "alias", ExtensionID: "shared-extension", ContributionID: "skill-two", OwnerSpaceID: "space-a", Scope: AgentSkillScopeSpace, ScopeID: "space-a", Enabled: false},
	} {
		if err := catalog.Register(item); err != nil {
			t.Fatal(err)
		}
	}
	for _, contribution := range []string{"skill-one", "skill-two"} {
		stateID := "space-a:shared-extension:" + contribution
		got, ok := catalog.GetScopedState("space-a", stateID)
		if !ok || got.ContributionID != contribution {
			t.Fatalf("state %q resolved to %+v (%v)", stateID, got, ok)
		}
		if _, ok := catalog.GetScopedState("space-b", stateID); ok {
			t.Fatal("foreign scope resolved a private state ID")
		}
	}
}
