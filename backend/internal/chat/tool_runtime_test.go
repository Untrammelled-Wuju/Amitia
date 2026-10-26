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
			name:     "capability discovery structured output",
			toolName: "find_capability",
			outcome:  toolExecOutcome{VisibleText: "Found 1 candidate(s)", Output: []byte(`{"candidates":[{"id":"provider","extensionId":"com.example/ext"}]}`), Found: true},
			want:     `{"candidates":[{"id":"provider","extensionId":"com.example/ext"}]}`,
		},
		{
			name:     "android ui tree structured output",
			toolName: "android_ui_tree_snapshot",
			outcome:  toolExecOutcome{VisibleText: "snapshot ready", Output: []byte(`{"snapshotId":"uis_1","nodeCount":2}`), Found: true},
			want:     `{"snapshotId":"uis_1","nodeCount":2}`,
		},
		{
			name:     "android virtual display structured output",
			toolName: "android_virtual_display_status",
			outcome:  toolExecOutcome{VisibleText: "display ready", Output: []byte(`{"supported":true,"activeCount":1}`), Found: true},
			want:     `{"supported":true,"activeCount":1}`,
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

func TestAssistantTurnRecorderCitationAuditUnknownWithoutAvailableCitations(t *testing.T) {
	recorder := &ownedCitationAudit{}
	audit := recorder.AuditCitationMarkers("unsupported marker [99], code `value[88]`")
	if len(audit.Available) != 0 || len(audit.Used) != 0 {
		t.Fatalf("unexpected available/used: %#v", audit)
	}
	if len(audit.Unknown) != 1 || audit.Unknown[0] != 99 {
		t.Fatalf("unknown = %#v, want [99]", audit.Unknown)
	}
}
