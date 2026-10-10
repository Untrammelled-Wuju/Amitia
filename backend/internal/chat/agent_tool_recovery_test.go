package chat

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	applog "github.com/u-ai/backend/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testAgentRecoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "agent-recovery.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAgentRecoveryRestoresCompletedDurableToolResultWithoutReplay(t *testing.T) {
	db := testAgentRecoveryDB(t)
	ctx := context.Background()
	initial := newAssistantTurnRecorder(db, "conv-resume", "char", "user", "req")
	if err := initial.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := initial.AddToolCall(ctx, "call-read", "workspace.read", `{"path":"a.go"}`, assistantTurnStatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := initial.AddToolResult(ctx, "call-read", "workspace.read", "source already read", assistantTurnStatusCompleted, "", 8); err != nil {
		t.Fatal(err)
	}
	resumed := newAssistantTurnRecorder(db, "conv-resume", "char", "user", "req", initial.TurnID, initial.ExecutionID)
	if err := resumed.Start(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []tool.Tool{{Type: "function", Function: tool.Function{Name: "workspace.read"}}}
	called := false
	svc := &service{llmWithTools: func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		called = true
		if len(messages) != 3 {
			t.Fatalf("restored messages = %#v", messages)
		}
		if messages[1]["role"] != "assistant" || messages[2]["role"] != "tool" || messages[2]["content"] != "source already read" {
			t.Fatalf("missing completed tool transcript: %#v", messages)
		}
		if strings.Contains(tools[0].Function.Name, ".") {
			t.Fatalf("tool alias not restored: %s", tools[0].Function.Name)
		}
		return "resume complete", "", nil, 7, nil
	}}
	reply, _, _, _, _, err := svc.invokeLLMWithTools(
		ctx, &ModelConfig{ContextWindow: 500000}, []map[string]interface{}{{"role": "system", "content": "run task"}},
		applog.TraceFields{}, nil, "user", "conv-resume", "char", "web", "req", "", "", "request_approval",
		nil, definitions, map[string]bool{}, ctx, resumed)
	if err != nil || reply != "resume complete" || !called {
		t.Fatalf("resume reply=%q called=%v err=%v", reply, called, err)
	}
}

func TestAgentRecoveryRefusesIndeterminateToolCall(t *testing.T) {
	db := testAgentRecoveryDB(t)
	ctx := context.Background()
	r := newAssistantTurnRecorder(db, "conv-resume", "char", "user", "req")
	if err := r.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.AddToolCall(ctx, "unknown-side-effect", "workspace.write", `{"path":"a.go"}`, assistantTurnStatusRunning); err != nil {
		t.Fatal(err)
	}
	restored := newAssistantTurnRecorder(db, "conv-resume", "char", "user", "req", r.TurnID, r.ExecutionID)
	if err := restored.Start(ctx); err != nil {
		t.Fatal(err)
	}
	called := false
	svc := &service{llmWithTools: func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		called = true
		return "unsafe", "", nil, 0, nil
	}}
	_, _, _, _, _, err := svc.invokeLLMWithTools(
		ctx, &ModelConfig{ContextWindow: 500000}, nil, applog.TraceFields{}, nil,
		"user", "conv-resume", "char", "web", "req", "", "", "request_approval", nil,
		[]tool.Tool{{Type: "function", Function: tool.Function{Name: "workspace.write"}}},
		map[string]bool{}, ctx, restored)
	if err == nil || !strings.Contains(err.Error(), "unresolved") || called {
		t.Fatalf("indeterminate side effects must not be replayed: err=%v called=%v", err, called)
	}
}

func TestAgentRecoveryKeepsParallelCallGroups(t *testing.T) {
	alias := map[string]string{"read_file": "read_file"}
	items := []AssistantTurnItem{
		{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "read_file", ArgumentsJSON: "{}"},
		{ItemType: assistantTurnItemToolCall, CallID: "b", ToolName: "read_file", ArgumentsJSON: "{}"},
		{ItemType: assistantTurnItemToolResult, CallID: "b", ResultJSON: `"second"`},
		{ItemType: assistantTurnItemToolResult, CallID: "a", ResultJSON: `"first"`},
		{ItemType: assistantTurnItemToolCall, CallID: "c", ToolName: "read_file", ArgumentsJSON: "{}"},
		{ItemType: assistantTurnItemToolResult, CallID: "c", ResultJSON: `"third"`},
	}
	msg, err := restoreAgentToolMessages(items, alias)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) != 5 || msg[0]["role"] != "assistant" || msg[3]["role"] != "assistant" {
		t.Fatalf("parallel tool groups reconstructed incorrectly: %#v", msg)
	}
	calls, _ := msg[0]["tool_calls"].([]map[string]interface{})
	if len(calls) != 2 || msg[1]["tool_call_id"] != "a" || msg[1]["content"] != "first" || msg[2]["content"] != "second" {
		t.Fatalf("tool result order or identity damaged: %#v", msg)
	}
}

func TestAgentRecoveryRejectsOrphanResult(t *testing.T) {
	_, err := restoreAgentToolMessages([]AssistantTurnItem{{ItemType: assistantTurnItemToolResult, CallID: "orphan"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("expected orphan result to be rejected, got %v", err)
	}
}
