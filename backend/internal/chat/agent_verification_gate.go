package chat

import (
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
)

type agentVerificationGate struct {
	mutationRevision        int
	verifiedRevision        int
	reminderRevision        int
	verificationAttempt     int
	reminderAttempt         int
	activeDelegation        string
	delegationReminder      string
	delegationPolls         int
	delegationReminderPolls int
}

func (g *agentVerificationGate) Observe(name, arguments string, outcome toolExecOutcome) {
	if g == nil {
		return
	}
	if g.Pending() && agentIsVerificationCommand(name, arguments) {
		g.verificationAttempt++
		if agentVerificationEvidenceSucceeded(outcome) {
			g.verifiedRevision = g.mutationRevision
		}
	}
	if outcome.HasError || !outcome.Found {
		return
	}
	if name == "harness_multi_agent_delegate" || name == "harness_multi_agent_status" || name == "harness_multi_agent_wait" {
		var coordination struct {
			CoordinationID string `json:"coordinationId"`
			Status         string `json:"status"`
		}
		if json.Unmarshal(outcome.Output, &coordination) == nil && coordination.CoordinationID != "" {
			switch coordination.Status {
			case "succeeded", "failed", "cancelled":
				if g.activeDelegation == coordination.CoordinationID {
					g.activeDelegation = ""
				}
			default:
				g.activeDelegation = coordination.CoordinationID
				g.delegationPolls++
			}
		}
	}
	if agentToolMutatesWorkspace(name, arguments) {
		g.mutationRevision++
		return
	}
}

func (g *agentVerificationGate) Pending() bool {
	return g != nil && g.mutationRevision > g.verifiedRevision
}

func (g *agentVerificationGate) PendingDelegation() bool {
	return g != nil && g.activeDelegation != ""
}

func (g *agentVerificationGate) ShouldRequestDelegationStatus() bool {
	if !g.PendingDelegation() || (g.delegationReminder == g.activeDelegation && g.delegationReminderPolls == g.delegationPolls) {
		return false
	}
	g.delegationReminder = g.activeDelegation
	g.delegationReminderPolls = g.delegationPolls
	return true
}

func (g *agentVerificationGate) ShouldRequestVerification() bool {
	if !g.Pending() || (g.reminderRevision == g.mutationRevision && g.reminderAttempt == g.verificationAttempt) {
		return false
	}
	g.reminderRevision = g.mutationRevision
	g.reminderAttempt = g.verificationAttempt
	return true
}

func agentIsWorkspaceMutation(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "apply_file", "apply_patch", "write_file", "edit_file", "delete_file", "workspace.write", "workspace.edit", "workspace.apply_patch", "workspace.delete", "harness_multi_agent_delegate":
		return true
	default:
		return false
	}
}

func agentIsVerificationCommand(name, arguments string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "execute_host_command", "execute_terminal", "execute_in_terminal_session_streaming":
	default:
		return false
	}
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return false
	}
	if agentShellHasOutputRedirection(params.Command) || strings.Contains(params.Command, "$(") || strings.Contains(params.Command, "`") {
		return false
	}
	var segments []string
	for _, segment := range agentVerificationCommandSegments(params.Command) {
		if strings.TrimSpace(segment) != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return false
	}
	for _, prefix := range segments[:len(segments)-1] {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(prefix)))
		if len(fields) < 2 || (fields[0] != "cd" && fields[0] != "set-location" && fields[0] != "pushd") {
			return false
		}
	}
	for _, segment := range segments[len(segments)-1:] {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(segment)))
		if len(fields) == 0 {
			continue
		}
		executable := strings.Trim(fields[0], string([]byte{34, 39}))
		executable = strings.ReplaceAll(executable, "\\", "/")
		if index := strings.LastIndexByte(executable, '/'); index >= 0 {
			executable = executable[index+1:]
		}
		if len(fields) < 2 {
			switch executable {
			case "pytest", "pytest.exe", "eslint", "eslint.exe", "vitest", "vitest.exe", "jest", "jest.exe":
				return true
			}
			continue
		}
		first := fields[1]
		switch executable {
		case "go", "go.exe":
			if first == "test" || first == "build" || first == "vet" {
				return true
			}
		case "cargo", "cargo.exe":
			if first == "test" || first == "check" || first == "build" {
				return true
			}
		case "flutter", "flutter.exe":
			if first == "test" || first == "analyze" || first == "build" {
				return true
			}
		case "gradle", "gradle.exe", "gradlew", "gradlew.bat", "./gradlew":
			if first == "test" || first == "build" || strings.HasSuffix(first, "test") {
				return true
			}
		case "mvn", "mvn.cmd", "mvnw", "mvnw.cmd":
			if first == "test" || first == "verify" || first == "package" {
				return true
			}
		case "dotnet", "dotnet.exe":
			if first == "test" || first == "build" {
				return true
			}
		case "pnpm", "pnpm.cmd", "yarn", "yarn.cmd", "npm", "npm.cmd":
			if first == "test" || first == "build" || first == "lint" {
				return true
			}
			if first == "run" && len(fields) > 2 && (fields[2] == "test" || fields[2] == "build" || fields[2] == "lint") {
				return true
			}
		case "python", "python.exe", "python3", "python3.exe":
			if first == "-m" && len(fields) > 2 && (fields[2] == "pytest" || fields[2] == "unittest" || fields[2] == "compileall") {
				return true
			}
		case "git", "git.exe":
			if first == "diff" && len(fields) > 2 && fields[2] == "--check" {
				return true
			}
		case "tsc", "tsc.exe":
			if first == "--noemit" {
				return true
			}
		case "pytest", "pytest.exe", "eslint", "eslint.exe", "vitest", "vitest.exe", "jest", "jest.exe":
			return true
		}
	}
	return false
}

func agentVerificationEvidenceSucceeded(outcome toolExecOutcome) bool {
	if outcome.HasError || !outcome.Found {
		return false
	}
	if len(outcome.Output) == 0 {
		return false
	}
	var result struct {
		ExitCode *int `json:"exitCode"`
		TimedOut bool `json:"timedOut"`
	}
	if err := json.Unmarshal(outcome.Output, &result); err != nil || result.ExitCode == nil {
		return false
	}
	return *result.ExitCode == 0 && !result.TimedOut
}

func agentVerificationToolAvailable(definitions []tool.Tool) bool {
	for _, definition := range definitions {
		switch definition.Function.Name {
		case "execute_host_command", "execute_terminal", "execute_in_terminal_session_streaming":
			return true
		}
	}
	return false
}

func (g *agentVerificationGate) Restore(items []AssistantTurnItem) {
	if g == nil {
		return
	}
	calls := make(map[string]AssistantTurnItem)
	for _, item := range items {
		switch item.ItemType {
		case assistantTurnItemToolCall:
			if strings.TrimSpace(item.CallID) != "" {
				calls[item.CallID] = item
			}
		case assistantTurnItemToolResult:
			call, ok := calls[item.CallID]
			if !ok {
				continue
			}
			content := item.ResultJSON
			var encodedString string
			if json.Unmarshal([]byte(content), &encodedString) == nil {
				content = encodedString
			}
			outcome := toolExecOutcome{
				Found:    true,
				Status:   item.Status,
				HasError: item.Status != assistantTurnStatusCompleted || item.ErrorCode != "",
				Output:   []byte(content),
			}
			g.Observe(call.ToolName, call.ArgumentsJSON, outcome)
		}
	}
}

func agentVerificationCommandSegments(command string) []string {
	var sections []string
	var current strings.Builder
	var quoted rune
	escaped := false
	for _, char := range command {
		switch {
		case escaped:
			escaped = false
		case quoted != 0 && char == '\\':
			escaped = true
		case quoted != 0 && char == quoted:
			quoted = 0
		case quoted == 0 && (char == '\'' || char == '"'):
			quoted = char
		case quoted == 0 && (char == ';' || char == '\n' || char == '&' || char == '|'):
			sections = append(sections, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(char)
	}
	if current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}
