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
			name:     "capability discovery structured output",
			toolName: "find_capability",
			outcome:  toolExecOutcome{VisibleText: "Found 1 candidate(s)", Output: []byte(`{"candidates":[{"id":"provider","extensionId":"com.example/ext"}]}`), Found: true},
			want:     `{"candidates":[{"id":"provider","extensionId":"com.example/ext"}]}`,
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

func TestAssistantTurnRecorderCitationAuditUnknownWithoutAvailableCitations(t *testing.T) {
	recorder := newAssistantTurnRecorder(nil, "conversation-1", "character-1", "message-1", "request-1")
	audit := recorder.AuditCitationMarkers("unsupported marker [99], code `value[88]`")
	if len(audit.Available) != 0 || len(audit.Used) != 0 {
		t.Fatalf("unexpected available/used: %#v", audit)
	}
	if len(audit.Unknown) != 1 || audit.Unknown[0] != 99 {
		t.Fatalf("unknown = %#v, want [99]", audit.Unknown)
	}
}

func TestAssistantTurnRecorderCitationClaimVerifier(t *testing.T) {
	recorder := newAssistantTurnRecorder(nil, "conversation-1", "character-1", "message-1", "request-1")
	if err := recorder.AddToolResult(nil, "call-1", "web_run", `{"citations":[{"index":1,"evidence_id":"ev-1","ref_id":"ref-1","title":"Release","url":"https://example.com","text":"Amitia version 2.0 was released in September 2026."}]}`, "completed", "", 1); err != nil {
		t.Fatalf("AddToolResult() error = %v", err)
	}
	audit := recorder.AuditCitationMarkers("Amitia version 2.0 was released in September 2026 [1].")
	if len(audit.Claims) != 1 {
		t.Fatalf("claims = %#v", audit.Claims)
	}
	if audit.Claims[0].Status != "supported" || audit.Claims[0].SupportScore <= 0 {
		t.Fatalf("claim audit = %#v", audit.Claims[0])
	}
}

func TestAssistantTurnRecorderDetectsMissingCitationClaimAfterWebResearch(t *testing.T) {
	recorder := newAssistantTurnRecorder(nil, "conversation-1", "character-1", "message-1", "request-1")
	if err := recorder.AddToolResult(nil, "call-1", "web_run", `{"citations":[{"index":1,"evidence_id":"ev-1","ref_id":"ref-1","text":"Amitia version 2.0 was released in September 2026."}]}`, "completed", "", 1); err != nil {
		t.Fatalf("AddToolResult() error = %v", err)
	}
	audit := recorder.AuditCitationMarkers("Amitia version 2.0 was released in September 2026. The project supports Linux [1].")
	if len(audit.MissingCitation) != 1 {
		t.Fatalf("missing citations = %#v", audit.MissingCitation)
	}
}

func TestAssistantTurnRecorderDoesNotFlagOpinionAsMissingCitation(t *testing.T) {
	recorder := newAssistantTurnRecorder(nil, "conversation-1", "character-1", "message-1", "request-1")
	if err := recorder.AddToolResult(nil, "call-1", "web_run", `{"citations":[{"index":1,"evidence_id":"ev-1","ref_id":"ref-1","text":"Reference evidence."}]}`, "completed", "", 1); err != nil {
		t.Fatalf("AddToolResult() error = %v", err)
	}
	audit := recorder.AuditCitationMarkers("我认为 2026 版本的交互更顺手。")
	if len(audit.MissingCitation) != 0 {
		t.Fatalf("opinion should not be flagged: %#v", audit.MissingCitation)
	}
}

func TestCitationAuditFlagsMissingClaimsWhenWebResearchReturnedNoCitations(t *testing.T) {
	recorder := newAssistantTurnRecorder(nil, "conv-1", "char-1", "msg-1", "req-1")
	recorder.captureCitationIDs("web_run", `{"operation":"search","citations":[]}`)

	audit := recorder.AuditCitationMarkers("Project X version 2.0 was released in 2026.")
	if len(audit.Available) != 0 {
		t.Fatalf("expected no available citations, got %#v", audit.Available)
	}
	if len(audit.MissingCitation) == 0 {
		t.Fatalf("expected missing citation audit after web research with no citations")
	}
}
