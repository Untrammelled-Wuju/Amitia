package prompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name)
}

func ensureGoldenDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("testdata", "golden")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create golden dir: %v", err)
	}
	return dir
}

func writeGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	ensureGoldenDir(t)
	path := goldenPath(name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write golden file: %v", err)
	}
}

func readGolden(t *testing.T, name string) ([]byte, bool) {
	t.Helper()
	path := goldenPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func TestGoldenCompileIRSnapshot(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 120, Source: "system", Sensitivity: SensitivityInternal, Content: "You are a helpful companion. Stay safe and respectful. Do not role-play harmful scenarios."},
		{Type: SectionTypeIdentity, Priority: 90, TokenBudget: 80, Source: "character", Sensitivity: SensitivityInternal, Content: "Name: Amitia. Identity: AI companion. Speaking style: warm and direct."},
		{Type: SectionTypeBehaviorPlan, Priority: 80, TokenBudget: 60, Source: "decision", Sensitivity: SensitivityInternal, Content: "Be warm. Use humor when appropriate. Stay concise."},
		{Type: SectionTypeMemory, Priority: 40, TokenBudget: 50, Source: "memory", Sensitivity: SensitivityUserData, Content: "User likes tea. User mentioned jogging on weekends."},
		{Type: SectionTypeHistory, Priority: 20, TokenBudget: 40, Source: "history", Sensitivity: SensitivityUserData, Trimmable: true, Content: "User: Hello\nAmitia: Hi there!"},
	}

	ir := CompileIR(sections, CompileOptions{
		DropEmptySections: true,
	})

	snapshot := SnapshotIR(ir)

	goldenName := "compile_ir_snapshot.json"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	expected, ok := readGolden(t, goldenName)
	if !ok {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	actual, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal actual: %v", err)
	}

	var expectedIR, actualIR IR
	if err := json.Unmarshal(expected, &expectedIR); err != nil {
		t.Fatalf("failed to unmarshal golden: %v", err)
	}
	if err := json.Unmarshal(actual, &actualIR); err != nil {
		t.Fatalf("failed to unmarshal actual: %v", err)
	}

	if expectedIR.Version != actualIR.Version {
		t.Fatalf("golden mismatch version:\nexpected=%s\nactual=%s", expectedIR.Version, actualIR.Version)
	}
	if len(expectedIR.Sections) != len(actualIR.Sections) {
		t.Fatalf("golden mismatch section count:\nexpected=%d\nactual=%d", len(expectedIR.Sections), len(actualIR.Sections))
	}
	for i := range expectedIR.Sections {
		if expectedIR.Sections[i].Type != actualIR.Sections[i].Type {
			t.Fatalf("golden mismatch section[%d] type:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Type, actualIR.Sections[i].Type)
		}
		if expectedIR.Sections[i].Content != actualIR.Sections[i].Content {
			t.Fatalf("golden mismatch section[%d] content:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Content, actualIR.Sections[i].Content)
		}
	}
}

func TestGoldenBudgetApply(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 20, Source: "system", Sensitivity: SensitivityInternal, Content: "You are a helpful assistant. Follow safety rules. Be respectful. Do not share personal data."},
		{Type: SectionTypeCurrentInput, Priority: 90, TokenBudget: 15, Source: "message", Sensitivity: SensitivityUserData, Content: "What is the weather like today in Beijing?"},
		{Type: SectionTypeMemory, Priority: 40, TokenBudget: 12, Source: "memory", Sensitivity: SensitivityUserData, Trimmable: true, Content: "User likes tea. User lives in Beijing. User jogs every morning."},
		{Type: SectionTypeHistory, Priority: 20, TokenBudget: 10, Source: "history", Sensitivity: SensitivityUserData, Trimmable: true, Content: "User: Hi\nAssistant: Hello! User: How are you today?"},
	}

	ir := CompileIR(sections, CompileOptions{DropEmptySections: true})

	budgeted := ApplyBudget(ir, BudgetPolicy{
		MaxPromptTokens: 30,
		SectionLimits: map[SectionType]SectionBudget{
			SectionTypeSystem:       {MaxTokens: 15, MinTokens: 4, Priority: 100},
			SectionTypeCurrentInput: {MaxTokens: 12, MinTokens: 4, Priority: 90},
			SectionTypeMemory:       {MaxTokens: 6, MinTokens: 0, Priority: 40, TrimReason: "low_priority_memory_trimmed"},
			SectionTypeHistory:      {MaxTokens: 4, MinTokens: 0, Priority: 20, TrimReason: "old_history_trimmed"},
		},
	})

	goldenName := "budget_apply.json"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		data, err := json.MarshalIndent(budgeted, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	expected, ok := readGolden(t, goldenName)
	if !ok {
		data, err := json.MarshalIndent(budgeted, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal golden: %v", err)
		}
		writeGolden(t, goldenName, data)
		return
	}

	actual, err := json.MarshalIndent(budgeted, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal actual: %v", err)
	}

	var expectedIR, actualIR IR
	if err := json.Unmarshal(expected, &expectedIR); err != nil {
		t.Fatalf("failed to unmarshal golden: %v", err)
	}
	if err := json.Unmarshal(actual, &actualIR); err != nil {
		t.Fatalf("failed to unmarshal actual: %v", err)
	}

	if len(expectedIR.Sections) != len(actualIR.Sections) {
		t.Fatalf("golden mismatch section count:\nexpected=%d\nactual=%d", len(expectedIR.Sections), len(actualIR.Sections))
	}
	for i := range expectedIR.Sections {
		if expectedIR.Sections[i].Type != actualIR.Sections[i].Type {
			t.Fatalf("golden mismatch section[%d] type:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Type, actualIR.Sections[i].Type)
		}
		if expectedIR.Sections[i].Content != actualIR.Sections[i].Content {
			t.Fatalf("golden mismatch section[%d] content:\nexpected=%s\nactual=%s", i, expectedIR.Sections[i].Content, actualIR.Sections[i].Content)
		}
	}
	if len(expectedIR.Audit.TrimRecords) != len(actualIR.Audit.TrimRecords) {
		t.Fatalf("golden mismatch trim records:\nexpected=%d\nactual=%d", len(expectedIR.Audit.TrimRecords), len(actualIR.Audit.TrimRecords))
	}
}

func TestGoldenRenderIR(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 40, Source: "system", Sensitivity: SensitivityInternal, Content: "You are a companion AI. Be safe, kind, and helpful."},
		{Type: SectionTypeIdentity, Priority: 90, TokenBudget: 30, Source: "character", Sensitivity: SensitivityInternal, Content: "Name: Amitia"},
		{Type: SectionTypeBehaviorPlan, Priority: 80, TokenBudget: 30, Source: "decision", Sensitivity: SensitivityInternal, Content: "Be warm and concise."},
		{Type: SectionTypeMemory, Priority: 40, TokenBudget: 20, Source: "memory", Sensitivity: SensitivityUserData, DataOnly: true, Content: "User likes tea."},
	}

	ir := CompileIR(sections, CompileOptions{DropEmptySections: true})
	rendered := RenderIR(ir)

	goldenName := "render_ir.txt"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		writeGolden(t, goldenName, []byte(rendered))
		return
	}

	expected, ok := readGolden(t, goldenName)
	if !ok {
		writeGolden(t, goldenName, []byte(rendered))
		return
	}

	if string(expected) != rendered {
		t.Fatalf("golden mismatch render:\nexpected:\n%s\nactual:\n%s", string(expected), rendered)
	}
}
