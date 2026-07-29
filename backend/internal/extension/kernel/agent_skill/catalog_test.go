package agent_skill

import "testing"

func truePtr() *bool { v := true; return &v }

func TestAgentSkillCatalogRegister(t *testing.T) {
	cat := NewAgentSkillCatalog()

	def := AgentSkillDefinition{
		ID:           "ext-001",
		ExtensionID:  "ext-001",
		Name:         "test-skill",
		Description:  "Test",
		Scope:        AgentSkillScopeGlobal,
		Enabled:      true,
		Instructions: SkillInstructionRef{Text: "You are a test."},
	}

	if err := cat.Register(def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	if cat.Count() != 1 {
		t.Fatalf("expected 1 skill, got %d", cat.Count())
	}

	if err := cat.Register(def); err == nil {
		t.Fatal("expected duplicate error")
	}

	retrieved, ok := cat.Get(def.ID)
	if !ok {
		t.Fatal("expected found")
	}
	if retrieved.Name != "test-skill" {
		t.Fatalf("expected name test-skill, got %s", retrieved.Name)
	}
}

func TestAgentSkillCatalogList(t *testing.T) {
	cat := NewAgentSkillCatalog()

	def1 := AgentSkillDefinition{ID: "s1", ExtensionID: "s1", Name: "one", Scope: AgentSkillScopeGlobal, Enabled: true}
	def2 := AgentSkillDefinition{ID: "s2", ExtensionID: "s2", Name: "two", Scope: AgentSkillScopeCharacter, Enabled: false}

	_ = cat.Register(def1)
	_ = cat.Register(def2)

	all := cat.List(CatalogFilter{})
	if len(all) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(all))
	}

	enabledOnly := cat.List(CatalogFilter{Enabled: truePtr()})
	if len(enabledOnly) != 1 {
		t.Fatalf("expected 1 enabled skill, got %d", len(enabledOnly))
	}
}

func TestAgentSkillCatalogUnregister(t *testing.T) {
	cat := NewAgentSkillCatalog()

	_ = cat.Register(AgentSkillDefinition{
		ID: "rm-me", ExtensionID: "rm-me", Name: "remove", Scope: AgentSkillScopeGlobal, Enabled: true,
	})

	if err := cat.Unregister("rm-me"); err != nil {
		t.Fatalf("unexpected Unregister error: %v", err)
	}

	if _, ok := cat.Get("rm-me"); ok {
		t.Fatal("expected not found")
	}
}
