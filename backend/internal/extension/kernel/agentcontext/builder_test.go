//go:build rsi_agentcontext

package agentcontext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func agentContextLog(t *testing.T, line string) {
	t.Helper()
	t.Logf("m05-s06 %s", line)
}

func agentContextState() capability.ToolState {
	return capability.ToolState{
		Installed:         true,
		ModuleEnabled:     true,
		CapabilityEnabled: true,
		ScopeAllowed:      true,
		PermissionGranted: true,
		RuntimeReady:      true,
		DependencyReady:   true,
		Health:            capability.HealthReady,
	}
}

func agentContextSchema(required int, optional int, pad int) json.RawMessage {
	properties := map[string]any{}
	requiredNames := make([]string, 0, required)
	for i := 0; i < required; i++ {
		name := fmt.Sprintf("req_%02d", i)
		properties[name] = map[string]any{"type": "string"}
		requiredNames = append(requiredNames, name)
	}
	for i := 0; i < optional; i++ {
		name := fmt.Sprintf("opt_%02d", i)
		properties[name] = map[string]any{"type": "string", "description": strings.Repeat("x", pad)}
	}
	body := map[string]any{"type": "object", "properties": properties}
	if len(requiredNames) > 0 {
		body["required"] = requiredNames
	}
	raw, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return raw
}

func agentContextTool(id string, name string, description string, keywords ...string) capability.ToolDefinition {
	metadata := map[string]any{}
	if len(keywords) > 0 {
		metadata["keywords"] = keywords
	}
	return capability.ToolDefinition{
		ID:          id,
		ModelName:   name,
		Source:      capability.ToolSourceBuiltin,
		Name:        name,
		Description: description,
		Enabled:     true,
		InputSchema: agentContextSchema(1, 6, 240),
		State:       agentContextState(),
		Metadata:    metadata,
	}
}

func agentContextRegistry(t *testing.T, defs ...capability.ToolDefinition) *capability.ToolRegistry {
	t.Helper()
	registry := capability.NewToolRegistry()
	for _, def := range defs {
		if err := registry.Register(context.Background(), def); err != nil {
			t.Fatalf("Register %s: %v", def.ID, err)
		}
	}
	return registry
}

func agentContextDefinitions() []capability.ToolDefinition {
	return []capability.ToolDefinition{
		agentContextTool("tool-report-writer", "report_writer", "写入周报与报告文档", "周报", "报告"),
		agentContextTool("tool-report-reader", "report_reader", "读取周报与报告", "周报", "报告"),
		agentContextTool("tool-weekly-summary", "weekly_summary", "汇总本周周报", "周报", "汇总"),
		agentContextTool("tool-calendar", "calendar_lookup", "查询日程", "日程"),
	}
}

func agentContextManager(t *testing.T, withSelector bool, withSchema bool) *capability.ToolExposureManager {
	t.Helper()
	defs := agentContextDefinitions()
	registry := agentContextRegistry(t, defs...)
	manager := capability.NewToolExposureManager(registry, nil)
	if withSelector {
		selector := capability.NewTaskSelector(registry, nil)
		manager = manager.WithTaskSelector(selector)
	}
	if withSchema {
		store := capability.NewSchemaStore(0)
		index, err := capability.BuildDescriptorIndex(store, defs)
		if err != nil {
			t.Fatalf("BuildDescriptorIndex: %v", err)
		}
		manager = manager.WithSchemaSource(index, store)
	}
	return manager
}

func TestAgentContextTaskModeSelectsExplicitTool(t *testing.T) {
	manager := agentContextManager(t, true, false)
	builder, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder: %v", err)
	}
	ctx := AgentContextRequest{Query: "用 report_writer 生成周报", Limit: 2}
	got, err := builder.Build(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Mode != AgentContextTask {
		t.Fatalf("mode=%s want %s", got.Mode, AgentContextTask)
	}
	names := got.Names()
	if len(names) == 0 || names[0] != "report_writer" {
		t.Fatalf("top1=%v want report_writer", names)
	}
	if got.Tokens <= 0 {
		t.Fatalf("tokens=%d want positive", got.Tokens)
	}
	agentContextLog(t, fmt.Sprintf("task top1=%s tools=%d tokens=%d mode=%s", names[0], len(names), got.Tokens, got.Mode))
}

func TestAgentContextLegacyModeMatchesExposure(t *testing.T) {
	manager := agentContextManager(t, false, false)
	builder, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder: %v", err)
	}
	session := capability.ToolInvocationContext{InvocationID: "inv-legacy", SpaceID: "space-1"}
	got, err := builder.Build(context.Background(), AgentContextRequest{Session: session})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := manager.GetModelTools(context.Background(), session, 0)
	if got.Mode != AgentContextLegacy {
		t.Fatalf("mode=%s want %s", got.Mode, AgentContextLegacy)
	}
	if !reflect.DeepEqual(got.Tools, want) {
		t.Fatalf("legacy tools differ from exposure output: got=%d want=%d", len(got.Tools), len(want))
	}
	agentContextLog(t, fmt.Sprintf("legacy views=%d equal=true mode=%s tokens=%d", len(got.Tools), got.Mode, got.Tokens))
}

func TestAgentContextDetailModeHonoursBudget(t *testing.T) {
	manager := agentContextManager(t, true, true)
	builder, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder: %v", err)
	}
	request := AgentContextRequest{Query: "把周报整理成报告", Limit: 2, TokenBudget: 220, Detail: true}
	first, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(first.Tools) == 0 {
		t.Fatalf("detail page is empty")
	}
	if first.Tokens > request.TokenBudget {
		t.Fatalf("tokens=%d exceeds budget=%d", first.Tokens, request.TokenBudget)
	}
	if first.Reads != int64(len(first.Tools)) {
		t.Fatalf("reads=%d want %d", first.Reads, len(first.Tools))
	}
	if first.Trimmed < 1 {
		t.Fatalf("trimmed=%d want at least one trimmed schema", first.Trimmed)
	}
	if !first.HasMore || first.Cursor == "" {
		t.Fatalf("has_more=%v cursor=%q want a next page", first.HasMore, first.Cursor)
	}
	agentContextLog(t, fmt.Sprintf("detail views=%d tokens=%d budget=%d reads=%d trimmed=%d has_more=%v", len(first.Tools), first.Tokens, request.TokenBudget, first.Reads, first.Trimmed, first.HasMore))

	request.Cursor = first.Cursor
	second, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build second page: %v", err)
	}
	seen := map[string]bool{}
	for _, name := range first.Names() {
		seen[name] = true
	}
	for _, name := range second.Names() {
		if seen[name] {
			t.Fatalf("duplicate tool across pages: %s", name)
		}
		seen[name] = true
	}
	if len(seen) != len(first.Tools)+len(second.Tools) {
		t.Fatalf("distinct=%d want %d", len(seen), len(first.Tools)+len(second.Tools))
	}
	agentContextLog(t, fmt.Sprintf("cursor second=%d distinct=%d", len(second.Tools), len(seen)))
}

func TestAgentContextRejectsInvalidRequests(t *testing.T) {
	manager := agentContextManager(t, true, false)
	builder, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder: %v", err)
	}
	rejections := 0
	if _, err := NewAgentContextBuilder(nil); !errors.Is(err, ErrAgentContextInvalid) {
		t.Fatalf("nil exposure err=%v want %v", err, ErrAgentContextInvalid)
	}
	rejections++
	if _, err := builder.WithTokenBudget(-1); !errors.Is(err, ErrAgentContextInvalid) {
		t.Fatalf("negative builder budget err=%v want %v", err, ErrAgentContextInvalid)
	}
	rejections++
	var nilBuilder *AgentContextBuilder
	if _, err := nilBuilder.Build(context.Background(), AgentContextRequest{Query: "x"}); !errors.Is(err, ErrAgentContextInvalid) {
		t.Fatalf("nil builder err=%v want %v", err, ErrAgentContextInvalid)
	}
	rejections++
	if _, err := builder.Build(context.Background(), AgentContextRequest{Query: "x", Limit: -1}); !errors.Is(err, ErrAgentContextInvalid) {
		t.Fatalf("negative limit err=%v want %v", err, ErrAgentContextInvalid)
	}
	rejections++
	if _, err := builder.Build(context.Background(), AgentContextRequest{Query: "x", TokenBudget: -1}); !errors.Is(err, ErrAgentContextInvalid) {
		t.Fatalf("negative budget err=%v want %v", err, ErrAgentContextInvalid)
	}
	rejections++
	if _, err := builder.Build(context.Background(), AgentContextRequest{Query: "   "}); !errors.Is(err, ErrAgentContextInvalid) {
		t.Fatalf("empty query err=%v want %v", err, ErrAgentContextInvalid)
	}
	rejections++
	agentContextLog(t, fmt.Sprintf("invalid rejections=%d all_rejected=true", rejections))
}

func TestAgentContextRepeatAndRestartAreStable(t *testing.T) {
	manager := agentContextManager(t, true, true)
	builder, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder: %v", err)
	}
	request := AgentContextRequest{Query: "把周报整理成报告", Limit: 2, TokenBudget: 220, Detail: true}
	first, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	second, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build repeat: %v", err)
	}
	if !reflect.DeepEqual(first.Names(), second.Names()) {
		t.Fatalf("repeat names differ: %v vs %v", first.Names(), second.Names())
	}
	if first.Tokens != second.Tokens || first.Cursor != second.Cursor {
		t.Fatalf("repeat tokens=%d/%d cursor=%q/%q", first.Tokens, second.Tokens, first.Cursor, second.Cursor)
	}
	if second.Reads != int64(len(second.Tools)) {
		t.Fatalf("repeat reads=%d want %d", second.Reads, len(second.Tools))
	}
	agentContextLog(t, fmt.Sprintf("repeat equal=true tokens=%d reads_per_call=%d", second.Tokens, second.Reads))

	restarted, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder restart: %v", err)
	}
	third, err := restarted.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build restart: %v", err)
	}
	if !reflect.DeepEqual(first.Tools, third.Tools) {
		t.Fatalf("restart tools differ: %v vs %v", first.Names(), third.Names())
	}
	agentContextLog(t, fmt.Sprintf("restart equal=true tools=%d", len(third.Tools)))
}

func TestAgentContextCancelPropagates(t *testing.T) {
	manager := agentContextManager(t, true, true)
	builder, err := NewAgentContextBuilder(manager)
	if err != nil {
		t.Fatalf("NewAgentContextBuilder: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := builder.Build(ctx, AgentContextRequest{Query: "把周报整理成报告", Detail: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want %v", err, context.Canceled)
	}
	if len(got.Tools) != 0 {
		t.Fatalf("cancel returned %d tools", len(got.Tools))
	}
	agentContextLog(t, fmt.Sprintf("cancel passed_through=true tools=%d", len(got.Tools)))
}
