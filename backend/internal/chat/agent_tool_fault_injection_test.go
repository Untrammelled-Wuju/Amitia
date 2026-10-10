package chat

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func faultWindowCall(db *gorm.DB) (*assistantTurnRecorder, agentToolCall) {
	recorder := newAssistantTurnRecorder(db, "crash-conversation", "crash-character",
		"crash-user", "crash-request", "crash-turn", "crash-execution")
	recorder.enabled = true
	return recorder, agentToolCall{
		ID: "crash-call", Name: "execute_host_command", Arguments: `{"command":"modify"}`,
		Scope: SkillScope{ConversationID: "crash-conversation", CharacterID: "crash-character",
			RequestID: "crash-request", Channel: "web", ToolCallID: "crash-call"},
	}
}

func TestAgentToolLedgerFaultHelper(t *testing.T) {
	if os.Getenv("HARNESS_FAULT_CHILD") != "1" {
		return
	}
	db, err := gorm.Open(sqlite.Open(os.Getenv("HARNESS_FAULT_DB")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	recorder, call := faultWindowCall(db)
	point := os.Getenv("HARNESS_FAULT_POINT")
	if point == "before_intent" {
		os.Exit(37)
	}
	ledger := newAgentToolLedger(recorder, call)
	if _, err := ledger.begin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if point == "after_intent" || point == "before_effect" {
		os.Exit(37)
	}
	file, err := os.OpenFile(os.Getenv("HARNESS_FAULT_EFFECT"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if point == "during_effect" || point == "after_effect" {
		os.Exit(37)
	}
	outcome := toolExecOutcome{Status: "SUCCEEDED", Output: []byte(`{"exitCode":0}`), Found: true}
	if err := ledger.finish(context.Background(), outcome); err != nil {
		t.Fatal(err)
	}
	if point == "after_ledger_result" {
		os.Exit(37)
	}
	if point == "after_turn_checkpoint" {
		err := db.Exec("INSERT INTO assistant_turn_items (id, turn_id, conversation_id, sequence, item_type, status, revision, call_id, tool_name, result_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			"checkpoint-result", "crash-turn", "crash-conversation", 2, assistantTurnItemToolResult,
			assistantTurnStatusCompleted, 2, "crash-call", "execute_host_command", `{"exitCode":0}`).Error
		if err != nil {
			t.Fatal(err)
		}
		os.Exit(37)
	}
	t.Fatalf("unrecognized fault point %q", point)
}

func TestAgentToolLedgerSevenRealProcessCrashWindows(t *testing.T) {
	for _, point := range []struct {
		name          string
		initialEffect int
		replayAllowed bool
	}{
		{name: "before_intent", initialEffect: 0, replayAllowed: true},
		{name: "after_intent", initialEffect: 0},
		{name: "before_effect", initialEffect: 0},
		{name: "during_effect", initialEffect: 1},
		{name: "after_effect", initialEffect: 1},
		{name: "after_ledger_result", initialEffect: 1},
		{name: "after_turn_checkpoint", initialEffect: 1},
	} {
		t.Run(point.name, func(t *testing.T) {
			dir := t.TempDir()
			databasePath := filepath.Join(dir, "journal.sqlite")
			effectPath := filepath.Join(dir, "side-effect.txt")
			db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			for _, ddl := range []string{
				`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, attempt_id TEXT, turn_id TEXT, execution_id TEXT, input_hash TEXT, owner_instance_id TEXT, result_ref TEXT, error_class TEXT, started_at TEXT, finished_at TEXT, created_at TEXT, updated_at TEXT)`,
				`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, attempt_id TEXT, turn_id TEXT, execution_id TEXT, input_hash TEXT, created_at TEXT)`,
			} {
				if err := db.Exec(ddl).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			_ = sqlDB.Close()
			cmd := exec.Command(os.Args[0], "-test.run=^TestAgentToolLedgerFaultHelper$")
			cmd.Env = append(os.Environ(),
				"HARNESS_FAULT_CHILD=1", "HARNESS_FAULT_POINT="+point.name,
				"HARNESS_FAULT_DB="+databasePath, "HARNESS_FAULT_EFFECT="+effectPath,
			)
			output, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 37 {
				t.Fatalf("fault process did not exit at injection point: err=%v output=%s", err, output)
			}
			db, err = gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err = db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			effect, err := os.ReadFile(effectPath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(effect) != point.initialEffect {
				t.Fatalf("unexpected pre-recovery side effect count: got=%d want=%d", len(effect), point.initialEffect)
			}
			recorder, call := faultWindowCall(db)
			ran := false
			recovery := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
				ran = true
				f, err := os.OpenFile(effectPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.WriteString("x")
				_ = f.Close()
				return agentToolExecution{Outcome: toolExecOutcome{
					Status: "SUCCEEDED", Output: []byte(`{"exitCode":0}`), Found: true,
				}}
			})
			if ran != point.replayAllowed {
				t.Fatalf("unsafe replay decision: ran=%v allowed=%v outcome=%+v", ran, point.replayAllowed, recovery)
			}
			if !point.replayAllowed && point.name != "after_ledger_result" && point.name != "after_turn_checkpoint" &&
				recovery.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
				t.Fatalf("unknown side effect did not enter reconciliation: %+v", recovery)
			}
			effect, err = os.ReadFile(effectPath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			expected := point.initialEffect
			if point.replayAllowed {
				expected++
			}
			if len(effect) != expected {
				t.Fatalf("side effect replayed: count=%d expected=%d", len(effect), expected)
			}
		})
	}
}
