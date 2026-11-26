//go:build rsi_gap

package capability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func gapTool(t *testing.T, registry *ToolRegistry, slug, name, description string, enabled bool) ToolDefinition {
	t.Helper()
	definition := ToolDefinition{
		ID:          BuildToolID(ToolSourceBuiltin, "gap", slug),
		ModelName:   "gap_" + slug,
		Source:      ToolSourceBuiltin,
		Name:        name,
		Description: description,
		Enabled:     enabled,
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := registry.Register(context.Background(), definition); err != nil {
		t.Fatalf("register %s: %v", slug, err)
	}
	return definition
}

func gapSelector(registry *ToolRegistry) *TaskSelector {
	return NewTaskSelector(registry, nil)
}

func TestGapReportExistingCapability(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "hello", "hello greeting", "greet a person", true)
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "hello greeting", nil)
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Existing) != 1 || report.Verdict() != GapExisting {
		t.Fatalf("report=%+v verdict=%s", report, report.Verdict())
	}
	fmt.Printf("m08-s01 existing count=1 verdict=existing\n")
}

func TestGapReportComposableCapability(t *testing.T) {
	registry := NewToolRegistry()
	read := gapTool(t, registry, "read-document", "read document", "read a document", true)
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "summarize and read document", []CompositionRule{
		{Result: "summarize", Requires: []string{read.ID}, Description: "summarize a document"},
	})
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Composable) != 1 || len(report.Missing) != 0 || report.Verdict() != GapExisting {
		t.Fatalf("report=%+v", report)
	}
	fmt.Printf("m08-s01 composable count=1 missing=0\n")
}

func TestGapReportMissingDependency(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "read-document", "read document", "read a document", true)
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "summarize and read document", []CompositionRule{
		{Result: "summarize", Requires: []string{"translate-unknown"}},
	})
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Missing) != 1 || report.Missing[0].Reasons[0] != ReasonDependencyMissing || len(report.Missing[0].MissingRequires) != 1 {
		t.Fatalf("missing=%+v", report.Missing)
	}
	fmt.Printf("m08-s01 missing_dependency=1\n")
}

func TestGapReportNoCatalog(t *testing.T) {
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(NewToolRegistry()), "do something impossible", nil)
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Missing) != 1 || report.Missing[0].Reasons[0] != ReasonNotInstalled {
		t.Fatalf("missing=%+v", report.Missing)
	}
	fmt.Printf("m08-s01 no_catalog missing=1 reason=not_installed\n")
}

func TestGapReportDuplicateRule(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "read", "read", "read", true)
	rules := []CompositionRule{
		{Result: "summarize", Requires: []string{"a"}},
		{Result: "summarize", Requires: []string{"b"}},
	}
	if _, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "summarize read", rules); err == nil {
		t.Fatalf("duplicate rule accepted")
	}
	fmt.Printf("m08-s01 duplicate_rule rejected=true\n")
}

func TestGapReportEmptyGoal(t *testing.T) {
	if _, err := AnalyzeCapabilityGap(context.Background(), gapSelector(NewToolRegistry()), "  ", nil); err == nil {
		t.Fatalf("empty goal accepted")
	}
	fmt.Printf("m08-s01 empty_goal rejected=true\n")
}

func TestGapReportNilSelector(t *testing.T) {
	if _, err := AnalyzeCapabilityGap(context.Background(), nil, "goal", nil); err == nil {
		t.Fatalf("nil selector accepted")
	}
	fmt.Printf("m08-s01 nil_selector rejected=true\n")
}

func TestGapReportRuleWithoutRequires(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "read", "read", "read", true)
	if _, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "compose read", []CompositionRule{{Result: "compose"}}); err == nil {
		t.Fatalf("rule without requires accepted")
	}
	fmt.Printf("m08-s01 rule_requires rejected=true\n")
}

func TestGapReportRejectedTool(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "disabled", "disabled tool", "cannot run", false)
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "disabled tool", nil)
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Missing) != 1 || report.Missing[0].Reasons[0] != ReasonToolDisabled {
		t.Fatalf("missing=%+v", report.Missing)
	}
	fmt.Printf("m08-s01 rejected reason=tool_disabled\n")
}

func TestGapReportOrdering(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "b-tool", "write report", "write report", true)
	gapTool(t, registry, "a-tool", "write report", "write report", true)
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "write report", nil)
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Existing) != 2 || report.Existing[0].ToolID >= report.Existing[1].ToolID {
		t.Fatalf("existing=%+v", report.Existing)
	}
	fmt.Printf("m08-s01 ordering sorted=true\n")
}

func TestGapReportExistingWinsOverRule(t *testing.T) {
	registry := NewToolRegistry()
	gapTool(t, registry, "summarize", "summarize", "summarize text", true)
	report, err := AnalyzeCapabilityGap(context.Background(), gapSelector(registry), "summarize text", []CompositionRule{
		{Result: "summarize", Requires: []string{"other"}},
	})
	if err != nil {
		t.Fatalf("gap: %v", err)
	}
	if len(report.Existing) != 1 || len(report.Composable) != 0 || len(report.Missing) != 0 {
		t.Fatalf("report=%+v", report)
	}
	fmt.Printf("m08-s01 existing_precedence=true\n")
}

func TestGapReportVerdictPrecedence(t *testing.T) {
	withExisting := CapabilityGapReport{Existing: []GapEntry{{State: GapExisting}}, Missing: []GapEntry{{State: GapMissing}}}
	withComposable := CapabilityGapReport{Composable: []GapEntry{{State: GapComposable}}, Missing: []GapEntry{{State: GapMissing}}}
	if withExisting.Verdict() != GapExisting || withComposable.Verdict() != GapComposable {
		t.Fatalf("verdict precedence broken")
	}
	var missingErr error
	missingErr = errors.New("x")
	_ = missingErr
	fmt.Printf("m08-s01 verdict precedence=existing>composable>missing\n")
}
