package extension

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
)

func TestLegacy_Tool_GenerateBaselineSnapshot(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, nil)
	ids, err := adapter.RegisterAll(context.Background(), registry)
	if err != nil {
		t.Fatal(err)
	}
	allTools := tool.GetAll()
	memoryTools := tool.GetMemoryTools()
	totalToolCount := len(allTools) + len(memoryTools)
	t.Logf("BUILTIN_TOOL_COUNT: %d (regular: %d, memory: %d)", totalToolCount, len(allTools), len(memoryTools))
	if len(ids) != totalToolCount {
		t.Fatalf("registered %d tools but expected %d", len(ids), totalToolCount)
	}
	for _, id := range ids {
		registered, err := registry.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if registered.Definition.Source != SkillSourceLegacy && registered.Definition.Source != "" {
			continue
		}
		t.Logf("TOOL: id=%s modelName=%s desc=%s inputHash=%s capabilities=%v sideEffects=%v idempotent=%v",
			registered.Definition.ID,
			registered.Definition.ModelName,
			truncate(registered.Definition.Description, 60),
			hashRaw(registered.Definition.InputSchema),
			registered.Definition.Capabilities,
			registered.Definition.HasSideEffects,
			registered.Definition.Idempotent,
		)
	}
	toolIDs := make([]string, len(ids))
	copy(toolIDs, ids)
	sort.Strings(toolIDs)
	for _, id := range toolIDs {
		t.Logf("TOOL_ID: %s", id)
	}
}

func TestLegacy_Tool_DuplicateRegistrationRejected(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, nil)
	allTools := tool.GetAll()
	if len(allTools) == 0 {
		t.Skip("no legacy tools registered")
	}
	first := allTools[0]
	definition, handler, err := adapter.Adapt(first, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	err = registry.Register(context.Background(), definition, handler)
	if err == nil {
		t.Fatal("expected duplicate registration to be rejected")
	}
	if asExtensionError(err).Code != ErrSkillDuplicateID {
		t.Fatalf("expected ErrSkillDuplicateID, got %v", asExtensionError(err).Code)
	}
}

func TestLegacy_Tool_NotFoundHandler(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	missing := tool.Tool{Type: "function", Function: tool.Function{Name: "nonexistent_legacy_tool", Description: "not found", Parameters: tool.Parameters{Type: "object", Properties: map[string]tool.Property{}, Required: []string{}}}}
	definition, handler, err := adapter.Adapt(missing, false)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ID != "dev.amitia.skill.nonexistent-legacy-tool" {
		t.Fatalf("unexpected ID: %s", definition.ID)
	}
	result, err := handler(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	if asExtensionError(err).Code != ErrSkillExecutionFailed {
		t.Fatalf("expected ErrSkillExecutionFailed, got %v", asExtensionError(err).Code)
	}
	if result.Status != "" {
		t.Fatalf("expected empty result for failed execution, got status=%s", result.Status)
	}
}

func TestLegacy_Tool_HandlerCancelledContext(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	allTools := tool.GetAll()
	if len(allTools) == 0 {
		t.Skip("no legacy tools registered")
	}
	first := allTools[0]
	_, handler, err := adapter.Adapt(first, false)
	if err != nil {
		t.Fatal(err)
	}
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := handler(cancelledCtx, ExecuteSkillRequest{Input: json.RawMessage(`{}`)})
	if err != nil {
		t.Logf("KNOWN_LEGACY_BEHAVIOR: handler returned error for cancelled context: %v", err)
		return
	}
	t.Log("KNOWN_LEGACY_BEHAVIOR: legacy tool handler does not propagate context cancellation")
	if result.Status != "" {
		t.Logf("result status for cancelled execution: %s", result.Status)
	}
}

func TestLegacy_Tool_MemoryToolCannotTriggerLLM(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	memoryTools := tool.GetMemoryTools()
	if len(memoryTools) == 0 {
		t.Skip("no memory tools registered")
	}
	first := memoryTools[0]
	definition, _, err := adapter.Adapt(first, true)
	if err != nil {
		t.Fatal(err)
	}
	hasLLM := false
	for _, trigger := range definition.Triggers {
		if trigger == TriggerLLM {
			hasLLM = true
			break
		}
	}
	if hasLLM {
		t.Fatal("memory tool should not have LLM trigger")
	}
	hasManual := false
	for _, trigger := range definition.Triggers {
		if trigger == TriggerManual {
			hasManual = true
			break
		}
	}
	if !hasManual {
		t.Fatal("memory tool should allow manual trigger")
	}
}

func TestLegacy_Tool_CapabilityMappingCoverage(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	knownTools := map[string]bool{
		"get_current_time":     true,
		"create_schedule":      true,
		"force_voice_reply":    true,
		"read_need_state":      true,
		"read_psyche_state":    true,
		"summarize_memories":   true,
		"save_memory":          true,
		"save_profile":         true,
		"save_episodic_memory": true,
	}
	allTools := append(tool.GetAll(), tool.GetMemoryTools()...)
	for _, legacy := range allTools {
		if knownTools[legacy.Function.Name] {
			definition, _, err := adapter.Adapt(legacy, false)
			if err != nil {
				t.Fatalf("adapt %s: %v", legacy.Function.Name, err)
			}
			if len(definition.Capabilities) == 0 {
				t.Fatalf("known tool %s should have capabilities", legacy.Function.Name)
			}
			delete(knownTools, legacy.Function.Name)
		}
	}
	if len(knownTools) > 0 {
		t.Logf("WARNING: %d known tools not found in GetAll()+GetMemoryTools(): %v", len(knownTools), knownTools)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func hashRaw(raw json.RawMessage) string {
	if raw == nil {
		return "nil"
	}
	h := sha256.Sum256(raw)
	return fmt.Sprintf("%x", h[:8])
}
