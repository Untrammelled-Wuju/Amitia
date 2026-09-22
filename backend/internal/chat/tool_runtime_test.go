package chat

import "testing"

func TestToolResultContent(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		outcome  toolExecOutcome
		want     string
	}{
		{
			name:    "visible text",
			outcome: toolExecOutcome{VisibleText: "done", Found: true},
			want:    "done",
		},
		{
			name:     "web run structured output",
			toolName: "web_run",
			outcome:  toolExecOutcome{VisibleText: "research complete", Output: []byte(`{"citations":[{"index":1}]}`), Found: true},
			want:     `{"citations":[{"index":1}]}`,
		},
		{
			name:    "empty successful result",
			outcome: toolExecOutcome{Found: true},
			want:    "",
		},
		{
			name:    "failed result with error code",
			outcome: toolExecOutcome{Found: false, HasError: true, ErrorCode: "permission_denied"},
			want:    "工具执行失败：permission_denied",
		},
		{
			name:    "failed result without error code",
			outcome: toolExecOutcome{Found: false},
			want:    "工具执行失败",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := toolResultContent(test.toolName, test.outcome); got != test.want {
				t.Fatalf("toolResultContent() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAssistantTurnRecorderCitationAudit(t *testing.T) {
	recorder := newAssistantTurnRecorder(nil, "conversation-1", "character-1", "message-1", "request-1")
	if err := recorder.AddToolResult(nil, "call-1", "web_run", `{"citations":[{"index":1},{"index":3}]}`, "completed", "", 1); err != nil {
		t.Fatalf("AddToolResult() error = %v", err)
	}
	audit := recorder.AuditCitationMarkers("supported [1], unknown [2], inline `value[2]`, fenced:\n```go\na[3]\n```\nand supported [3]")
	if len(audit.Available) != 2 || audit.Available[0] != 1 || audit.Available[1] != 3 {
		t.Fatalf("available = %#v, want [1 3]", audit.Available)
	}
	if len(audit.Used) != 2 || audit.Used[0] != 1 || audit.Used[1] != 3 {
		t.Fatalf("used = %#v, want [1 3]", audit.Used)
	}
	if len(audit.Unknown) != 1 || audit.Unknown[0] != 2 {
		t.Fatalf("unknown = %#v, want [2]", audit.Unknown)
	}
}
