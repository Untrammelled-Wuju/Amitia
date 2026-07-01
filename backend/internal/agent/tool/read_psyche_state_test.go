package tool

import (
	"context"
	"testing"
)

func TestReadPsycheStateInit(t *testing.T) {
	for _, memTool := range GetMemoryTools() {
		if memTool.Function.Name == "read_psyche_state" {
			if memTool.Function.Description == "" {
				t.Fatal("read_psyche_state description should not be empty")
			}
			if len(memTool.Function.Parameters.Properties) == 0 {
				t.Fatal("read_psyche_state should have parameters")
			}
			_, hasChar := memTool.Function.Parameters.Properties["character_id"]
			_, hasBelief := memTool.Function.Parameters.Properties["include_beliefs"]
			if !hasChar {
				t.Fatal("read_psyche_state should have character_id parameter")
			}
			if !hasBelief {
				t.Fatal("read_psyche_state should have include_beliefs parameter")
			}
			return
		}
	}
	t.Fatal("read_psyche_state tool not registered in memory tools")
}

func TestReadPsycheStateMissingCharacter(t *testing.T) {
	result := readPsycheState(context.Background(), ToolExecutionContext{}, map[string]interface{}{})
	if result.Status != ToolStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.ErrorCode != "missing_character_scope" {
		t.Fatalf("expected missing_character_scope, got %s", result.ErrorCode)
	}
}

func TestReadPsycheStateCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := readPsycheState(ctx, ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-1",
	})
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", result.Status)
	}
}

func TestReadPsycheStateNoDB(t *testing.T) {
	oldDB := toolDB
	toolDB = nil
	defer func() { toolDB = oldDB }()

	result := readPsycheState(context.Background(), ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-1",
	})
	if result.Status != ToolStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.ErrorCode != "database_not_initialized" {
		t.Fatalf("expected database_not_initialized, got %s", result.ErrorCode)
	}
}
