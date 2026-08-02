package extension

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func TestLegacy_AgentSkill_RemoveChain(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nFollow the checklist in references/checklist.md."),
		"code-review/references/checklist.md": []byte("Check correctness and security."),
	}, nil)
	preview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	if err := service.Enable(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}

	activation, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review", Explicit: true})
	if err != nil || !strings.Contains(activation.Prompt, "checklist") {
		t.Fatalf("pre-remove activation failed: %+v %v", activation, err)
	}

	if _, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/checklist.md"}); err != nil {
		t.Fatalf("pre-remove resource read failed: %v", err)
	}

	if err := service.Remove(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}

	_, err = service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review"})
	assertExtensionErrorCode(t, err, ErrAgentSkillNotFound)

	_, err = service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/checklist.md"})
	assertExtensionErrorCode(t, err, ErrAgentSkillResourceDenied)

	_, _, err = service.Get(ctx, scope, installed.ExtensionID)
	assertExtensionErrorCode(t, err, ErrAgentSkillNotFound)

	page, err := service.List(ctx, scope, AgentSkillFilter{Page: 1, PageSize: 10})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("removed skill still listed: %+v %v", page.Items, err)
	}

	if err := service.Remove(ctx, scope, installed.ExtensionID); err == nil {
		t.Fatal("expected error on double remove")
	}
}

func TestLegacy_AgentSkill_PseudoSkillRegistration(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":        []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nBody."),
		"code-review/assets/icon.png": []byte("fake-png"),
	}, nil)
	preview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	if err := service.Enable(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}

	if err := registerAgentSkillRuntime(ctx, registry, service); err != nil {
		t.Fatal(err)
	}

	expectedPseudo := map[string]bool{
		"dev.amitia.skill.agent-skill-activate":       true,
		"dev.amitia.skill.agent-skill-list-resources": true,
		"dev.amitia.skill.agent-skill-read-resource":  true,
		"dev.amitia.skill.agent-skill-get-asset":      true,
	}
	allItems, err := registry.List(ctx, SkillFilter{IncludeInternal: true})
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, item := range allItems {
		if expectedPseudo[item.Definition.ID] {
			if item.Definition.Entry.Kind != "builtin" {
				t.Fatalf("%s expected kind builtin, got %s", item.Definition.ID, item.Definition.Entry.Kind)
			}
			found++
		}
	}
	if found != 4 {
		t.Fatalf("expected 4 pseudo skills, found %d", found)
	}

	executor := NewExecutor(registry, validator, nil, repository)

	activateReq := ExecuteSkillRequest{
		SkillID: "dev.amitia.skill.agent-skill-activate",
		Input:   []byte(`{"agentSkill":"code-review"}`),
		Scope:   ExecutionScope{Trigger: TriggerLLM, UserID: "user-1", CharacterID: "char-1"},
	}
	_, err = executor.Execute(ctx, activateReq)
	if err != nil {
		t.Fatalf("activate pseudo skill execution failed: %v", err)
	}

	listReq := ExecuteSkillRequest{
		SkillID: "dev.amitia.skill.agent-skill-list-resources",
		Input:   []byte(`{"agentSkill":"code-review"}`),
		Scope:   ExecutionScope{Trigger: TriggerLLM, UserID: "user-1", CharacterID: "char-1"},
	}
	_, err = executor.Execute(ctx, listReq)
	if err != nil {
		t.Fatalf("list-resources pseudo skill execution failed: %v", err)
	}

	readReq := ExecuteSkillRequest{
		SkillID: "dev.amitia.skill.agent-skill-read-resource",
		Input:   []byte(`{"agentSkill":"code-review","path":"SKILL.md"}`),
		Scope:   ExecutionScope{Trigger: TriggerLLM, UserID: "user-1", CharacterID: "char-1"},
	}
	_, err = executor.Execute(ctx, readReq)
	if err != nil {
		t.Fatalf("read-resource pseudo skill execution failed: %v", err)
	}

	getReq := ExecuteSkillRequest{
		SkillID: "dev.amitia.skill.agent-skill-get-asset",
		Input:   []byte(`{"agentSkill":"code-review","path":"assets/icon.png"}`),
		Scope:   ExecutionScope{Trigger: TriggerLLM, UserID: "user-1", CharacterID: "char-1"},
	}
	_, err = executor.Execute(ctx, getReq)
	if err != nil {
		t.Fatalf("get-asset pseudo skill execution failed: %v", err)
	}

	instructionsID := installed.ExtensionID
	instructions, err := registry.Get(ctx, instructionsID)
	if err != nil {
		t.Fatal(err)
	}
	if instructions.Definition.Entry.Kind != "instructions" {
		t.Fatalf("Agent Skill instructions expected kind instructions, got %s", instructions.Definition.Entry.Kind)
	}

	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: instructionsID, Input: []byte(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillNotExecutable)

	instructions, err = registry.Get(ctx, instructionsID)
	if err != nil {
		t.Fatal(err)
	}
	if instructions.Handler != nil {
		t.Fatalf("Agent Skill instructions must not have a handler (KNOWN_LEGACY_BEHAVIOR: instructions registered with nil handler)")
	}
}

func TestLegacy_AgentSkill_ResourceReadEdges(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":            []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nSee references/guide.md"),
		"code-review/references/guide.md": []byte("Checklist content here."),
		"code-review/assets/info.txt":     []byte("info"),
		"code-review/scripts/check.py":    []byte("print('not executed')"),
	}, nil)
	preview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	if err := service.Enable(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review", Explicit: true}); err != nil {
		t.Fatal(err)
	}

	content, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/guide.md"})
	if err != nil || content.Executable || content.Kind != AgentSkillResourceReference {
		t.Fatalf("reference resource read failed: %+v %v", content, err)
	}

	_, err = service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/nonexistent.md"})
	assertExtensionErrorCode(t, err, ErrAgentSkillResourceNotFound)

	script, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "scripts/check.py"})
	if err != nil || script.Kind != AgentSkillResourceScript || script.Executable {
		t.Fatalf("script resource read failed: %+v %v", script, err)
	}

	asset, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "assets/info.txt"})
	if err != nil || asset.Kind != AgentSkillResourceAsset {
		t.Fatalf("asset resource read failed: %+v %v", asset, err)
	}

	resources, err := service.ListResources(ctx, ListAgentSkillResourcesRequest{Scope: scope, NameOrID: "code-review"})
	if err != nil || len(resources) != 4 {
		t.Fatalf("list resources failed: %d %v", len(resources), err)
	}

	service.EndRound(scope)
	_, err = service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/guide.md"})
	assertExtensionErrorCode(t, err, ErrAgentSkillResourceDenied)
}

func TestLegacy_AgentSkill_ActivationEdges(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	skillA := agentSkillBaselineZIP(t, map[string][]byte{
		"skill-a/SKILL.md": []byte("---\nname: skill-a\ndescription: Skill A for testing. Use when A is requested.\n---\n\nBody A."),
	}, nil)
	skillB := agentSkillBaselineZIP(t, map[string][]byte{
		"skill-b/SKILL.md": []byte("---\nname: skill-b\ndescription: Skill B for testing. Use when B is requested.\n---\n\nBody B."),
	}, nil)

	for _, s := range []struct {
		name string
		raw  []byte
	}{
		{"skill-a", skillA},
		{"skill-b", skillB},
	} {
		preview, err := service.PreviewZIP(ctx, "user-1", s.raw)
		if err != nil {
			t.Fatal(err)
		}
		installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
		if err != nil {
			t.Fatal(err)
		}
		scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
		if err := service.Enable(ctx, scope, installed.ExtensionID); err != nil {
			t.Fatal(err)
		}
	}

	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}

	a1, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "skill-a", Explicit: true})
	if err != nil || a1.Definition.Description != "Skill A for testing. Use when A is requested." {
		t.Fatalf("first activation failed: %+v %v", a1, err)
	}

	a2, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "skill-b", Explicit: true})
	if err != nil || a2.Definition.Description != "Skill B for testing. Use when B is requested." {
		t.Fatalf("second activation failed: %+v %v", a2, err)
	}

	if a1.ActivationID == a2.ActivationID {
		t.Fatal("multiple skills must produce distinct activations")
	}

	a1Dup, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "skill-a"})
	if err != nil || a1Dup.ActivationID != a1.ActivationID {
		t.Fatalf("duplicate activation not deduplicated: %+v %v", a1Dup, err)
	}

	catalog, err := service.ResolveCatalog(ctx, scope)
	if err != nil || len(catalog) != 2 {
		t.Fatalf("catalog after dual activation: %+v %v", catalog, err)
	}

	if _, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "nonexistent"}); err == nil {
		t.Fatal("expected error activating nonexistent skill")
	}

	service.EndRound(scope)

	round2, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "skill-a", Explicit: true})
	if err != nil {
		t.Fatalf("activation after EndRound should start a new round (KNOWN_LEGACY_BEHAVIOR: EndRound resets state): %v", err)
	}
	if round2.ActivationID == a1.ActivationID {
		t.Fatal("activation after EndRound must produce a new activation ID")
	}
}

func TestLegacy_AgentSkill_ConcurrentResourceRead(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":            []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nRead references/guide.md"),
		"code-review/references/guide.md": []byte("Concurrent read test content."),
	}, nil)
	preview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	if err := service.Enable(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review", Explicit: true}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/guide.md"})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("concurrent resource read failed: %v", e)
	}
}

func TestLegacy_AgentSkill_DeleteDoesNotAffectSharedNameCopy(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md": []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nBody."),
	}, nil)

	preview1, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed1, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview1.PreviewID, Scope: AgentSkillScopeCharacter, CharacterID: "char-1"})
	if err != nil {
		t.Fatal(err)
	}

	preview2, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed2, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview2.PreviewID, Scope: AgentSkillScopeCharacter, CharacterID: "char-2"})
	if err != nil {
		t.Fatal(err)
	}

	scope1 := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	scope2 := ExecutionScope{UserID: "user-1", CharacterID: "char-2", ConversationID: "conv-2", Channel: "web", Trigger: TriggerLLM}

	if err := service.Enable(ctx, scope1, installed1.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := service.Enable(ctx, scope2, installed2.ExtensionID); err != nil {
		t.Fatal(err)
	}

	if err := service.Remove(ctx, scope1, installed1.ExtensionID); err != nil {
		t.Fatal(err)
	}

	_, _, err = service.Get(ctx, scope1, installed1.ExtensionID)
	assertExtensionErrorCode(t, err, ErrAgentSkillNotFound)

	_, _, err = service.Get(ctx, scope2, installed2.ExtensionID)
	assertExtensionErrorCode(t, err, ErrAgentSkillNotFound)
}

func TestLegacy_AgentSkill_ActivationTrace(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nRead references/guide.md and references/checklist.md"),
		"code-review/references/guide.md":     []byte("Guide content."),
		"code-review/references/checklist.md": []byte("Checklist content."),
	}, nil)
	preview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	if err := service.Enable(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review", Explicit: true}); err != nil {
		t.Fatal(err)
	}

	service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/guide.md"})
	service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/checklist.md"})

	records, err := repository.ListAgentSkillActivations(ctx, installed.ExtensionID, "user-1", 10)
	if err != nil || len(records) != 1 || len(records[0].ResourcePaths) != 2 {
		t.Fatalf("activation trace incomplete: %+v %v", records, err)
	}
}

func TestLegacy_AgentSkill_EmptyBody(t *testing.T) {
	limits := DefaultAgentSkillLimits()
	_, err := parseAgentSkillFiles(map[string][]byte{
		"SKILL.md": []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\n"),
	}, "code-review", AgentSkillSourceDirectory, limits)
	assertExtensionErrorCode(t, err, ErrAgentSkillFrontmatter)
}

func TestLegacy_AgentSkill_LargeResource(t *testing.T) {
	limits := DefaultAgentSkillLimits()
	limits.MaxTextResourceBytes = 10
	limits.MaxResourceBytes = 10
	files := map[string][]byte{
		"SKILL.md":            []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nSee guide."),
		"references/guide.md": bytesRepeat(20, 'a'),
	}
	_, err := parseAgentSkillFiles(files, "code-review", AgentSkillSourceDirectory, limits)
	assertExtensionErrorCode(t, err, ErrAgentSkillArchiveLimit)
}

func TestLegacy_AgentSkill_NameConflictGlobalAndCharacter(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	rawGlobal := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md": []byte("---\nname: code-review\ndescription: Global instance. Use when users request an audit.\n---\n\nGlobal."),
	}, nil)
	previewG, err := service.PreviewZIP(ctx, "user-1", rawGlobal)
	if err != nil {
		t.Fatal(err)
	}
	installedG, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: previewG.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", Trigger: TriggerLLM}
	if err := service.Enable(ctx, scope, installedG.ExtensionID); err != nil {
		t.Fatal(err)
	}

	rawChar := rawGlobal
	previewC, err := service.PreviewZIP(ctx, "user-1", rawChar)
	if err != nil {
		t.Fatal(err)
	}
	installedC, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: previewC.PreviewID, Scope: AgentSkillScopeCharacter, CharacterID: "char-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Enable(ctx, scope, installedC.ExtensionID); err != nil {
		t.Fatal(err)
	}

	catalog, err := service.ResolveCatalog(ctx, scope)
	if err != nil || len(catalog) != 1 || catalog[0].ExtensionID != installedC.ExtensionID {
		t.Fatalf("character-scoped skill should shadow global: %+v %v", catalog, err)
	}
}

func agentSkillBaselineDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := migration.Runner{DB: db, SkipBackup: true}
	migs := []migration.Migration{
		migration.ExtensionsMigration(),
		migration.ExtensionScopeBindingsMigration(),
		migration.ExtensionWorkshopMigration(),
		migration.ExtensionAgentSkillsMigration(),
		migration.ExtensionAgentSkillTraceMigration(),
	}
	if err := runner.Apply(migs); err != nil {
		t.Fatal(err)
	}
	return db
}

func agentSkillBaselineZIP(t *testing.T, entries map[string][]byte, mode *uint32) []byte {
	t.Helper()
	return agentSkillTestZIP(t, entries, mode)
}

func bytesRepeat(n int, b byte) []byte {
	result := make([]byte, n)
	for i := range result {
		result[i] = b
	}
	return result
}
