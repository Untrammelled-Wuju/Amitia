package chat

import "testing"

func TestToolResultContent(t *testing.T) {
	tests := []struct {
		name    string
		outcome toolExecOutcome
		want    string
	}{
		{
			name:    "visible text",
			outcome: toolExecOutcome{VisibleText: "done", Found: true},
			want:    "done",
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
			if got := toolResultContent(test.outcome); got != test.want {
				t.Fatalf("toolResultContent() = %q, want %q", got, test.want)
			}
		})
	}
}
