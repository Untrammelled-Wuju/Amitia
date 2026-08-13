package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockAgentSkillBackend struct {
	catalog      []SkillCatalogEntry
	activateFunc func(scope LegacyScope, name string) (SkillActivationResult, error)
	activePrompt []SkillActivePrompt
	activeLabels []string
	endRoundHit  int
}

func (m *mockAgentSkillBackend) ResolveCatalog(ctx context.Context, scope LegacyScope) ([]SkillCatalogEntry, error) {
	return m.catalog, nil
}

func (m *mockAgentSkillBackend) Activate(ctx context.Context, scope LegacyScope, name string, explicit bool) (SkillActivationResult, error) {
	if m.activateFunc != nil {
		return m.activateFunc(scope, name)
	}
	return SkillActivationResult{ActivationID: "act-" + name, ExtensionID: "ext-" + name, Name: name, Tokens: 100, Explicit: explicit}, nil
}

func (m *mockAgentSkillBackend) ActivePrompts(ctx context.Context, scope LegacyScope) ([]SkillActivePrompt, error) {
	return m.activePrompt, nil
}

func (m *mockAgentSkillBackend) EndRound(scope LegacyScope) {
	m.endRoundHit++
}

func TestBuildActivateSkillTool_HasDynamicEnum(t *testing.T) {
	names := []string{"pdf", "xlsx", "pptx"}
	tool := buildActivateSkillTool(names)

	if tool.Function.Name != ActivateSkillToolName {
		t.Fatalf("expected tool name %q, got %q", ActivateSkillToolName, tool.Function.Name)
	}

	prop, ok := tool.Function.Parameters.Properties["name"]
	if !ok {
		t.Fatal("expected 'name' property in parameters")
	}

	if len(prop.Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(prop.Enum))
	}

	for i, name := range names {
		if prop.Enum[i] != name {
			t.Fatalf("expected enum[%d]=%q, got %q", i, name, prop.Enum[i])
		}
	}
}

func TestBuildActivateSkillTool_EmptyNames(t *testing.T) {
	tool := buildActivateSkillTool(nil)
	if tool.Function.Name != ActivateSkillToolName {
		t.Fatalf("expected tool name %q, got %q", ActivateSkillToolName, tool.Function.Name)
	}
}

func TestModelTools_InjectsActivateSkillWhenCatalogNonEmpty(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	facade := &ToolFacade{
		toolRegistry: toolRegistry,
		agentSkillBackend: &mockAgentSkillBackend{
			catalog: []SkillCatalogEntry{
				{Name: "pdf", Description: "PDF processing"},
			},
		},
		counters: NewToolFacadeCounters(),
	}

	tools, err := facade.ModelTools(context.Background(), LegacyScope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tool := range tools {
		if tool.Function.Name == ActivateSkillToolName {
			found = true
			prop := tool.Function.Parameters.Properties["name"]
			if len(prop.Enum) != 1 || prop.Enum[0] != "pdf" {
				t.Fatalf("expected enum=[pdf], got %v", prop.Enum)
			}
			break
		}
	}

	if !found {
		t.Fatal("expected activate_skill tool to be injected")
	}
}

func TestModelTools_NoActivateSkillWhenCatalogEmpty(t *testing.T) {
	facade := &ToolFacade{
		agentSkillBackend: &mockAgentSkillBackend{
			catalog: nil,
		},
		counters: NewToolFacadeCounters(),
	}

	tools, err := facade.ModelTools(context.Background(), LegacyScope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tool := range tools {
		if tool.Function.Name == ActivateSkillToolName {
			t.Fatal("activate_skill should NOT be injected when catalog is empty")
		}
	}
}

func TestModelTools_NoActivateSkillWithoutBackend(t *testing.T) {
	facade := &ToolFacade{
		counters: NewToolFacadeCounters(),
	}

	tools, err := facade.ModelTools(context.Background(), LegacyScope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}
}

func TestExecuteModelTool_ActivateSkillSuccess(t *testing.T) {
	facade := &ToolFacade{
		agentSkillBackend: &mockAgentSkillBackend{
			catalog: []SkillCatalogEntry{{Name: "pdf", Description: "PDF"}},
			activateFunc: func(scope LegacyScope, name string) (SkillActivationResult, error) {
				return SkillActivationResult{
					ActivationID:        "act-123",
					ExtensionID:         "ext-456",
					Name:                name,
					Tokens:              500,
					Scope:               "global",
					CompatibilityStatus: "compatible",
					ContentHash:         "sha256:abc",
					Explicit:            true,
				}, nil
			},
		},
		counters: NewToolFacadeCounters(),
	}

	input := json.RawMessage(`{"name":"pdf"}`)
	result, found := facade.ExecuteModelTool(context.Background(), ActivateSkillToolName, input, LegacyScope{}, "idem-1")

	if !found {
		t.Fatal("expected found=true for activate_skill")
	}

	if result.Status != "SUCCESS" {
		t.Fatalf("expected status SUCCESS, got %q", result.Status)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if output["activationId"] != "act-123" {
		t.Fatalf("expected activationId=act-123, got %v", output["activationId"])
	}
	if output["name"] != "pdf" {
		t.Fatalf("expected name=pdf, got %v", output["name"])
	}
	if output["status"] != "activated" {
		t.Fatalf("expected status=activated, got %v", output["status"])
	}
}

func TestExecuteModelTool_ActivateSkillFailure(t *testing.T) {
	facade := &ToolFacade{
		agentSkillBackend: &mockAgentSkillBackend{
			catalog: []SkillCatalogEntry{{Name: "pdf", Description: "PDF"}},
			activateFunc: func(scope LegacyScope, name string) (SkillActivationResult, error) {
				return SkillActivationResult{}, &mockError{text: "skill not found"}
			},
		},
		counters: NewToolFacadeCounters(),
	}

	input := json.RawMessage(`{"name":"unknown"}`)
	result, found := facade.ExecuteModelTool(context.Background(), ActivateSkillToolName, input, LegacyScope{}, "idem-1")

	if !found {
		t.Fatal("expected found=true even on activation failure")
	}

	if result.Status != "FAILED" {
		t.Fatalf("expected status FAILED, got %q", result.Status)
	}

	if result.Error == nil || result.Error.Code != "ACTIVATION_FAILED" {
		t.Fatalf("expected ACTIVATION_FAILED error, got %v", result.Error)
	}
}

func TestExecuteModelTool_ActivateSkillInvalidInput(t *testing.T) {
	facade := &ToolFacade{
		agentSkillBackend: &mockAgentSkillBackend{
			catalog: []SkillCatalogEntry{{Name: "pdf", Description: "PDF"}},
		},
		counters: NewToolFacadeCounters(),
	}

	input := json.RawMessage(`{"bad_field":"value"}`)
	result, found := facade.ExecuteModelTool(context.Background(), ActivateSkillToolName, input, LegacyScope{}, "idem-1")

	if !found {
		t.Fatal("expected found=true even on invalid input")
	}

	if result.Status != "FAILED" {
		t.Fatalf("expected status FAILED, got %q", result.Status)
	}

	if result.Error == nil || result.Error.Code != "INVALID_INPUT" {
		t.Fatalf("expected INVALID_INPUT error, got %v", result.Error)
	}
}

func TestExecuteModelTool_ActivateSkillNoBackend(t *testing.T) {
	facade := &ToolFacade{
		counters: NewToolFacadeCounters(),
	}

	input := json.RawMessage(`{"name":"pdf"}`)
	_, found := facade.ExecuteModelTool(context.Background(), ActivateSkillToolName, input, LegacyScope{}, "idem-1")

	if found {
		t.Fatal("expected found=false when no backend configured")
	}
}

func TestPrepareAgentSkillPrompt_FromBackendExplicitActivation(t *testing.T) {
	mock := &mockAgentSkillBackend{
		catalog: []SkillCatalogEntry{
			{Name: "pdf", Description: "PDF processing"},
			{Name: "xlsx", Description: "Excel processing"},
		},
		activePrompt: []SkillActivePrompt{
			{
				ActivationID: "act-pdf",
				ExtensionID:  "ext-pdf",
				Name:         "pdf",
				Body:         "<active_agent_skill>PDF body</active_agent_skill>",
				BodyTokens:   500,
				Explicit:     true,
			},
		},
	}

	facade := &ToolFacade{
		agentSkillBackend: mock,
		counters:          NewToolFacadeCounters(),
	}

	scope := LegacyScope{UserID: "user1", ConversationID: "conv1"}
	catalog, activated, errs := facade.PrepareAgentSkillPrompt(context.Background(), scope, "Please use $pdf for this task")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if catalog == "" {
		t.Fatal("expected non-empty catalog")
	}

	if len(activated) != 1 {
		t.Fatalf("expected 1 activated skill, got %d", len(activated))
	}

	if activated[0].Name != "pdf" {
		t.Fatalf("expected activated skill name=pdf, got %q", activated[0].Name)
	}

	if !activated[0].Explicit {
		t.Fatal("expected explicit activation")
	}
}

func TestPrepareAgentSkillPrompt_FromBackendNoExplicit(t *testing.T) {
	mock := &mockAgentSkillBackend{
		catalog: []SkillCatalogEntry{
			{Name: "pdf", Description: "PDF processing"},
		},
	}

	facade := &ToolFacade{
		agentSkillBackend: mock,
		counters:          NewToolFacadeCounters(),
	}

	scope := LegacyScope{UserID: "user1"}
	catalog, activated, errs := facade.PrepareAgentSkillPrompt(context.Background(), scope, "Just a normal message")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if catalog == "" {
		t.Fatal("expected non-empty catalog")
	}

	if len(activated) != 0 {
		t.Fatalf("expected 0 activated skills, got %d", len(activated))
	}
}

func TestEndAgentSkillRound_CallsBackend(t *testing.T) {
	mock := &mockAgentSkillBackend{}
	facade := &ToolFacade{
		agentSkillBackend: mock,
		counters:          NewToolFacadeCounters(),
	}

	scope := LegacyScope{UserID: "user1"}
	facade.EndAgentSkillRound(scope)

	if mock.endRoundHit != 1 {
		t.Fatalf("expected EndRound to be called once, got %d", mock.endRoundHit)
	}
}

func TestResolveVisibleSkillNames(t *testing.T) {
	facade := &ToolFacade{
		agentSkillBackend: &mockAgentSkillBackend{
			catalog: []SkillCatalogEntry{
				{Name: "pdf", Description: "PDF"},
				{Name: "xlsx", Description: "Excel"},
				{Name: "pptx", Description: "PowerPoint"},
			},
		},
		counters: NewToolFacadeCounters(),
	}

	names, err := facade.resolveVisibleSkillNames(context.Background(), LegacyScope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	expected := map[string]bool{"pdf": true, "xlsx": true, "pptx": true}
	for _, name := range names {
		if !expected[name] {
			t.Fatalf("unexpected name: %q", name)
		}
	}
}

type mockError struct {
	text string
}

func (e *mockError) Error() string {
	return e.text
}
