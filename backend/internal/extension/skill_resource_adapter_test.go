// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package extension

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel"
)

func TestSkillResourceAdapter_HasResourceCapableSkills_NoActiveSkills(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)
	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")

	scope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}
	has, names, err := adapter.HasResourceCapableSkills(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatal("expected no resource capable skills")
	}
	if len(names) != 0 {
		t.Fatalf("expected empty names, got %v", names)
	}
}

func TestSkillResourceAdapter_HasResourceCapableSkills_WithActiveSkills(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code.\n---\n\nFollow the checklist."),
		"code-review/references/checklist.md": []byte("Check correctness and security."),
		"code-review/assets/icon.png":         []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}
	has, names, err := adapter.HasResourceCapableSkills(ctx, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatal("expected resource capable skills")
	}
	found := false
	for _, name := range names {
		if name == "code-review" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected code-review in names, got %v", names)
	}
}

func TestSkillResourceAdapter_ListSkillResources(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/references/checklist.md": []byte("Check correctness."),
		"code-review/references/guide.md":     []byte("Guide content."),
		"code-review/assets/icon.png":         []byte{0x89, 0x50, 0x4E, 0x47},
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleListSkillResources(ctx, kernel.ListSkillResourcesInput{Skill: "code-review"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error != "" {
		t.Fatalf("unexpected error in output: %s", output.Error)
	}
	if output.TotalCount < 3 {
		t.Fatalf("expected at least 3 resources, got %d", output.TotalCount)
	}
	if len(output.Resources) < 3 {
		t.Fatalf("expected at least 3 resource descriptors, got %d", len(output.Resources))
	}
}

func TestSkillResourceAdapter_ListSkillResources_WithKindFilter(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/references/checklist.md": []byte("Check correctness."),
		"code-review/assets/icon.png":         []byte{0x89, 0x50, 0x4E, 0x47},
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleListSkillResources(ctx, kernel.ListSkillResourcesInput{Skill: "code-review", Kind: "reference"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.TotalCount != 1 {
		t.Fatalf("expected 1 reference, got %d", output.TotalCount)
	}
	if len(output.Resources) != 1 || !strings.HasPrefix(output.Resources[0].RelativePath, "references/") {
		t.Fatalf("expected reference resource, got %v", output.Resources)
	}
}

func TestSkillResourceAdapter_ListSkillResources_WithPrefixFilter(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/references/checklist.md": []byte("Check correctness."),
		"code-review/assets/icon.png":         []byte{0x89, 0x50, 0x4E, 0x47},
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleListSkillResources(ctx, kernel.ListSkillResourcesInput{Skill: "code-review", Prefix: "assets/"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.TotalCount != 1 {
		t.Fatalf("expected 1 asset, got %d", output.TotalCount)
	}
	if len(output.Resources) != 1 || !strings.HasPrefix(output.Resources[0].RelativePath, "assets/") {
		t.Fatalf("expected asset resource, got %v", output.Resources)
	}
}

func TestSkillResourceAdapter_ListSkillResources_InactiveSkill(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", Channel: "web"}

	output, err := adapter.HandleListSkillResources(ctx, kernel.ListSkillResourcesInput{Skill: "nonexistent"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error == "" {
		t.Fatal("expected error for inactive skill")
	}
}

func TestSkillResourceAdapter_ReadSkillResource(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/references/checklist.md": []byte("Line 1\nLine 2\nLine 3\nLine 4\nLine 5"),
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleReadSkillResource(ctx, kernel.ReadSkillResourceInput{Skill: "code-review", Path: "references/checklist.md"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error != "" {
		t.Fatalf("unexpected error in output: %s", output.Error)
	}
	if output.StartLine != 1 {
		t.Fatalf("expected start line 1, got %d", output.StartLine)
	}
	if output.TotalLines < 5 {
		t.Fatalf("expected at least 5 total lines, got %d", output.TotalLines)
	}
	if !strings.Contains(output.Content, "Line 1") {
		t.Fatalf("expected content to contain Line 1, got %s", output.Content)
	}
	if output.Truncated {
		t.Fatal("expected no truncation for small content")
	}
}

func TestSkillResourceAdapter_ReadSkillResource_Pagination(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	lines := make([]string, 0, 250)
	for i := 0; i < 250; i++ {
		lines = append(lines, "line content")
	}
	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":                []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/references/checklist.md": []byte(strings.Join(lines, "\n")),
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleReadSkillResource(ctx, kernel.ReadSkillResourceInput{Skill: "code-review", Path: "references/checklist.md", MaxLines: 100}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error != "" {
		t.Fatalf("unexpected error: %s", output.Error)
	}
	if output.TotalLines < 250 {
		t.Fatalf("expected at least 250 total lines, got %d", output.TotalLines)
	}
	if !output.Truncated {
		t.Fatal("expected truncation with maxLines=100")
	}
	if output.NextStartLine != 101 {
		t.Fatalf("expected next start line 101, got %d", output.NextStartLine)
	}
}

func TestSkillResourceAdapter_ReadSkillResource_InactiveSkill(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", Channel: "web"}

	output, err := adapter.HandleReadSkillResource(ctx, kernel.ReadSkillResourceInput{Skill: "nonexistent", Path: "references/checklist.md"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error == "" {
		t.Fatal("expected error for inactive skill")
	}
}

func TestSkillResourceAdapter_MaterializeSkillResource(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":        []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/assets/icon.png": []byte{0x89, 0x50, 0x4E, 0x47},
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleMaterializeSkillResource(ctx, kernel.MaterializeSkillResourceInput{Skill: "code-review", Path: "assets/icon.png"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error != "" {
		t.Fatalf("unexpected error: %s", output.Error)
	}
	if output.ResourceURI == "" {
		t.Fatal("expected non-empty resource URI")
	}
	if output.Skill != "code-review" {
		t.Fatalf("expected skill code-review, got %s", output.Skill)
	}
	if output.Path != "assets/icon.png" {
		t.Fatalf("expected path assets/icon.png, got %s", output.Path)
	}
	if output.LeaseID == "" {
		t.Fatal("expected non-empty lease ID")
	}
}

func TestSkillResourceAdapter_MaterializeSkillResource_InactiveSkill(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", Channel: "web"}

	output, err := adapter.HandleMaterializeSkillResource(ctx, kernel.MaterializeSkillResourceInput{Skill: "nonexistent", Path: "assets/icon.png"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error == "" {
		t.Fatal("expected error for inactive skill")
	}
}

func TestSkillResourceAdapter_MaterializeSkillResource_PathTraversal(t *testing.T) {
	db := agentSkillBaselineDB(t)
	ctx := context.Background()
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	service := NewAgentSkillService(repository, registry, validator)

	raw := agentSkillBaselineZIP(t, map[string][]byte{
		"code-review/SKILL.md":        []byte("---\nname: code-review\ndescription: Review code.\n---\n\nBody."),
		"code-review/assets/icon.png": []byte{0x89, 0x50, 0x4E, 0x47},
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

	adapter := NewSkillResourceAdapter(service, "http://localhost:18899")
	legacyScope := kernel.LegacyScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"}

	output, err := adapter.HandleMaterializeSkillResource(ctx, kernel.MaterializeSkillResourceInput{Skill: "code-review", Path: "../secret"}, legacyScope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Error == "" {
		t.Fatal("expected error for path traversal attempt")
	}
}
