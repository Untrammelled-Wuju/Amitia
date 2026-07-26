package migration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestLegacySkillToTool(t *testing.T) {
	def := extension.SkillDefinition{
		ID:             "dev.amitia.skill.get-current-time",
		ModelName:      "get_current_time",
		Name:           "get_current_time",
		Description:    "Get current time",
		Version:        "1.0.0",
		Source:         extension.SkillSourceLegacy,
		InputSchema:    json.RawMessage(`{"type":"object"}`),
		OutputSchema:   json.RawMessage(`{"type":"object"}`),
		Capabilities:   []string{"runtime.time.read"},
		HasSideEffects: false,
		Idempotent:     true,
		Enabled:        true,
		Compatible:     true,
		TimeoutMS:      5000,
	}

	result := LegacySkillToTool(def)

	if result.Source != capability.ToolSourceLegacy {
		t.Fatalf("expected source legacy_tool, got %s", result.Source)
	}
	if result.Name != "get_current_time" {
		t.Fatalf("expected name get_current_time, got %s", result.Name)
	}
	if result.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if !result.Idempotent {
		t.Fatal("expected idempotent true")
	}
	if result.HasSideEffects {
		t.Fatal("expected HasSideEffects false")
	}
	if len(result.Permissions) != 1 {
		t.Fatalf("expected 1 permission requirement, got %d", len(result.Permissions))
	}
}

func TestBuiltinSkillToTool(t *testing.T) {
	def := extension.SkillDefinition{
		ID:          "dev.amitia.skill.agent-skill-activate",
		ModelName:   "agent_skill_activate",
		Name:        "agent_skill_activate",
		Description: "Activate Agent Skill",
		Version:     "1.0.0",
		Source:      extension.SkillSourceBuiltin,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Internal:    true,
		Enabled:     true,
		Compatible:  true,
		TimeoutMS:   5000,
	}

	result := BuiltinSkillToTool(def)

	if result.Source != capability.ToolSourceBuiltin {
		t.Fatalf("expected source builtin, got %s", result.Source)
	}
	if !result.Internal {
		t.Fatal("expected internal true")
	}
	if !result.Enabled {
		t.Fatal("expected enabled true")
	}
}

func TestInternalSkillToTool(t *testing.T) {
	def := extension.SkillDefinition{
		ID:          "dev.amitia.skill.agent-skill-activate",
		ModelName:   "agent_skill_activate",
		Name:        "agent_skill_activate",
		Description: "Activate Agent Skill",
		Version:     "1.0.0",
		Source:      extension.SkillSourceBuiltin,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Internal:    true,
		Enabled:     true,
		TimeoutMS:   5000,
	}

	result := InternalSkillToTool(def)

	if result.Source != capability.ToolSourceInternal {
		t.Fatalf("expected source internal, got %s", result.Source)
	}
	if !result.Internal {
		t.Fatal("expected internal true")
	}
	if result.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if !strings.Contains(result.ID, "internal/") {
		t.Fatalf("expected ID to contain internal/, got %s", result.ID)
	}
}

func TestPluginSkillToTool(t *testing.T) {
	def := extension.SkillDefinition{
		ID:             "com.example.weather.query",
		ModelName:      "query_weather",
		Name:           "query_weather",
		Description:    "Query weather",
		Version:        "1.0.0",
		Source:         extension.SkillSourceWorkflow,
		InputSchema:    json.RawMessage(`{"type":"object"}`),
		OutputSchema:   json.RawMessage(`{"type":"object"}`),
		Capabilities:   []string{"external.account.read"},
		HasSideEffects: false,
		Idempotent:     true,
		Enabled:        true,
		Compatible:     true,
		TimeoutMS:      10000,
	}

	pluginID := "com.example.weather"
	result := PluginSkillToTool(def, pluginID)

	if result.Source != capability.ToolSourcePlugin {
		t.Fatalf("expected source plugin, got %s", result.Source)
	}
	if result.ExtensionID != pluginID {
		t.Fatalf("expected ExtensionID %s, got %s", pluginID, result.ExtensionID)
	}
	if !strings.Contains(result.ID, pluginID) {
		t.Fatalf("expected ID to contain plugin ID, got %s", result.ID)
	}
}

func TestMCPSkillToTool(t *testing.T) {
	def := extension.SkillDefinition{
		ID:             "mcp.server-uuid.tool-search",
		ModelName:      "mcp_server-uuid_search",
		Name:           "search",
		Description:    "Search documents",
		Version:        "1.0.0",
		Source:         extension.SkillSourceMCP,
		InputSchema:    json.RawMessage(`{"type":"object"}`),
		OutputSchema:   json.RawMessage(`{"type":"object"}`),
		Capabilities:   []string{"mcp.invoke", "mcp.server.server-uuid", "mcp.tool.server-uuid.search", "external.account.read"},
		HasSideEffects: false,
		Idempotent:     true,
		Enabled:        true,
		Compatible:     true,
		TimeoutMS:      30000,
	}

	serverID := "server-uuid"
	result := MCPSkillToTool(def, serverID)

	if result.Source != capability.ToolSourceMCP {
		t.Fatalf("expected source mcp, got %s", result.Source)
	}
	if result.Scope.Type != "mcp_server" {
		t.Fatalf("expected scope type mcp_server, got %s", result.Scope.Type)
	}
	if result.Scope.ID != serverID {
		t.Fatalf("expected scope ID %s, got %s", serverID, result.Scope.ID)
	}
	if !strings.Contains(result.ID, "mcp/") {
		t.Fatalf("expected ID to contain mcp/, got %s", result.ID)
	}
	if result.RiskLevel != capability.RiskLow {
		t.Fatalf("expected RiskLevel low, got %s", result.RiskLevel)
	}
}

func TestBuildToolIDStability(t *testing.T) {
	id1 := capability.BuildToolID(capability.ToolSourceLegacy, "amitia", "get-current-time")
	id2 := capability.BuildToolID(capability.ToolSourceLegacy, "amitia", "get-current-time")

	if id1 != id2 {
		t.Fatalf("expected stable ID, got %s vs %s", id1, id2)
	}

	expected := "legacy_tool/amitia/get-current-time"
	if id1 != expected {
		t.Fatalf("expected %s, got %s", expected, id1)
	}
}

func TestAgentSkillToDefinition(t *testing.T) {
	old := extension.AgentSkillDefinition{
		ExtensionID:         "ext-001",
		Name:                "test-skill",
		Description:         "A test skill",
		DisplayName:         "Test Skill",
		Body:                "You are a test assistant.",
		Scope:               extension.AgentSkillScopeGlobal,
		Enabled:             true,
		Source:              extension.AgentSkillSourceBundled,
		Compatibility:       "1.0.0",
		License:             "MIT",
		CompatibilityStatus: extension.AgentSkillCompatible,
		Metadata:            map[string]string{"key": "value"},
		Resources: []extension.AgentSkillResource{
			{Path: "ref/doc.md", Kind: extension.AgentSkillResourceReference, MIMEType: "text/markdown", Size: 100, TextReadable: true},
		},
	}

	result := AgentSkillToDefinition(old)

	if result.ID != "ext-001" {
		t.Fatalf("expected ID ext-001, got %s", result.ID)
	}
	if result.Name != "test-skill" {
		t.Fatalf("expected name test-skill, got %s", result.Name)
	}
	if result.Instructions.Text != "You are a test assistant." {
		t.Fatalf("expected instructions match, got %s", result.Instructions.Text)
	}
	if !result.Enabled {
		t.Fatal("expected enabled true")
	}
	if !result.Compatible {
		t.Fatal("expected compatible true")
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.TokenPolicy.MaxInstructionTokens == 0 {
		t.Fatal("expected non-zero token budget")
	}
}
