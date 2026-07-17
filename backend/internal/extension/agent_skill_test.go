package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func TestAgentSkillParserValidation(t *testing.T) {
	limits := DefaultAgentSkillLimits()
	tests := []struct {
		name  string
		skill string
		root  string
		code  string
	}{
		{"minimal", "---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\n# Review\nFollow the checklist.", "code-review", ""},
		{"bom", "\ufeff---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nBody", "code-review", ""},
		{"missing-frontmatter", "# Review", "code-review", ErrAgentSkillFrontmatter},
		{"missing-name", "---\ndescription: Review code. Use when requested.\n---\n\nBody", "code-review", ErrAgentSkillNameInvalid},
		{"missing-description", "---\nname: code-review\n---\n\nBody", "code-review", ErrAgentSkillDescription},
		{"invalid-name", "---\nname: Code_Review\ndescription: Review code. Use when requested.\n---\n\nBody", "Code_Review", ErrAgentSkillNameInvalid},
		{"mismatch", "---\nname: code-review\ndescription: Review code. Use when requested.\n---\n\nBody", "other", ErrAgentSkillNameMismatch},
		{"empty-body", "---\nname: code-review\ndescription: Review code. Use when requested.\n---\n\n", "code-review", ErrAgentSkillFrontmatter},
		{"duplicate-key", "---\nname: code-review\nname: duplicate\ndescription: Review code. Use when requested.\n---\n\nBody", "code-review", ErrAgentSkillFrontmatter},
		{"alias", "---\nname: code-review\ndescription: &value Review code. Use when requested.\nlicense: *value\n---\n\nBody", "code-review", ErrAgentSkillFrontmatter},
		{"tag", "---\nname: code-review\ndescription: !unsafe Review code. Use when requested.\n---\n\nBody", "code-review", ErrAgentSkillFrontmatter},
		{"multiple-documents", "---\nname: code-review\ndescription: Review code. Use when requested.\n--- # second\nother: value\n---\n\nBody", "code-review", ErrAgentSkillFrontmatter},
		{"metadata-object", "---\nname: code-review\ndescription: Review code. Use when requested.\nmetadata:\n  nested:\n    value: invalid\n---\n\nBody", "code-review", ErrAgentSkillFrontmatter},
		{"long-description", "---\nname: code-review\ndescription: " + strings.Repeat("a", 1025) + "\n---\n\nBody", "code-review", ErrAgentSkillDescription},
		{"long-compatibility", "---\nname: code-review\ndescription: Review code. Use when requested.\ncompatibility: " + strings.Repeat("a", 501) + "\n---\n\nBody", "code-review", ErrAgentSkillFrontmatter},
		{"secret", "---\nname: code-review\ndescription: Review code. Use when requested.\n---\n\napi_key=abcdefgh12345678", "code-review", ErrAgentSkillArtifactInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseAgentSkillFiles(map[string][]byte{"SKILL.md": []byte(test.skill)}, test.root, AgentSkillSourceDirectory, limits)
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			assertExtensionErrorCode(t, err, test.code)
		})
	}
}

func TestAgentSkillDirectoryLimitsAndUnsafeSVG(t *testing.T) {
	service := NewAgentSkillService(nil, nil, nil)
	service.limits.MaxFiles = 1
	_, err := service.PreviewDirectory(context.Background(), "user-1", "code-review", map[string][]byte{"SKILL.md": []byte("x"), "guide.md": []byte("x")})
	assertExtensionErrorCode(t, err, ErrAgentSkillArchiveLimit)
	service.limits = DefaultAgentSkillLimits()
	_, err = service.PreviewDirectory(context.Background(), "user-1", "code-review", map[string][]byte{"SKILL.md": []byte("x"), "Guide.md": []byte("x"), "guide.md": []byte("x")})
	assertExtensionErrorCode(t, err, ErrAgentSkillInvalidArchive)
	files := map[string][]byte{
		"SKILL.md":        []byte("---\nname: code-review\ndescription: Review code. Use when requested.\n---\n\nUse the icon."),
		"assets/icon.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
	}
	parsed, err := parseAgentSkillFiles(files, "code-review", AgentSkillSourceDirectory, DefaultAgentSkillLimits())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Report.Status != AgentSkillBlocked {
		t.Fatalf("unsafe SVG was not blocked: %+v", parsed.Report)
	}
}

func TestAgentSkillParserResourcesMappingsAndOpenAI(t *testing.T) {
	files := map[string][]byte{
		"SKILL.md":            []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\nallowed-tools: Read WebSearch Bash(git:*) MCP(github)\nunknown-field: preserved\n---\n\nSee [guide](references/guide.md). Run scripts/check.py."),
		"references/guide.md": []byte("Checklist"), "assets/template.txt": []byte("Template"), "scripts/check.py": []byte("print('never executed')"),
		"agents/openai.yaml": []byte("interface:\n  display_name: \"Code Review\"\n  short_description: \"Review safely\"\n  brand_color: \"#3B82F6\"\n  icon_small: \"./assets/template.txt\"\n  default_prompt: \"Use $code-review\"\ndependencies:\n  tools:\n    - type: mcp\n      value: github\n"),
	}
	parsed, err := parseAgentSkillFiles(files, "code-review", AgentSkillSourceDirectory, DefaultAgentSkillLimits())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Definition.DisplayName != "Code Review" || len(parsed.Definition.Resources) != 5 {
		t.Fatalf("unexpected parsed definition: %+v", parsed.Definition)
	}
	if parsed.Report.Status != AgentSkillPartiallyCompatible || len(parsed.Report.RequiredScripts) != 1 {
		t.Fatalf("unexpected report: %+v", parsed.Report)
	}
	statuses := map[string]string{}
	for _, mapping := range parsed.Definition.ToolMappings {
		statuses[mapping.SourceTool] = mapping.Status
	}
	if statuses["Read"] != "mapped" || statuses["Bash(git:*)"] != "blocked" || statuses["MCP(github)"] != "unsupported" {
		t.Fatalf("unexpected mappings: %+v", statuses)
	}
	for _, resource := range parsed.Definition.Resources {
		if resource.Executable {
			t.Fatalf("resource became executable: %s", resource.Path)
		}
	}
}

func TestAgentSkillZIPSecurity(t *testing.T) {
	limits := DefaultAgentSkillLimits()
	valid := agentSkillTestZIP(t, map[string][]byte{"code-review/SKILL.md": []byte("---\nname: code-review\ndescription: Review code. Use when requested.\n---\n\nBody")}, nil)
	files, root, err := readAgentSkillZIP(valid, limits)
	if err != nil || root != "code-review" || len(files) != 1 {
		t.Fatalf("valid ZIP failed: %s %v", root, err)
	}
	cases := []struct {
		name, path string
		mode       *uint32
		code       string
	}{{"traversal", "code-review/../evil.txt", nil, ErrAgentSkillPathTraversal}, {"absolute", "/code-review/SKILL.md", nil, ErrAgentSkillPathTraversal}, {"drive", "C:/code-review/SKILL.md", nil, ErrAgentSkillPathTraversal}, {"multiple-roots", "one/SKILL.md", nil, ErrAgentSkillInvalidArchive}}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			entries := map[string][]byte{test.path: []byte("x")}
			if test.name == "multiple-roots" {
				entries["two/file.txt"] = []byte("x")
			}
			raw := agentSkillTestZIP(t, entries, test.mode)
			_, _, err := readAgentSkillZIP(raw, limits)
			assertExtensionErrorCode(t, err, test.code)
		})
	}
	mode := uint32(os.ModeSymlink | 0777)
	raw := agentSkillTestZIP(t, map[string][]byte{"code-review/SKILL.md": []byte("target")}, &mode)
	_, _, err = readAgentSkillZIP(raw, limits)
	assertExtensionErrorCode(t, err, ErrAgentSkillInvalidArchive)
	bomb := agentSkillTestZIP(t, map[string][]byte{"code-review/SKILL.md": bytes.Repeat([]byte("a"), 20000)}, nil)
	_, _, err = readAgentSkillZIP(bomb, limits)
	assertExtensionErrorCode(t, err, ErrAgentSkillArchiveLimit)
	collision := agentSkillTestZIP(t, map[string][]byte{"code-review/SKILL.md": []byte("x"), "code-review/references/é.md": []byte("x"), "code-review/references/e\u0301.md": []byte("x")}, nil)
	_, _, err = readAgentSkillZIP(collision, limits)
	assertExtensionErrorCode(t, err, ErrAgentSkillInvalidArchive)
}

func TestInstructionsRegistryAndExecutorContract(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, nil)
	definition := AgentSkillDefinition{ExtensionID: "local.agentskill.scope.code-review", Name: "code-review", Description: "Review code. Use when requested.", ArtifactID: "artifact", ContentHash: strings.Repeat("a", 64), CompatibilityStatus: AgentSkillCompatible}
	skill := buildAgentSkillManifest(definition, "0.0.0+aaaaaaaaaaaa")
	if err := registry.Register(context.Background(), skill, nil); err != nil {
		t.Fatal(err)
	}
	available, err := registry.Available(context.Background(), ExecutionScope{Trigger: TriggerLLM})
	if err != nil || len(available) != 0 {
		t.Fatalf("instructions exposed as model tool: %v %v", available, err)
	}
	executor := NewExecutor(registry, validator, nil, nil)
	_, err = executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: skill.ID, Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillNotExecutable)
}

func TestAgentSkillInstallActivateResourceAndRestore(t *testing.T) {
	db := agentSkillTestDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)
	raw := agentSkillTestZIP(t, map[string][]byte{"code-review/SKILL.md": []byte("---\nname: code-review\ndescription: Review code. Use when users request an audit.\n---\n\nRead references/checklist.md and report findings."), "code-review/references/checklist.md": []byte("Check correctness and security.")}, nil)
	preview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: preview.PreviewID, Scope: AgentSkillScopeGlobal})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Enabled {
		t.Fatal("Agent Skill must be disabled by default")
	}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", TraceID: "trace-1", Trigger: TriggerLLM}
	if err := service.Enable(ctx, ExecutionScope{UserID: "user-1"}, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	catalog, err := service.ResolveCatalog(ctx, scope)
	if err != nil || len(catalog) != 1 {
		t.Fatalf("catalog: %+v %v", catalog, err)
	}
	characterPreview, err := service.PreviewZIP(ctx, "user-1", raw)
	if err != nil {
		t.Fatal(err)
	}
	characterSkill, err := service.Install(ctx, InstallAgentSkillRequest{UserID: "user-1", PreviewID: characterPreview.PreviewID, Scope: AgentSkillScopeCharacter, CharacterID: "char-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Enable(ctx, scope, characterSkill.ExtensionID); err != nil {
		t.Fatal(err)
	}
	catalog, err = service.ResolveCatalog(ctx, scope)
	if err != nil || len(catalog) != 1 || catalog[0].ExtensionID != characterSkill.ExtensionID {
		t.Fatalf("character priority failed: %+v %v", catalog, err)
	}
	globalView, _, err := service.Get(ctx, ExecutionScope{UserID: "user-1", CharacterID: "char-2"}, characterSkill.ExtensionID)
	if err != nil || !globalView.Enabled || globalView.Scope != AgentSkillScopeGlobal {
		t.Fatalf("global binding not inherited: %+v %v", globalView, err)
	}
	if err := service.Disable(ctx, scope, characterSkill.ExtensionID); err != nil {
		t.Fatal(err)
	}
	catalog, err = service.ResolveCatalog(ctx, scope)
	if err != nil || len(catalog) != 0 {
		t.Fatalf("character disable did not override global binding: %+v %v", catalog, err)
	}
	if err := service.Enable(ctx, scope, characterSkill.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveCatalog(ctx, scope); err != nil {
		t.Fatal(err)
	}
	activation, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review", Explicit: true})
	if err != nil || !strings.Contains(activation.Prompt, "active_agent_skill") {
		t.Fatalf("activation failed: %+v %v", activation, err)
	}
	if len(service.artifacts) == 0 || len(service.catalogs) == 0 {
		t.Fatalf("Agent Skill caches were not populated")
	}
	content, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/checklist.md"})
	if err != nil || content.Executable || !strings.Contains(content.Content, "correctness") {
		t.Fatalf("resource read failed: %+v %v", content, err)
	}
	records, err := repository.ListAgentSkillActivations(ctx, installed.ExtensionID, "user-1", 10)
	if err != nil || len(records) != 1 || records[0].ResourceReads != 1 || len(records[0].ResourcePaths) != 1 {
		t.Fatalf("activation resource trace not updated: %+v %v", records, err)
	}
	service.limits.MaxActivations = 1
	duplicate, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review"})
	if err != nil || duplicate.ActivationID != activation.ActivationID {
		t.Fatalf("duplicate activation was not deduplicated: %+v %v", duplicate, err)
	}
	_, err = service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "../secret"})
	assertExtensionErrorCode(t, err, ErrAgentSkillPathTraversal)
	service.EndRound(scope)
	service.limits.MaxActivations = 0
	_, err = service.Activate(ctx, ActivateAgentSkillRequest{Scope: scope, NameOrID: "code-review"})
	assertExtensionErrorCode(t, err, ErrAgentSkillActivationLimit)
	service.EndRound(scope)
	_, err = service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: scope, NameOrID: "code-review", Path: "references/checklist.md"})
	assertExtensionErrorCode(t, err, ErrAgentSkillResourceDenied)
	restoredRegistry := NewRegistry("1.0.0", validator, repository)
	restored := NewAgentSkillService(repository, restoredRegistry, validator)
	if err := restored.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	item, err := restoredRegistry.Get(ctx, installed.ExtensionID)
	if err != nil || !item.Definition.Enabled {
		t.Fatalf("restore failed: %+v %v", item, err)
	}
}

func agentSkillTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]migration.Migration{migration.ExtensionsMigration(), migration.ExtensionWorkshopMigration(), migration.ExtensionAgentSkillsMigration(), migration.ExtensionAgentSkillTraceMigration()}); err != nil {
		t.Fatal(err)
	}
	return db
}
func agentSkillTestZIP(t *testing.T, entries map[string][]byte, mode *uint32) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if mode != nil {
			header.SetMode(os.FileMode(*mode))
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func assertExtensionErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	var extErr *ExtensionError
	if !errors.As(err, &extErr) || extErr.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
