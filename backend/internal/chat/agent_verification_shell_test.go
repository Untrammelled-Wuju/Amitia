package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
)

func TestAgentShellMutationDetection(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{`echo "hello" > src/main.go`, true},
		{`echo "fake; go test ./..." > src/main.go`, true},
		{`Write-Output "x > y"`, false},
		{`Set-Content -Path src/main.go -Value "new"`, true},
		{`git apply change.patch`, true},
		{`git diff --check`, false},
		{`go test ./internal/chat`, false},
		{`go fmt ./...`, true},
		{`gofmt -w main.go`, true},
		{`& 'C:\Code\Go\bin\gofmt.exe' -w src/main.go`, true},
		{`python -c "open('x', 'w').write('test')"`, true},
		{`pnpm install`, true},
		{`pnpm test`, false},
		{`sed -i 's/a/b/' main.go`, true},
		{`rm -rf node_modules`, true},
		{`ls -la`, false},
	}
	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			data, err := json.Marshal(map[string]string{"command": tc.command})
			if err != nil {
				t.Fatal(err)
			}
			if got := agentToolMutatesWorkspace("execute_host_command", string(data)); got != tc.want {
				t.Fatalf("shell mutation=%v want=%v for %q", got, tc.want, tc.command)
			}
		})
	}
}

func TestAgentShellMutationRequiresFreshTestEvidence(t *testing.T) {
	gate := &agentVerificationGate{}
	ok := toolExecOutcome{Found: true, Output: json.RawMessage(`{"exitCode":0,"stdout":"done"}`)}
	gate.Observe("execute_host_command", `{"command":"go test ./internal/chat"}`, ok)
	if gate.Pending() {
		t.Fatal("test before any changes should not create a pending revision")
	}
	gate.Observe("execute_host_command", `{"command":"echo changed > src/main.go"}`, ok)
	if !gate.Pending() {
		t.Fatal("terminal source edit bypassed verification gate")
	}
	gate.Observe("execute_host_command", `{"command":"echo 'go test ./...'"}`, ok)
	if !gate.Pending() {
		t.Fatal("fake test command was accepted")
	}
	gate.Observe("execute_host_command", `{"command":"go test ./internal/chat"}`, toolExecOutcome{
		Found: true, HasError: true, Output: json.RawMessage(`{"exitCode":1,"stderr":"compile failed"}`),
	})
	if !gate.Pending() {
		t.Fatal("failed test was accepted")
	}
	gate.Observe("execute_host_command", `{"command":"go test ./internal/chat"}`, ok)
	if gate.Pending() {
		t.Fatal("successful fresh verification was not accepted")
	}
	gate.Observe("execute_host_command", `{"command":"Set-Content -Path src/main.go -Value fixed"}`, ok)
	if !gate.Pending() {
		t.Fatal("next terminal edit failed to invalidate prior verification")
	}
}

func TestWorkspaceAgentTerminalEditTestCompletion(t *testing.T) {
	runtime := &verificationGateRuntime{}
	definitions := []tool.Tool{{Function: tool.Function{Name: "execute_host_command"}}}
	calls := 0
	reply, err := testWorkspaceVerificationRun(t, runtime, definitions, func(messages []map[string]interface{}) (string, []map[string]interface{}) {
		calls++
		switch calls {
		case 1:
			return "", workspaceVerificationCall("shell-write", "execute_host_command", `{"command":"Set-Content -Path src/main.go -Value changed"}`)
		case 2:
			return "I am done", nil
		case 3:
			found := false
			for _, m := range messages {
				if m["role"] == "system" && strings.Contains(m["content"].(string), "执行验收门禁") {
					found = true
				}
			}
			if !found {
				t.Fatal("shell edit did not trigger mandatory verification prompt")
			}
			return "", workspaceVerificationCall("shell-test", "execute_host_command", `{"command":"go test ./internal/chat"}`)
		default:
			return "verified", nil
		}
	})
	if err != nil || reply != "verified" || runtime.commands != 2 || calls != 4 {
		t.Fatalf("shell edit verification failed: reply=%q err=%v commands=%d rounds=%d", reply, err, runtime.commands, calls)
	}
}
