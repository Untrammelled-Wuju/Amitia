package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	coreexec "github.com/u-ai/backend/internal/execution"
	applog "github.com/u-ai/backend/log"
)

type verificationGateRuntime struct {
	ModelToolRuntime
	commands         int
	mutations        int
	failVerification bool
	delegationStatus string
}

func (r *verificationGateRuntime) ExecuteModelTool(_ context.Context, name string, _ json.RawMessage, _ SkillScope, _ string) (ToolResult, bool) {
	switch name {
	case "harness_multi_agent_delegate", "harness_multi_agent_status":
		status := r.delegationStatus
		if status == "" {
			status = "running"
		}
		return ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"coordinationId":"coord-1","status":"` + status + `"}`)}, true
	case "apply_file":
		r.mutations++
		return ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"applied":true}`)}, true
	case "execute_host_command":
		r.commands++
		if r.failVerification {
			return ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"exitCode":1,"stderr":"test failed"}`)}, true
		}
		return ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"exitCode":0,"stdout":"ok"}`)}, true
	default:
		return ToolResult{Status: "FAILED", Error: &ToolError{Code: "UNKNOWN_TOOL"}}, false
	}
}

func workspaceVerificationCall(id, name, args string) []map[string]interface{} {
	return []map[string]interface{}{{
		"id":       id,
		"type":     "function",
		"function": map[string]interface{}{"name": name, "arguments": args},
	}}
}

func testWorkspaceVerificationRun(t *testing.T, runtime *verificationGateRuntime, toolDefinitions []tool.Tool, model func([]map[string]interface{}) (string, []map[string]interface{})) (string, error) {
	t.Helper()
	svc := &service{toolRuntime: runtime}
	svc.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		content, calls := model(messages)
		return content, "", calls, 1, nil
	}
	answer, _, _, _, _, err := svc.invokeLLMWithTools(
		context.Background(), &ModelConfig{ContextWindow: 128000},
		[]map[string]interface{}{{"role": "system", "content": "Work on the repository"}},
		applog.TraceFields{}, nil, "user", "verification-conv", "char", "web", "verification-request",
		"space", "", "request_approval", &coreexec.ExecutionContext{WorkspaceID: "bound-workspace"},
		toolDefinitions, map[string]bool{}, context.Background(), testAgentLoopRecorder(t, "verification-conv", "char", "verification-request"),
	)
	return answer, err
}

func TestWorkspaceAgentRequiresVerificationAfterFileMutation(t *testing.T) {
	runtime := &verificationGateRuntime{}
	defs := []tool.Tool{{Function: tool.Function{Name: "apply_file"}}, {Function: tool.Function{Name: "execute_host_command"}}}
	calls := 0
	reply, err := testWorkspaceVerificationRun(t, runtime, defs, func(messages []map[string]interface{}) (string, []map[string]interface{}) {
		calls++
		switch calls {
		case 1:
			return "", workspaceVerificationCall("mutation-1", "apply_file", `{"path":"src/app.go","operation":"write","content":"package main"}`)
		case 2:
			return "all done", nil
		case 3:
			foundReminder := false
			for _, message := range messages {
				if message["role"] == "system" && strings.Contains(strings.ToLower(message["content"].(string)), "执行验收门禁") {
					foundReminder = true
					break
				}
			}
			if !foundReminder {
				t.Fatal("verification request was not added to the model context")
			}
			return "", workspaceVerificationCall("verification-1", "execute_host_command", `{"command":"go test ./..."}`)
		default:
			return "verified changes", nil
		}
	})
	if err != nil || reply != "verified changes" || calls != 4 || runtime.mutations != 1 || runtime.commands != 1 {
		t.Fatalf("workspace verification did not close: reply=%q err=%v rounds=%d changes=%d checks=%d", reply, err, calls, runtime.mutations, runtime.commands)
	}
}

func TestWorkspaceAgentRefusesUnverifiedFinalClaim(t *testing.T) {
	runtime := &verificationGateRuntime{}
	defs := []tool.Tool{{Function: tool.Function{Name: "apply_file"}}, {Function: tool.Function{Name: "execute_host_command"}}}
	calls := 0
	reply, err := testWorkspaceVerificationRun(t, runtime, defs, func(_ []map[string]interface{}) (string, []map[string]interface{}) {
		calls++
		if calls == 1 {
			return "", workspaceVerificationCall("mutation-1", "apply_file", `{"path":"src/app.go"}`)
		}
		return "I verified everything", nil
	})
	if err == nil || !strings.Contains(err.Error(), "no successful build or test evidence") || reply != "" || calls != 3 {
		t.Fatalf("unverified completion falsely accepted: reply=%q err=%v rounds=%d", reply, err, calls)
	}
}

func TestWorkspaceAgentRejectsFailedVerification(t *testing.T) {
	runtime := &verificationGateRuntime{failVerification: true}
	defs := []tool.Tool{{Function: tool.Function{Name: "apply_file"}}, {Function: tool.Function{Name: "execute_host_command"}}}
	calls := 0
	_, err := testWorkspaceVerificationRun(t, runtime, defs, func(_ []map[string]interface{}) (string, []map[string]interface{}) {
		calls++
		switch calls {
		case 1:
			return "", workspaceVerificationCall("mutation-1", "apply_file", `{"path":"src/app.go"}`)
		case 2:
			return "", workspaceVerificationCall("test-1", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		default:
			return "all done", nil
		}
	})
	if err == nil || runtime.commands != 1 {
		t.Fatalf("failed build/test must not satisfy verification: err=%v checks=%d", err, runtime.commands)
	}
}

func TestWorkspaceAgentVerificationPersistsAcrossResume(t *testing.T) {
	g := &agentVerificationGate{}
	g.Restore([]AssistantTurnItem{
		{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "apply_file", ArgumentsJSON: `{"path":"a.go"}`},
		{ItemType: assistantTurnItemToolResult, CallID: "a", Status: assistantTurnStatusCompleted, ResultJSON: `{"applied":true}`},
		{ItemType: assistantTurnItemToolCall, CallID: "b", ToolName: "execute_host_command", ArgumentsJSON: `{"command":"go test ./..."}`},
		{ItemType: assistantTurnItemToolResult, CallID: "b", Status: assistantTurnStatusFailed, ResultJSON: `{"exitCode":1}`},
	})
	if !g.Pending() {
		t.Fatal("failed verification should remain pending after crash")
	}
	g.Observe("execute_host_command", `{"command":"go test ./..."}`, toolExecOutcome{Found: true, Output: json.RawMessage(`{"exitCode":0}`)})
	if g.Pending() {
		t.Fatal("successful resumed verification should satisfy current mutation")
	}
}

func TestCommandExitCodeIsARealToolFailure(t *testing.T) {
	failed := toolResultToOutcome(ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"exitCode":17,"stderr":"compile failed"}`)}, true)
	if !failed.HasError || failed.ErrorCode != "TOOL_NONZERO_EXIT" || !strings.Contains(toolResultContent("execute_host_command", failed), "compile failed") {
		t.Fatalf("nonzero exit cannot be marked successful: %+v", failed)
	}
	timedOut := toolResultToOutcome(ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"exitCode":0,"timedOut":true}`)}, true)
	if !timedOut.HasError || timedOut.ErrorCode != "TOOL_TIMED_OUT" {
		t.Fatalf("timeout cannot satisfy task completion: %+v", timedOut)
	}
	passed := toolResultToOutcome(ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"exitCode":0,"stdout":"ok"}`)}, true)
	if passed.HasError || !strings.Contains(toolResultContent("execute_host_command", passed), `"exitCode":0`) {
		t.Fatalf("successful command evidence should be preserved: %+v", passed)
	}
}

func TestVerificationCannotBeForgedByQuotedTestNames(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		want          bool
	}{
		{"quoted echo", `echo "go test ./..."`, false},
		{"quoted fake separator", `echo "fake; go test ./..."`, false},
		{"powershell write", `Write-Output 'go test passed'`, false},
		{"unrelated script", `python clean.py`, false},
		{"simple Go test", `go test ./internal/chat`, true},
		{"Windows Go executable", `& 'C:\Code\Go\bin\go.exe' test ./internal/chat`, true},
		{"shell compound", `cd backend && go test ./internal/chat`, true},
		{"package manager build", `pnpm run build`, true},
		{"targeted lint", `eslint .`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arguments, _ := json.Marshal(map[string]string{"command": tc.command})
			if got := agentIsVerificationCommand("execute_host_command", string(arguments)); got != tc.want {
				t.Fatalf("command=%q verified=%v want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestDuplicateModelToolIDsFailBeforeAnySideEffects(t *testing.T) {
	runtime := &verificationGateRuntime{}
	definitions := []tool.Tool{{Function: tool.Function{Name: "apply_file"}}}
	rounds := 0
	svc := &service{toolRuntime: runtime}
	svc.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		rounds++
		calls := workspaceVerificationCall("same-id", "apply_file", `{"path":"a.go"}`)
		calls = append(calls, workspaceVerificationCall("same-id", "apply_file", `{"path":"b.go"}`)...)
		return "", "", calls, 1, nil
	}
	_, _, _, _, _, err := svc.invokeLLMWithTools(context.Background(), &ModelConfig{},
		nil, applog.TraceFields{}, nil, "user", "duplicate", "char", "web", "req", "", "",
		"request_approval", &coreexec.ExecutionContext{WorkspaceID: "workspace"},
		definitions, map[string]bool{}, context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate tool call id") || runtime.mutations != 0 || rounds != 1 {
		t.Fatalf("duplicate tool IDs caused side effects: err=%v edits=%d rounds=%d", err, runtime.mutations, rounds)
	}
}

func TestAgentTokenBudgetIncludesMultimodalParts(t *testing.T) {
	textOnly := estimateModelMessagesTokens([]map[string]interface{}{{
		"role": "user", "parts": []ModelContentPart{{Type: ContentTypeText, Text: "Check the screenshot"}},
	}})
	withImages := estimateModelMessagesTokens([]map[string]interface{}{{
		"role": "user", "parts": []ModelContentPart{
			{Type: ContentTypeText, Text: "Check the screenshot"},
			{Type: ContentTypeImage, Detail: "high", ResourceURI: "https://example.test/screenshot.png"},
			{Type: ContentTypeAudio, ResourceURI: "https://example.test/audio.wav"},
		},
	}})
	if withImages < textOnly+6000 {
		t.Fatalf("image/audio prompt content was not reserved in budget: text=%d rich=%d", textOnly, withImages)
	}
}

func TestHarnessDelegationResultVisibleToModel(t *testing.T) {
	outcome := toolResultToOutcome(ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"coordinationId":"coord-a","status":"running","assignments":[{"status":"running"}]}`)}, true)
	for _, name := range []string{"harness_multi_agent_delegate", "harness_multi_agent_status"} {
		content := toolResultContent(name, outcome)
		if !strings.Contains(content, `"coordinationId":"coord-a"`) || !strings.Contains(content, `"assignments"`) {
			t.Fatalf("multi-agent model tool %q lost its structured outcome: %q", name, content)
		}
	}
}

func TestWorkspaceAgentRefusesIncompleteDelegation(t *testing.T) {
	runtime := &verificationGateRuntime{delegationStatus: "running"}
	defs := []tool.Tool{
		{Function: tool.Function{Name: "harness_multi_agent_delegate"}},
		{Function: tool.Function{Name: "harness_multi_agent_status"}},
		{Function: tool.Function{Name: "execute_host_command"}},
	}
	rounds := 0
	reply, err := testWorkspaceVerificationRun(t, runtime, defs, func(messages []map[string]interface{}) (string, []map[string]interface{}) {
		rounds++
		switch rounds {
		case 1:
			return "", workspaceVerificationCall("delegate", "harness_multi_agent_delegate", `{"objectives":["Analyze backend","Check frontend"]}`)
		case 2:
			return "Both workers done", nil
		case 3:
			hasReminder := false
			for _, m := range messages {
				if m["role"] == "system" && strings.Contains(m["content"].(string), "子任务完成门禁") {
					hasReminder = true
				}
			}
			if !hasReminder {
				t.Fatal("missing mandatory sub-agent status refresh")
			}
			return "", workspaceVerificationCall("check", "harness_multi_agent_status", `{"coordination_id":"coord-1"}`)
		default:
			return "I finished all delegated work", nil
		}
	})
	if err == nil || reply != "" || !strings.Contains(err.Error(), "has not reached a terminal state") || rounds != 5 {
		t.Fatalf("uncompleted worker was misreported as success: reply=%q err=%v rounds=%d", reply, err, rounds)
	}
}

func TestWorkspaceAgentRequiresParentValidationAfterDelegatedChildrenFinish(t *testing.T) {
	runtime := &verificationGateRuntime{delegationStatus: "succeeded"}
	defs := []tool.Tool{
		{Function: tool.Function{Name: "harness_multi_agent_delegate"}},
		{Function: tool.Function{Name: "execute_host_command"}},
	}
	rounds := 0
	reply, err := testWorkspaceVerificationRun(t, runtime, defs, func(_ []map[string]interface{}) (string, []map[string]interface{}) {
		rounds++
		switch rounds {
		case 1:
			return "", workspaceVerificationCall("delegate", "harness_multi_agent_delegate", `{"objectives":["Implement backend","Fix tests"]}`)
		case 2:
			return "Everything is verified", nil
		case 3:
			return "", workspaceVerificationCall("build", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		default:
			return "Parent build and tests passed", nil
		}
	})
	if err != nil || reply != "Parent build and tests passed" || runtime.commands != 1 || rounds != 4 {
		t.Fatalf("delegated edits were not revalidated by parent: reply=%q err=%v rounds=%d", reply, err, rounds)
	}
}

func TestHarnessGateRestoresPendingDelegationFromDurableTurn(t *testing.T) {
	items := []AssistantTurnItem{
		{ItemType: assistantTurnItemToolCall, CallID: "delegation-call", ToolName: "harness_multi_agent_delegate", ArgumentsJSON: `{"objectives":["inspect backend","verify tests"]}`, Status: assistantTurnStatusCompleted},
		{ItemType: assistantTurnItemToolResult, CallID: "delegation-call", ResultJSON: `{"coordinationId":"coord-restored","status":"running"}`, Status: assistantTurnStatusCompleted},
	}
	gate := &agentVerificationGate{}
	gate.Restore(items)
	if !gate.PendingDelegation() || gate.activeDelegation != "coord-restored" || !gate.Pending() {
		t.Fatalf("incomplete restored worker was permitted to finish: %+v", gate)
	}
	gate.Observe("harness_multi_agent_wait", `{"coordination_id":"coord-restored"}`,
		toolExecOutcome{Found: true, Output: json.RawMessage(`{"coordinationId":"coord-restored","status":"succeeded"}`)})
	if gate.PendingDelegation() {
		t.Fatal("terminal child checkpoint still appears running")
	}
	if !gate.Pending() {
		t.Fatal("restored delegated workspace work bypassed parent validation")
	}
	gate.Observe("execute_host_command", `{"command":"go test ./internal/chat"}`,
		toolExecOutcome{Found: true, Output: json.RawMessage(`{"exitCode":0}`)})
	if gate.Pending() {
		t.Fatal("fresh parent validation did not clear pending revision")
	}
}

func TestVerificationGateRestoresJSONEncodedToolResults(t *testing.T) {
	encodedVerification, _ := json.Marshal(`{"exitCode":0,"timedOut":false}`)
	encodedDelegation, _ := json.Marshal(`{"coordinationId":"coord-encoded","status":"running"}`)
	for _, encoded := range []bool{false, true} {
		check := `{"exitCode":0,"timedOut":false}`
		delegation := `{"coordinationId":"coord-encoded","status":"running"}`
		if encoded {
			check = string(encodedVerification)
			delegation = string(encodedDelegation)
		}
		gate := &agentVerificationGate{}
		gate.Restore([]AssistantTurnItem{
			{ItemType: assistantTurnItemToolCall, CallID: "edit", ToolName: "apply_file", ArgumentsJSON: `{"path":"main.go"}`},
			{ItemType: assistantTurnItemToolResult, CallID: "edit", Status: assistantTurnStatusCompleted, ResultJSON: `{"applied":true}`},
			{ItemType: assistantTurnItemToolCall, CallID: "delegate", ToolName: "harness_multi_agent_delegate", ArgumentsJSON: `{"objectives":["inspect backend","review tests"]}`},
			{ItemType: assistantTurnItemToolResult, CallID: "delegate", Status: assistantTurnStatusCompleted, ResultJSON: delegation},
			{ItemType: assistantTurnItemToolCall, CallID: "check", ToolName: "execute_host_command", ArgumentsJSON: `{"command":"go test ./internal/chat"}`},
			{ItemType: assistantTurnItemToolResult, CallID: "check", Status: assistantTurnStatusCompleted, ResultJSON: check},
		})
		if gate.Pending() || !gate.PendingDelegation() || gate.activeDelegation != "coord-encoded" {
			t.Fatalf("persisted tool checkpoint was misread: encoded=%v gate=%+v", encoded, gate)
		}
	}
}

func TestWorkspaceAgentRetriesFailedValidationAfterRepair(t *testing.T) {
	runtime := &verificationGateRuntime{}
	defs := []tool.Tool{{Function: tool.Function{Name: "apply_file"}}, {Function: tool.Function{Name: "execute_host_command"}}}
	rounds := 0
	result, err := testWorkspaceVerificationRun(t, runtime, defs, func(messages []map[string]interface{}) (string, []map[string]interface{}) {
		rounds++
		switch rounds {
		case 1:
			return "", workspaceVerificationCall("first-edit", "apply_file", `{"path":"src/recover.go"}`)
		case 2:
			runtime.failVerification = true
			return "", workspaceVerificationCall("failing-test", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		case 3:
			return "I am done", nil
		case 4:
			foundFixPrompt := false
			for _, m := range messages {
				if m["role"] == "system" && strings.Contains(m["content"].(string), "失败修复") {
					foundFixPrompt = true
				}
			}
			if !foundFixPrompt {
				t.Fatal("model was not given explicit diagnostic and repair guidance")
			}
			return "", workspaceVerificationCall("repair-edit", "apply_file", `{"path":"src/recover.go"}`)
		case 5:
			runtime.failVerification = false
			return "", workspaceVerificationCall("passing-test", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		default:
			return "repaired and verified", nil
		}
	})
	if err != nil || result != "repaired and verified" || rounds != 6 ||
		runtime.mutations != 2 || runtime.commands != 2 {
		t.Fatalf("agent repair-and-retest cycle failed: reply=%q err=%v rounds=%d edits=%d tests=%d", result, err, rounds, runtime.mutations, runtime.commands)
	}
}

func TestWorkspaceAgentCanRevalidateAfterEarlierReminder(t *testing.T) {
	runtime := &verificationGateRuntime{}
	defs := []tool.Tool{{Function: tool.Function{Name: "apply_file"}}, {Function: tool.Function{Name: "execute_host_command"}}}
	rounds := 0
	result, err := testWorkspaceVerificationRun(t, runtime, defs, func(messages []map[string]interface{}) (string, []map[string]interface{}) {
		rounds++
		switch rounds {
		case 1:
			return "", workspaceVerificationCall("file-write", "apply_file", `{"path":"src/agent.go"}`)
		case 2:
			return "already done", nil
		case 3:
			runtime.failVerification = true
			return "", workspaceVerificationCall("first-test", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		case 4:
			return "all good", nil
		case 5:
			foundSecondReminder := false
			for _, message := range messages {
				if message["role"] == "system" && strings.Contains(message["content"].(string), "失败修复") {
					foundSecondReminder = true
				}
			}
			if !foundSecondReminder {
				t.Fatal("failed test did not reopen the parent verification gate")
			}
			runtime.failVerification = false
			return "", workspaceVerificationCall("second-test", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		default:
			return "now verified", nil
		}
	})
	if err != nil || result != "now verified" || rounds != 6 || runtime.commands != 2 {
		t.Fatalf("failed verification did not allow remediation: result=%q err=%v rounds=%d", result, err, rounds)
	}
}

func TestVerificationGateAllowsNewStatusAfterWorkerPoll(t *testing.T) {
	gate := &agentVerificationGate{}
	running := toolExecOutcome{Found: true, Output: json.RawMessage(`{"coordinationId":"coord-running","status":"running"}`)}
	gate.Observe("harness_multi_agent_delegate", `{"objectives":["review backend","review tests"]}`, running)
	if !gate.ShouldRequestDelegationStatus() || gate.ShouldRequestDelegationStatus() {
		t.Fatal("first outstanding delegation reminder was not single-shot")
	}
	gate.Observe("harness_multi_agent_wait", `{"coordination_id":"coord-running"}`, running)
	if !gate.ShouldRequestDelegationStatus() || gate.ShouldRequestDelegationStatus() {
		t.Fatal("new real worker status did not reopen the outstanding reminder")
	}
	gate.Observe("harness_multi_agent_wait", `{"coordination_id":"coord-running"}`,
		toolExecOutcome{Found: true, Output: json.RawMessage(`{"coordinationId":"coord-running","status":"succeeded"}`)})
	if gate.PendingDelegation() || gate.ShouldRequestDelegationStatus() {
		t.Fatal("terminal coordination still blocked parent finalization")
	}
}

func TestWorkspaceVerificationRejectsShellSuccessMasking(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{command: "go test ./... && echo success", want: false},
		{command: "go test ./... || true", want: false},
		{command: "go test ./... ; exit 0", want: false},
		{command: "go test ./... | tee result.log", want: false},
		{command: "go test ./... > result.log", want: false},
		{command: "echo done && go test ./...", want: false},
		{command: "go test $(echo ./...)", want: false},
		{command: "cd backend && go test ./internal/chat", want: true},
		{command: "go test ./internal/chat", want: true},
	} {
		args, err := json.Marshal(map[string]string{"command": tc.command})
		if err != nil {
			t.Fatal(err)
		}
		if got := agentIsVerificationCommand("execute_host_command", string(args)); got != tc.want {
			t.Errorf("verification classifier command=%q got=%v want=%v", tc.command, got, tc.want)
		}
	}
}
